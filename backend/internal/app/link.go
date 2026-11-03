package app

// Every package that registers fx modules (modules.Register) must be linked into the binary for its
// registration to run. Adding a package means one line here; its wiring stays in the package itself.
import (
	_ "github.com/DmytroFrame/oneboard/backend/internal/config"

	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/bcrypthash"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/imagethumb"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/jwttoken"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/mailer"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/postgres"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/redisstore"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/s3store"
	_ "github.com/DmytroFrame/oneboard/backend/internal/adapter/searchindex"

	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/absence"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/attachment"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/backwoods"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/dashboard"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/hr"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/people"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/project"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/reports"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/tasks"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/timetracking"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/user"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/voice"
	_ "github.com/DmytroFrame/oneboard/backend/internal/usecase/wiki"

	_ "github.com/DmytroFrame/oneboard/backend/internal/transport/httpapi"
)
