# OmiTeam features in OneBoard

The OmiTeam requirements (`new-req-docs/`: an ERP/HRM system reverse-engineered from its production
build) are added to OneBoard **as new features**. Nothing OneBoard already does is removed: real-time
boards, live cursors, collaborative editing, voice rooms, organizations, attachments, PWA, three
languages.

## Decisions

| # | Question | Decision |
|---|---|---|
| 1 | OmiTeam's company roles vs OneBoard's organizations | **Superseded by 17.** First: global company roles with organizations as project groups. |
| 2 | Design | OneBoard's Liquid Glass design system. OmiTeam pages are rebuilt from their content and behaviour, not their look. |
| 3 | API | OneBoard conventions: `/api/...`, numeric ids, `{error, code}`. OmiTeam's endpoint tables are a list of operations, not a wire contract. |
| 4 | Languages | English, Ukrainian and German; every new string in all three. |
| 5 | Registration | **By invitation only** (see 17): an email with a pending invitation can sign up and joins with the invited role. The very first account of an installation founds the first organization. `OPEN_REGISTRATION` is gone. |
| 6 | Open questions in the guide (§15) | Its recommended defaults: `/candidates` and `/statistics` gated by `canViewCandidates` / `canAccessStatistics`; 5-minute steps in time logs; pages of 20; vacation days count working days without weekends and holidays; milestone status computed from dates; no "daily report" widget. |
| 7 | Known bugs of the original (§13) | Not reproduced: route-pattern matching for the active menu item, deep links kept through sign-in, DELETE bodies, encoded query strings, error boundary on lazy chunks. |
| 8 | Out of scope until requested | Slack integration and GitHook smart commits (external systems); "What's new" changelog dialog. |
| 9 | Permission matrix | Copied from business-logic.md §1.4, except that OWNER also gets `seeFilterInVacationStatistics` and `deleteItemsFromStatistics` (the original gave them to ADMIN but not to OWNER, who has "full access"). `showBirthdays` still excludes OWNER, as documented. `canAccessStatistics` (HR statistics) is kept as specified: OWNER, ADMIN, SALES, LEAD — not HR; worth reviewing. |
| 10 | Role changes | Nobody changes their own role or the owner's; OWNER and CANDIDATE are never assigned through the editor; a role can be changed only by someone who could have granted the current one (HR can't demote an admin). |
| 11 | Existing data | Migration 00004 makes the oldest account `OWNER` and everyone else `EMPLOYEE`. |
| 12 | Reports access | Time report: everyone with the section; contractors see only their own logs, people without `viewAllProjects` only projects they are in. Billability and bench: staff (`viewCrewBlock`). Periods up to 93 days. A log is billable when both its project and its task are. Bench % = (norm − billable − vacation − day off − sick day) ÷ norm; the team's figure divides sums, never averages percentages; "not tracked" counts only days that have come. The CSV export neutralises cells that would start a spreadsheet formula. |
| 13 | Task statistics | Not specified beyond its controls, so: per week/month/year, tasks created, tasks moved into a "done" column, and the average time tasks stayed in the chosen columns (all but "done" by default), from the task history. |
| 14 | Candidates | A candidate is a `CANDIDATE` account (added like any person, so it gets a profile with contacts and links) plus recruiting data in `candidates`. Rejection uses the documented reasons; hiring changes the role, sets the start date and records the hire. `hr_events` records hires (people added with a staff role, hired candidates), dismissals and rejections; entries can be removed from the statistics without touching the person. |
| 15 | Wiki access | Owner edits; then the access list of the article, or of its nearest ancestor that doesn't inherit: people and groups as readers or editors, "everyone in the company" (staff) as readers or editors. Project articles are also editable by the project's members. Contractors and candidates have no wiki. Moving an article needs edit rights on the new parent and can't create a cycle. Groups are managed by their creator or company owners/admins; deleting one removes it from every article. |
| 16 | Backups | No `pg_dump` dependency: a tar.gz with `schema.json` and one CSV per table (`COPY … TO STDOUT`), all from one read-only REPEATABLE READ snapshot, uploaded under `backups/` in the private bucket and never deleted. An advisory lock keeps instances from dumping at once; every instance checks every 10 minutes whether a scheduled (CRON) backup is due. Restore is manual (`COPY … FROM` in dependency order). |
| 17 | Organizations are tenants | Each organization is an isolated workspace. A person can belong to several and switches between them; every request carries `X-Organization` and the middleware puts the user's membership there (company role, LEAD, active, dismissal) on `domain.User` (`OrgID`, `Role`…), so every `user.Can()` and every query is about that organization. The owner/admin/member org roles are replaced by the company roles (`OWNER`… `CANDIDATE`); an organization can have several owners and always keeps one. Per organization: members and their roles, HR profiles, documents, comments and feedback, reference lists, public holidays, absence requests, candidates, HR events, wiki articles and groups, and everything under its projects. Shared across organizations, owned by the person: the account (name, email, password, avatar, settings), so an organization may not reset the password of, or rename, someone who also works elsewhere (`shared_account`). Asking for an organization one is no longer active in returns `org_unavailable` and the client starts over in another. Migration 00014 converts existing data (memberships take the org rank or the former company role; profiles and reference lists are copied to every organization of their person). |
| 18 | Platform admins | Database backups, the schema and runtime versions concern the whole installation, so they belong to platform admins (`PLATFORM_ADMINS`, emails), not to any organization role. Organization owners and admins keep the data checks of the admin panel, limited to their organization. |

