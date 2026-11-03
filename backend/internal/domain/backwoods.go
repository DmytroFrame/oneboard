package domain

import "time"

// Backup is a compressed dump of the database in the private bucket.
type Backup struct {
	ID        int64
	Key       string
	Size      int64
	Tables    int
	Author    *User // nil: made by the schedule (CRON)
	CreatedAt time.Time
}

// DBSchema describes the database without any values, to check migrations (sent and saved as JSON).
type DBSchema struct {
	Database         string              `json:"database"`
	MigrationVersion int64               `json:"migrationVersion"`
	Tables           []TableSchema       `json:"tables"`
	UsersByRole      map[CompanyRole]int `json:"usersByRole"`
	HashVersions     map[string]int      `json:"hashVersions"` // scheme and cost, e.g. "bcrypt $2a$ cost 10"
}

type TableSchema struct {
	Name    string         `json:"name"`
	Rows    int64          `json:"rows"`
	Columns []ColumnSchema `json:"columns"`
	Indexes []IndexSchema  `json:"indexes"`
}

type ColumnSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  bool   `json:"hasDefault"`
}

type IndexSchema struct {
	Name       string `json:"name"`
	Unique     bool   `json:"unique"`
	Definition string `json:"definition"`
}

// RuntimeVersion: what the app or a dependency runs right now ("" = unknown).
type RuntimeVersion struct {
	Name    string
	Version string
}

// ProjectFinding is a project matched by an admin check, with what was found in it.
type ProjectFinding struct {
	ProjectID int64
	Name      string
	Active    bool
	Count     int
	Tasks     []FoundTask
}

type FoundTask struct {
	ID          int64
	Key         string
	Title       string
	SpecialType SpecialType
}
