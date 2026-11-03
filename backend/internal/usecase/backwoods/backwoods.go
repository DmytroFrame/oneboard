// Package backwoods implements OmiTeam's admin panel ("Backwoods"). What concerns the whole installation
// (database backups, the schema, runtime versions) is for platform admins (PLATFORM_ADMINS); the data
// checks are for owners and admins of an organization and only look at its projects.
package backwoods

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/health"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
)

var Module = modules.Register(modules.UseCases, fx.Module("backwoods",
	fx.Provide(NewService),
	fx.Invoke(func(lc fx.Lifecycle, s *Service) { s.schedule(lc) }),
))

type Backups interface {
	// Last lists the newest backups.
	Last(ctx context.Context, n int) ([]domain.Backup, error)
	Record(ctx context.Context, b *domain.Backup) error
	Get(ctx context.Context, id int64) (domain.Backup, error)
}

// Dumper writes a consistent dump of every table; domain.ErrBackupRunning if one is under way.
type Dumper interface {
	Dump(ctx context.Context, w io.Writer) (tables int, err error)
}

type Storage interface {
	Put(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error
	Read(ctx context.Context, key string) (io.ReadCloser, error)
}

type Inspector interface {
	Schema(ctx context.Context) (domain.DBSchema, error)
	SpecialTaskProjects(ctx context.Context, orgID int64) ([]domain.ProjectFinding, error)
	CommentedProjects(ctx context.Context, orgID int64) ([]domain.ProjectFinding, error)
}

type Deps struct {
	fx.In
	Backups   Backups
	Dumper    Dumper
	Storage   Storage
	Inspector Inspector
	Versions  []health.Version `group:"versions"`
	Config    config.Config
	Log       *slog.Logger
}

type Service struct {
	backups   Backups
	dumper    Dumper
	storage   Storage
	inspector Inspector
	versions  []health.Version
	every     time.Duration
	minGap    time.Duration
	log       *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{d.Backups, d.Dumper, d.Storage, d.Inspector, d.Versions, d.Config.BackupEvery, d.Config.BackupMinGap, d.Log}
}

// allowed: the installation's tools are for platform admins only; organizations are tenants.
func allowed(u domain.User) error {
	if !u.PlatformAdmin {
		return domain.ErrPermissionDenied
	}
	return nil
}

// checks: an organization's owners and admins check its own data.
func checks(u domain.User) error {
	if !u.Can(domain.PermOpenBackwoods) {
		return domain.ErrPermissionDenied
	}
	return nil
}

// BackupState: the latest backups and when the next one can be made by hand.
type BackupState struct {
	Backups []domain.Backup
	NextAt  *time.Time
}

func (s *Service) Backups(ctx context.Context, u domain.User) (BackupState, error) {
	if err := allowed(u); err != nil {
		return BackupState{}, err
	}
	return s.state(ctx)
}

func (s *Service) state(ctx context.Context) (BackupState, error) {
	list, err := s.backups.Last(ctx, 10)
	if err != nil {
		return BackupState{}, err
	}
	st := BackupState{Backups: list}
	if len(list) > 0 {
		if next := list[0].CreatedAt.Add(s.minGap); next.After(time.Now()) {
			st.NextAt = &next
		}
	}
	return st, nil
}

// Backup makes a backup by hand, at most once per BackupMinGap.
func (s *Service) Backup(ctx context.Context, u domain.User) (BackupState, error) {
	if err := allowed(u); err != nil {
		return BackupState{}, err
	}
	st, err := s.state(ctx)
	if err != nil {
		return st, err
	}
	if st.NextAt != nil {
		return st, domain.ErrBackupTooSoon
	}
	if _, err := s.backup(ctx, &u); err != nil {
		return st, err
	}
	return s.state(ctx)
}

// backup dumps the database to a temporary file, uploads it and records it (author nil = CRON).
func (s *Service) backup(ctx context.Context, author *domain.User) (domain.Backup, error) {
	f, err := os.CreateTemp("", "oneboard-backup-*.tar.gz")
	if err != nil {
		return domain.Backup{}, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	tables, err := s.dumper.Dump(ctx, f)
	if err != nil {
		return domain.Backup{}, err
	}
	size, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return domain.Backup{}, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return domain.Backup{}, err
	}
	now := time.Now().UTC()
	b := domain.Backup{Key: fmt.Sprintf("backups/%s.tar.gz", now.Format("2006-01-02T15-04-05Z")), Size: size, Tables: tables, Author: author}
	if err := s.storage.Put(ctx, b.Key, "application/gzip", f, size); err != nil {
		return domain.Backup{}, err
	}
	if err := s.backups.Record(ctx, &b); err != nil {
		return domain.Backup{}, err
	}
	s.log.Info("database backup made", "key", b.Key, "bytes", size, "tables", tables, "cron", author == nil)
	return b, nil
}

// Download streams a backup's archive.
func (s *Service) Download(ctx context.Context, u domain.User, id int64) (domain.Backup, io.ReadCloser, error) {
	if err := allowed(u); err != nil {
		return domain.Backup{}, nil, err
	}
	b, err := s.backups.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNoRows) {
			err = domain.ErrBackupNotFound
		}
		return b, nil, err
	}
	body, err := s.storage.Read(ctx, b.Key)
	return b, body, err
}

