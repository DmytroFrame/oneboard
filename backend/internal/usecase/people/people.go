// Package people implements the team directory and profiles (OmiTeam): who works here, their
// details, internal comments, 360 feedback and documents, and HR creating accounts.
package people

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"slices"
	"strings"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("people", fx.Provide(NewService)))

// Everything here belongs to one organization (the viewer's OrgID): its members, their profiles,
// notes and documents, and its reference lists.

type Users interface {
	ByEmail(ctx context.Context, email string) (domain.User, error)
	// InOrg loads a person with their membership; domain.ErrNoRows for people outside the organization.
	InOrg(ctx context.Context, orgID, userID int64) (domain.User, error)
	Memberships(ctx context.Context, userID int64) ([]domain.Membership, error)
	Create(ctx context.Context, u *domain.User) error // domain.ErrDuplicate on a taken email
	Join(ctx context.Context, orgID, userID int64, role domain.CompanyRole) error
	Count(ctx context.Context) (int, error)
	Rename(ctx context.Context, userID int64, name string) error
}

type Profiles interface {
	Get(ctx context.Context, orgID, userID int64) (domain.Profile, error) // domain.ErrNoRows for non-members
	List(ctx context.Context, orgID int64, f domain.PersonFilter) (domain.PersonPage, error)
	Save(ctx context.Context, orgID int64, p domain.Profile) error
}

type Directory interface {
	List(ctx context.Context, orgID int64, kind domain.DirectoryKind) ([]domain.DirectoryItem, error)
	Create(ctx context.Context, orgID int64, kind domain.DirectoryKind, name string) (domain.DirectoryItem, error)
	Rename(ctx context.Context, orgID, id int64, name string) (domain.DirectoryItem, error)
	Delete(ctx context.Context, orgID, id int64) error
}

type Notes interface {
	Comments(ctx context.Context, orgID, userID int64) ([]domain.PersonComment, error)
	AddComment(ctx context.Context, orgID int64, c *domain.PersonComment) error
	Comment(ctx context.Context, orgID, id int64) (domain.PersonComment, error)
	DeleteComment(ctx context.Context, orgID, id int64) error
	Feedback(ctx context.Context, orgID, userID int64) ([]domain.Feedback, error)
	AddFeedback(ctx context.Context, orgID int64, f *domain.Feedback) error
}

type Documents interface {
	List(ctx context.Context, orgID, userID int64) ([]domain.Document, error)
	Get(ctx context.Context, orgID, id int64) (domain.Document, error)
	Create(ctx context.Context, orgID int64, d *domain.Document) error
	Update(ctx context.Context, d domain.Document) error
	Delete(ctx context.Context, id int64) error
}