## Mapping

| OmiTeam | OneBoard today | Work |
|---|---|---|
| Roles & permissions (`RolePermissions`) | org/project roles | new: company roles; the matrix lives only in `domain/company.go`, the UI gets the granted permissions from `/api/auth/me` |
| Sign-in, refresh, logout | ✔ (JWT + rotating refresh cookie, cross-tab safe) | extend: password reset by email, change password, admin reset, logout confirmation |
| Dashboard `/dashboard` | `/home` (stats, my tasks) | extend `/home` with the OmiTeam widgets |
| Projects `/projects` | organizations → projects | extend: client, start date, active, billable, pinned, project admins, budget, sections |
| Board + task modal | real-time board, Yjs editor, attachments | extend: tags, story points, budget, billable, special type, checklists, comments, history, templates, bulk move, column statuses |
| Backlog, milestones, sprints, versions | — | new |
| Time tracking, reports (time, billability, bench) | — | new |
| Team, profiles, documents, feedback | settings page, avatars | new (profile extends settings) |
| Candidates, HR statistics | — | new |
| Vacations, public holidays | — | new |
| Wiki (+ groups, access, history) | — | new; editor reuses TipTap |
| Admin panel "Backwoods" | — | new: `pg_dump` backups to S3, Postgres schema, runtime versions |

## Phases

| Phase | Scope | Status |
|---|---|---|
| 1 | Company roles: schema, permission matrix, role in the session, role management endpoint, sections by role, 403/404 pages | done |
| 2 | Accounts: password reset by email (SMTP, Mailpit in compose), change password, admin reset, deactivate / dismiss | done |
| 3 | Team & profiles: directory with filters, profile tabs, documents, comments and feedback | done |
| 4 | Projects: OmiTeam fields, pinning, project admins | done |
| 5 | Board & tasks: tags, story points, checklists, comments, history, templates, bulk move, column statuses | done |
| 6 | Backlog, milestones, sprints, versions | done |
| 7 | Time tracking | done |
| 8 | Vacations & holidays | done |
| 9 | Dashboard widgets | done |
| 10 | Reports: time, billability, bench, task statistics | done |
| 11 | Candidates & HR statistics | done |
| 12 | Wiki | done |
| 13 | Backwoods | done |

New backend contexts (see [architecture.md](architecture.md)): **People** (profiles, documents, feedback,
candidates), **Time** (time logs, reports), **Absence** (vacations, holidays), **Knowledge** (wiki),
**Admin** (backwoods). Each follows the same rules: its own use case package and tables, ports for
everything outside it, events through `internal/contract`.