// schedule makes a CRON backup whenever the latest one is older than BackupEvery. Every instance
// checks; the dump's lock lets only one of them make it.
func (s *Service) schedule(lc fx.Lifecycle) {
	if s.every <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				t := time.NewTicker(10 * time.Minute)
				defer t.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-t.C:
						s.maybeBackup(ctx)
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			<-done
			return nil
		},
	})
}

func (s *Service) maybeBackup(ctx context.Context) {
	last, err := s.backups.Last(ctx, 1)
	if err != nil || (len(last) > 0 && time.Since(last[0].CreatedAt) < s.every) {
		return
	}
	if _, err := s.backup(ctx, nil); err != nil && !errors.Is(err, domain.ErrBackupRunning) && ctx.Err() == nil {
		s.log.Error("scheduled backup", "err", err)
	}
}

func (s *Service) Schema(ctx context.Context, u domain.User) (domain.DBSchema, error) {
	if err := allowed(u); err != nil {
		return domain.DBSchema{}, err
	}
	return s.inspector.Schema(ctx)
}

// Versions: the app's own build and every dependency that reports one ("" = unknown).
func (s *Service) Versions(ctx context.Context, u domain.User) ([]domain.RuntimeVersion, error) {
	if err := allowed(u); err != nil {
		return nil, err
	}
	out := []domain.RuntimeVersion{{Name: "Go", Version: runtime.Version()}, {Name: "OS / arch", Version: runtime.GOOS + "/" + runtime.GOARCH}}
	if info, ok := debug.ReadBuildInfo(); ok {
		rev := ""
		for _, kv := range info.Settings {
			if kv.Key == "vcs.revision" && len(kv.Value) >= 7 {
				rev = kv.Value[:7]
			}
		}
		out = append(out, domain.RuntimeVersion{Name: "OneBoard", Version: rev})
	}
	for _, v := range s.versions {
		vctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		ver, err := v.Get(vctx)
		cancel()
		if err != nil {
			ver = ""
		}
		out = append(out, domain.RuntimeVersion{Name: v.Name, Version: ver})
	}
	return out, nil
}

func (s *Service) SpecialTaskProjects(ctx context.Context, u domain.User) ([]domain.ProjectFinding, error) {
	if err := checks(u); err != nil {
		return nil, err
	}
	return s.inspector.SpecialTaskProjects(ctx, u.OrgID)
}

func (s *Service) CommentedProjects(ctx context.Context, u domain.User) ([]domain.ProjectFinding, error) {
	if err := checks(u); err != nil {
		return nil, err
	}
	return s.inspector.CommentedProjects(ctx, u.OrgID)
}