type FileStorage interface {
	Put(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error
	Read(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// HRLog records hires for the HR statistics.
type HRLog interface {
	Record(ctx context.Context, e domain.HREvent) error
}

type Service struct {
	users     Users
	profiles  Profiles
	directory Directory
	notes     Notes
	docs      Documents
	files     FileStorage
	hasher    PasswordHasher
	mailer    Mailer
	hr        HRLog
	appURL    string
}

type Deps struct {
	fx.In
	Users     Users
	Profiles  Profiles
	Directory Directory
	Notes     Notes
	Documents Documents
	Files     FileStorage
	Hasher    PasswordHasher
	Mailer    Mailer
	HR        HRLog
	Config    config.Config
}

func NewService(d Deps) *Service {
	return &Service{users: d.Users, profiles: d.Profiles, directory: d.Directory, notes: d.Notes, docs: d.Documents,
		files: d.Files, hasher: d.Hasher, mailer: d.Mailer, hr: d.HR, appURL: d.Config.AppURL}
}

// PersonView is a profile as one viewer sees it.
type PersonView struct {
	Profile domain.Profile
	Access  domain.ProfileAccess
}

// List is the team page. Candidates and the inactive are hidden unless asked for; contractors and
// candidates can't see the team at all.
func (s *Service) List(ctx context.Context, viewer domain.User, f domain.PersonFilter) (domain.PersonPage, []domain.ProfileAccess, error) {
	if !viewer.Can(domain.PermViewCrewBlock) {
		return domain.PersonPage{}, nil, domain.ErrPermissionDenied
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if !viewer.Can(domain.PermEditUser) {
		f.IncludeInactive = false
	}
	page, err := s.profiles.List(ctx, viewer.OrgID, f)
	if err != nil {
		return page, nil, err
	}
	access := make([]domain.ProfileAccess, len(page.People))
	for i, p := range page.People {
		access[i] = domain.AccessTo(viewer, p.User)
		page.People[i] = access[i].Redact(p)
	}
	return page, access, nil
}

// Get returns a profile redacted for the viewer. Candidates only see their own; contractors only
// their own and the basics of others.
func (s *Service) Get(ctx context.Context, viewer domain.User, id int64) (PersonView, error) {
	p, err := s.profiles.Get(ctx, viewer.OrgID, id)
	if err != nil {
		return PersonView{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	if viewer.ID != id && (viewer.Role == domain.CompanyCandidate ||
		(p.User.Role == domain.CompanyCandidate && !viewer.Can(domain.PermViewCandidates))) {
		return PersonView{}, domain.ErrUserNotFound
	}
	a := domain.AccessTo(viewer, p.User)
	return PersonView{Profile: a.Redact(p), Access: a}, nil
}

// ProfileUpdate is a full profile form; fields the editor may not change are kept as they were.
type ProfileUpdate struct {
	Name    string
	Profile domain.Profile
}

func (s *Service) Update(ctx context.Context, viewer domain.User, id int64, in ProfileUpdate) (PersonView, error) {
	current, err := s.profiles.Get(ctx, viewer.OrgID, id)
	if err != nil {
		return PersonView{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	a := domain.AccessTo(viewer, current.User)
	if !a.EditBasics && !a.EditWork && !a.EditFinance {
		return PersonView{}, domain.ErrPermissionDenied
	}
	next := current
	in.Profile.User = current.User
	if a.EditBasics {
		d, src := &next.Details, in.Profile.Details
		d.About, d.Phone, d.Telegram, d.WhatsApp, d.Country, d.City = src.About, src.Phone, src.Telegram, src.WhatsApp, src.Country, src.City
		d.TechStack, d.EnglishLevel = src.TechStack, src.EnglishLevel
		d.GitHub, d.GitLab, d.LinkedIn, d.Upwork, d.CVLink, d.OtherLinks = src.GitHub, src.GitLab, src.LinkedIn, src.Upwork, src.CVLink, src.OtherLinks
		d.PostAddress, d.Emergency = src.PostAddress, src.Emergency
		next.NameCyrillic, next.Birthday = in.Profile.NameCyrillic, in.Profile.Birthday
	}
	if a.EditWork {
		next.StartDate, next.TrialUntil, next.WorkingHours, next.Billable = in.Profile.StartDate, in.Profile.TrialUntil, in.Profile.WorkingHours, in.Profile.Billable
		next.Details.Specialization, next.Details.Grade, next.Details.CompanyEmail = in.Profile.Details.Specialization, in.Profile.Details.Grade, in.Profile.Details.CompanyEmail
		next.Details.HRNotes = in.Profile.Details.HRNotes
	}
	if a.EditFinance {
		next.Finance = in.Profile.Finance
	} else if a.Self {
		// People keep their own payment details current; salary and rate stay HR's.
		f := in.Profile.Finance
		next.Finance.FOP, next.Finance.PayoneerEmail, next.Finance.MonobankCard, next.Finance.PrivatBankCard, next.Finance.TaxID =
			f.FOP, f.PayoneerEmail, f.MonobankCard, f.PrivatBankCard, f.TaxID
	}
	if err := next.Normalize(); err != nil {
		return PersonView{}, err
	}
	if name := strings.TrimSpace(in.Name); name != "" && name != current.User.Name && (a.EditWork || a.Self) {
		if len(name) > 100 {
			return PersonView{}, domain.ErrNameInvalid
		}
		// The name belongs to the account, which other organizations see too: HR renames only people
		// who work nowhere else.
		if !a.Self {
			if ms, err := s.users.Memberships(ctx, id); err != nil {
				return PersonView{}, err
			} else if len(ms) > 1 {
				return PersonView{}, domain.ErrSharedAccount
			}
		}
		if err := s.users.Rename(ctx, id, name); err != nil {
			return PersonView{}, err
		}
		next.User.Name = name
	}
	if err := s.profiles.Save(ctx, viewer.OrgID, next); err != nil {
		return PersonView{}, err
	}
	return PersonView{Profile: a.Redact(next), Access: a}, nil
}

// NewPerson is HR adding someone to the organization: a new account with a generated password emailed
// to them, or, when the email already has an account, that account joining the organization.
type NewPerson struct {
	Name  string
	Email string
	Role  domain.CompanyRole
}

func (s *Service) Create(ctx context.Context, viewer domain.User, in NewPerson) (domain.User, error) {
	if !viewer.Can(domain.PermAddUser) {
		return domain.User{}, domain.ErrPermissionDenied
	}
	name, email := strings.TrimSpace(in.Name), domain.NormalizeEmail(in.Email)
	if name == "" || len(name) > 100 {
		return domain.User{}, domain.ErrNameInvalid
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return domain.User{}, domain.ErrEmailInvalid
	}
	if in.Role != domain.CompanyCandidate && !slices.Contains(viewer.AssignableRoles(), in.Role) {
		return domain.User{}, domain.ErrRoleChangeForbidden
	}
	org := viewer.OrgID
	if existing, err := s.users.ByEmail(ctx, email); err == nil {
		// The person already has an account (from another organization): they join this one as they are;
		// their password and name stay theirs.
		if _, err := s.users.InOrg(ctx, org, existing.ID); err == nil {
			return domain.User{}, domain.ErrAlreadyOrgMember
		}
		if err := s.users.Join(ctx, org, existing.ID, in.Role); err != nil {
			return domain.User{}, err
		}
		if err := s.recordHire(ctx, viewer, existing, in.Role); err != nil {
			return existing, err
		}
		body := fmt.Sprintf("Hi %s,\n\nYou have been added to an organization in OneBoard. Sign in at %s with your account and pick it in the organization switcher.",
			existing.Name, s.appURL)
		_ = s.mailer.Send(ctx, existing.Email, "You were added to an organization in OneBoard", body)
		return s.users.InOrg(ctx, org, existing.ID)
	} else if !errors.Is(err, domain.ErrNoRows) {
		return domain.User{}, err
	}
	password := randomPassword()
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return domain.User{}, err
	}
	count, err := s.users.Count(ctx)
	if err != nil {
		return domain.User{}, err
	}
	u := domain.User{Name: name, Email: email, PasswordHash: hash, Color: domain.AvatarColors[count%len(domain.AvatarColors)]}
	if err := s.users.Create(ctx, &u); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			return domain.User{}, domain.ErrEmailTaken
		}
		return domain.User{}, err
	}
	if err := s.users.Join(ctx, org, u.ID, in.Role); err != nil {
		return domain.User{}, err
	}
	if err := s.recordHire(ctx, viewer, u, in.Role); err != nil {
		return u, err
	}
	body := fmt.Sprintf("Hi %s,\n\nAn account has been created for you in OneBoard.\nSign in at %s with:\n\n  email: %s\n  password: %s\n\nThen change the password in Settings.",
		u.Name, s.appURL, u.Email, password)
	// A failed email doesn't undo the account: HR can reset the password, which sends a new one.
	_ = s.mailer.Send(ctx, u.Email, "Your OneBoard account", body)
	return s.users.InOrg(ctx, org, u.ID)
}

// recordHire puts people added with a staff role into the HR statistics.
func (s *Service) recordHire(ctx context.Context, viewer, u domain.User, role domain.CompanyRole) error {
	if role == domain.CompanyCandidate {
		return nil
	}
	return s.hr.Record(ctx, domain.HREvent{OrgID: viewer.OrgID, User: u, Kind: domain.HRHired, Day: domain.Today(), ActorID: &viewer.ID})
}

// randomPassword is 16 random hex characters plus one of each class the policy wants.
func randomPassword() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "Ob-" + hex.EncodeToString(b) + "!X7"
}

// --- reference lists ---

func (s *Service) Directory(ctx context.Context, viewer domain.User, kind domain.DirectoryKind) ([]domain.DirectoryItem, error) {
	if !kind.Valid() {
		return nil, domain.ErrDirectoryInvalid
	}
	return s.directory.List(ctx, viewer.OrgID, kind)
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return "", domain.ErrDirectoryInvalid
	}
	return name, nil
}

func (s *Service) AddDirectoryItem(ctx context.Context, viewer domain.User, kind domain.DirectoryKind, name string) (domain.DirectoryItem, error) {
	if !viewer.Can(domain.PermEditUser) {
		return domain.DirectoryItem{}, domain.ErrPermissionDenied
	}
	if !kind.Valid() {
		return domain.DirectoryItem{}, domain.ErrDirectoryInvalid
	}
	name, err := cleanName(name)
	if err != nil {
		return domain.DirectoryItem{}, err
	}
	item, err := s.directory.Create(ctx, viewer.OrgID, kind, name)
	if errors.Is(err, domain.ErrDuplicate) {
		return item, domain.ErrDirectoryDuplicate
	}
	return item, err
}

func (s *Service) RenameDirectoryItem(ctx context.Context, viewer domain.User, id int64, name string) (domain.DirectoryItem, error) {
	if !viewer.Can(domain.PermEditUser) {
		return domain.DirectoryItem{}, domain.ErrPermissionDenied
	}
	name, err := cleanName(name)
	if err != nil {
		return domain.DirectoryItem{}, err
	}
	item, err := s.directory.Rename(ctx, viewer.OrgID, id, name)
	switch {
	case errors.Is(err, domain.ErrDuplicate):
		return item, domain.ErrDirectoryDuplicate
	case errors.Is(err, domain.ErrNoRows):
		return item, domain.ErrDirectoryNotFound
	}
	return item, err
}

func (s *Service) DeleteDirectoryItem(ctx context.Context, viewer domain.User, id int64) error {
	if !viewer.Can(domain.PermEditUser) {
		return domain.ErrPermissionDenied
	}
	return usecase.NotFoundAs(s.directory.Delete(ctx, viewer.OrgID, id), domain.ErrDirectoryNotFound)
}

// --- comments and feedback ---

// canReadNotes: internal comments are for colleagues with the permission, never for the person.
func (s *Service) canReadNotes(viewer domain.User, userID int64) bool {
	return viewer.ID != userID && viewer.Can(domain.PermViewUserComments)
}

func (s *Service) Comments(ctx context.Context, viewer domain.User, userID int64) ([]domain.PersonComment, error) {
	if !s.canReadNotes(viewer, userID) {
		return nil, domain.ErrPermissionDenied
	}
	items, err := s.notes.Comments(ctx, viewer.OrgID, userID)
	for i := range items {
		if items[i].Anonymous {
			items[i].AuthorID, items[i].AuthorName = nil, ""
		}
	}
	return items, err
}

func (s *Service) AddComment(ctx context.Context, viewer domain.User, userID int64, kind, body string, anonymous bool) (domain.PersonComment, error) {
	if !s.canReadNotes(viewer, userID) {
		return domain.PersonComment{}, domain.ErrPermissionDenied
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 5000 || !slices.Contains(domain.CommentKinds, kind) {
		return domain.PersonComment{}, domain.ErrCommentInvalid
	}
	if _, err := s.users.InOrg(ctx, viewer.OrgID, userID); err != nil {
		return domain.PersonComment{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	author := viewer.ID
	c := domain.PersonComment{UserID: userID, AuthorID: &author, AuthorName: viewer.Name, Kind: kind, Body: body, Anonymous: anonymous}
	if err := s.notes.AddComment(ctx, viewer.OrgID, &c); err != nil {
		return c, err
	}
	if c.Anonymous {
		c.AuthorID, c.AuthorName = nil, ""
	}
	return c, nil
}

// DeleteComment: the author, or HR.
func (s *Service) DeleteComment(ctx context.Context, viewer domain.User, id int64) error {
	c, err := s.notes.Comment(ctx, viewer.OrgID, id)
	if err != nil {
		return usecase.NotFoundAs(err, domain.ErrCommentNotFound)
	}
	if (c.AuthorID == nil || *c.AuthorID != viewer.ID) && !viewer.Can(domain.PermEditUser) {
		return domain.ErrPermissionDenied
	}
	return s.notes.DeleteComment(ctx, viewer.OrgID, id)
}

// Feedback (360) is written by colleagues and read by them and by the person it is about.
func (s *Service) Feedback(ctx context.Context, viewer domain.User, userID int64) ([]domain.Feedback, error) {
	if viewer.ID != userID && !viewer.Can(domain.PermViewUserComments) {
		return nil, domain.ErrPermissionDenied
	}
	items, err := s.notes.Feedback(ctx, viewer.OrgID, userID)
	for i := range items {
		if items[i].Anonymous {
			items[i].AuthorID, items[i].AuthorName = nil, ""
		}
	}
	return items, err
}

func (s *Service) AddFeedback(ctx context.Context, viewer domain.User, userID int64, f domain.Feedback) (domain.Feedback, error) {
	if !s.canReadNotes(viewer, userID) {
		return domain.Feedback{}, domain.ErrPermissionDenied
	}
	f.StartDoing, f.StopDoing, f.KeepDoing = strings.TrimSpace(f.StartDoing), strings.TrimSpace(f.StopDoing), strings.TrimSpace(f.KeepDoing)
	if f.StartDoing+f.StopDoing+f.KeepDoing == "" || len(f.StartDoing)+len(f.StopDoing)+len(f.KeepDoing) > 6000 {
		return domain.Feedback{}, domain.ErrCommentInvalid
	}
	if _, err := s.users.InOrg(ctx, viewer.OrgID, userID); err != nil {
		return domain.Feedback{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	author := viewer.ID
	f.UserID, f.AuthorID, f.AuthorName = userID, &author, viewer.Name
	if err := s.notes.AddFeedback(ctx, viewer.OrgID, &f); err != nil {
		return f, err
	}
	if f.Anonymous {
		f.AuthorID, f.AuthorName = nil, ""
	}
	return f, nil
}
