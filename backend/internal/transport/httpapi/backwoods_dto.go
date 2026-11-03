package httpapi

import (
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/backwoods"
)

type backupDTO struct {
	ID         int64              `json:"id"`
	Size       int64              `json:"size"`
	Tables     int                `json:"tables"`
	AuthorName string             `json:"authorName"` // "" = CRON
	AuthorRole domain.CompanyRole `json:"authorRole,omitempty"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type backupStateDTO struct {
	Backups []backupDTO `json:"backups"`
	NextAt  *time.Time  `json:"nextAt"`
}

func backupStateDTOOf(st backwoods.BackupState) backupStateDTO {
	return backupStateDTO{
		Backups: mapSlice(nonNil(st.Backups), func(b domain.Backup) backupDTO {
			d := backupDTO{ID: b.ID, Size: b.Size, Tables: b.Tables, CreatedAt: b.CreatedAt}
			if b.Author != nil {
				d.AuthorName, d.AuthorRole = b.Author.Name, b.Author.Role
			}
			return d
		}),
		NextAt: st.NextAt,
	}
}

type foundTaskDTO struct {
	ID          int64              `json:"id"`
	Key         string             `json:"key"`
	Title       string             `json:"title"`
	SpecialType domain.SpecialType `json:"specialType"`
}

type projectFindingDTO struct {
	ProjectID int64          `json:"projectId"`
	Name      string         `json:"name"`
	Active    bool           `json:"active"`
	Count     int            `json:"count"`
	Tasks     []foundTaskDTO `json:"tasks"`
}

func projectFindingDTOOf(f domain.ProjectFinding) projectFindingDTO {
	return projectFindingDTO{f.ProjectID, f.Name, f.Active, f.Count, mapSlice(nonNil(f.Tasks), func(t domain.FoundTask) foundTaskDTO {
		return foundTaskDTO{t.ID, t.Key, t.Title, t.SpecialType}
	})}
}
