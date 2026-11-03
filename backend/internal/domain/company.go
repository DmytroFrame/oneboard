package domain

import "slices"

// CompanyRole is a user's role in the company, on their account. It governs company-wide features
// (people, HR, time, reports, wiki, admin). Organizations and projects keep their own roles
// (OrgRole, Role) for access to boards.
type CompanyRole string

const (
	CompanyOwner      CompanyRole = "OWNER"
	CompanyAdmin      CompanyRole = "ADMIN"
	CompanyManager    CompanyRole = "MANAGER"
	CompanyRecruiter  CompanyRole = "RECRUITER"
	CompanyHR         CompanyRole = "HR"
	CompanySales      CompanyRole = "SALES"
	CompanyEmployee   CompanyRole = "EMPLOYEE"
	CompanyContractor CompanyRole = "CONTRACTOR"
	CompanyAccountant CompanyRole = "ACCOUNTANT"
	CompanyCandidate  CompanyRole = "CANDIDATE"
)

// CompanyRoles in the order they are listed in the UI.
var CompanyRoles = []CompanyRole{
	CompanyOwner, CompanyAdmin, CompanyManager, CompanyRecruiter, CompanyHR,
	CompanySales, CompanyEmployee, CompanyContractor, CompanyAccountant, CompanyCandidate,
}

func (r CompanyRole) Valid() bool { return slices.Contains(CompanyRoles, r) }

// AdditionRole widens a user's permissions on top of their role.
type AdditionRole string

const AdditionLead AdditionRole = "LEAD"

func (r AdditionRole) Valid() bool { return r == AdditionLead }

// Permission names a company-wide ability. The server is the only place the matrix lives: it enforces
// it, and /api/auth/me tells the UI which permissions the user has, so the UI can hide what would be
// refused without a copy of its own.
type Permission string

const (
	PermAddUser               Permission = "addUser"
	PermEditUser              Permission = "editUser"
	PermFireUser              Permission = "fireUser"
	PermEditGrades            Permission = "editGrades"
	PermViewSalary            Permission = "viewSalary"
	PermViewCandidates        Permission = "viewCandidates"
	PermAddDocs               Permission = "addDocs"
	PermResetPassword         Permission = "resetPassword"
	PermFireYourself          Permission = "fireYourself"
	PermEditProjectComment    Permission = "editProjectComment"
	PermAddTask               Permission = "addTask"
	PermOpenBackwoods         Permission = "openBackwoods"
	PermEditProfile           Permission = "editProfile"
	PermEditCandidate         Permission = "editCandidate"
	PermViewGrades            Permission = "viewGrades"
	PermViewMoreTeamInfo      Permission = "viewMoreTeamInfo"
	PermViewUserComments      Permission = "viewUserComments"
	PermCreateWikiArticle     Permission = "createWikiArticle"
	PermViewCrewBlock         Permission = "viewCrewBlock"
	PermAddOwnDocs            Permission = "addOwnDocs"
	PermDeleteDocs            Permission = "deleteDocs"
	PermEditDocs              Permission = "editDocs"
	PermSeeHRInformation      Permission = "seeHRInformation"
	PermChangeAdditionRole    Permission = "changeAdditionRole"
	PermViewPrivateDocs       Permission = "viewPrivateDocs"
	PermSeeProjectMembers     Permission = "seeProjectMembers"
	PermAddProjectMembers     Permission = "addProjectMembers"
	PermAddProject            Permission = "addProject"
	PermEditProject           Permission = "editProject"
	PermEditAllStatuses       Permission = "editAllStatuses"
	PermAddProjectCharters    Permission = "addProjectCharters"
	PermEditWorklogs          Permission = "editWorklogs"
	PermSeeVacationsRequests  Permission = "seeVacationsRequests"
	PermViewAllProjects       Permission = "viewAllProjects"
	PermEditBudget            Permission = "editBudget"
	PermEditHolidays          Permission = "editHolidays"
	PermViewTrialUser         Permission = "viewTrialUser"
	PermAccessStatistics      Permission = "accessStatistics"
	PermVacationStatsFilter   Permission = "seeFilterInVacationStatistics"
	PermDeleteStatisticsItems Permission = "deleteItemsFromStatistics"
	PermViewGlobalSearch      Permission = "viewGlobalSearch"
	PermShowBirthdays         Permission = "showBirthdays"
	PermEditUserFields        Permission = "editUserFields"
)

type permissionRule struct {
	roles []CompanyRole
	lead  bool // also granted by the LEAD addition role
}

// Role groups that share many permissions.
var (
	oa         = []CompanyRole{CompanyOwner, CompanyAdmin}
	oaRH       = []CompanyRole{CompanyOwner, CompanyAdmin, CompanyRecruiter, CompanyHR}
	oaMgr      = []CompanyRole{CompanyOwner, CompanyAdmin, CompanyManager}
	oaMRHS     = []CompanyRole{CompanyOwner, CompanyAdmin, CompanyManager, CompanyRecruiter, CompanyHR, CompanySales}
	staff      = append(slices.Clone(oaMRHS), CompanyEmployee, CompanyAccountant) // everyone but contractors and candidates
	everyoneIn = append(slices.Clone(staff), CompanyCandidate)
)

// The permission matrix of OmiTeam's RolePermissions (new-req-docs/business-logic.md §1.4). Deviations,
// recorded in docs/omiteam-integration.md: OWNER ("full access") also gets the vacation-statistics filter
// and deleting statistics items, which the original granted to ADMIN alone.
var permissions = map[Permission]permissionRule{
	PermAddUser:        {roles: oaRH},
	PermEditUser:       {roles: oaRH},
	PermFireUser:       {roles: oaRH},
	PermEditGrades:     {roles: oaRH},
	PermViewSalary:     {roles: oaRH},
	PermViewCandidates: {roles: oaRH},
	PermAddDocs:        {roles: oaRH},

	PermResetPassword:      {roles: oa},
	PermFireYourself:       {roles: oa},
	PermEditProjectComment: {roles: oa},
	PermAddTask:            {roles: oa},
	PermOpenBackwoods:      {roles: oa},

	PermEditProfile:   {roles: staff, lead: true},
	PermEditCandidate: {roles: append(slices.Clone(oaMRHS), CompanyEmployee), lead: true},

	PermViewGrades:       {roles: oaMRHS, lead: true},
	PermViewMoreTeamInfo: {roles: oaMRHS, lead: true},

	PermViewUserComments:  {roles: staff, lead: true},
	PermCreateWikiArticle: {roles: staff, lead: true},
	PermViewCrewBlock:     {roles: staff, lead: true},
	PermAddOwnDocs:        {roles: everyoneIn},

	PermDeleteDocs:         {roles: oaRH},
	PermEditDocs:           {roles: oaRH},
	PermSeeHRInformation:   {roles: oaRH},
	PermChangeAdditionRole: {roles: oaRH},
	PermViewPrivateDocs:    {roles: append(slices.Clone(oaRH), CompanySales)},

	PermSeeProjectMembers: {roles: oaMRHS, lead: true},

	PermAddProjectMembers:    {roles: oaMgr, lead: true},
	PermAddProject:           {roles: oaMgr, lead: true},
	PermEditProject:          {roles: oaMgr, lead: true},
	PermEditAllStatuses:      {roles: oaMgr, lead: true},
	PermAddProjectCharters:   {roles: oaMgr, lead: true},
	PermEditWorklogs:         {roles: oaMgr, lead: true},
	PermSeeVacationsRequests: {roles: oaMgr, lead: true},

	PermViewAllProjects: {roles: append(slices.Clone(oaMgr), CompanyHR), lead: true},
	PermEditBudget:      {roles: oaMgr},
	PermEditHolidays:    {roles: []CompanyRole{CompanyOwner, CompanyAdmin, CompanyHR}},
	PermViewTrialUser:   {roles: []CompanyRole{CompanyOwner, CompanyAdmin, CompanyManager, CompanyRecruiter, CompanyHR}, lead: true},

	PermAccessStatistics:      {roles: []CompanyRole{CompanyOwner, CompanyAdmin, CompanySales}, lead: true},
	PermVacationStatsFilter:   {roles: []CompanyRole{CompanyOwner, CompanyAdmin, CompanyManager, CompanyHR}, lead: true},
	PermDeleteStatisticsItems: {roles: oa},

	PermViewGlobalSearch: {roles: everyoneIn},
	// Birthdays are hidden from the owner and candidates on purpose.
	PermShowBirthdays: {roles: []CompanyRole{CompanyAdmin, CompanyManager, CompanyRecruiter, CompanyHR, CompanySales, CompanyEmployee, CompanyAccountant, CompanyContractor}},
	PermEditUserFields: {roles: []CompanyRole{
		CompanyOwner, CompanyAdmin, CompanyManager, CompanyRecruiter, CompanyHR, CompanySales, CompanyAccountant, CompanyContractor,
	}},
}

// Permissions lists every permission, for tests and the API.
func Permissions() []Permission {
	out := make([]Permission, 0, len(permissions))
	for p := range permissions {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// Can reports whether the user has a company permission.
func (u User) Can(p Permission) bool {
	rule, ok := permissions[p]
	if !ok {
		return false
	}
	return slices.Contains(rule.roles, u.Role) || (rule.lead && u.IsLead())
}

func (u User) IsLead() bool { return slices.Contains(u.AdditionRoles, AdditionLead) }

// assignableRoles are the roles that can be given through the role editor: OWNER only by an owner
// (co-owners of an organization), never CANDIDATE (a candidate becomes someone by being hired).
var assignableRoles = []CompanyRole{
	CompanyAdmin, CompanyManager, CompanyRecruiter, CompanyHR, CompanySales, CompanyEmployee, CompanyContractor, CompanyAccountant,
}

// AssignableRoles are the roles this user may give to someone: owners any role but candidate, admins
// any but owner, recruiters, HR and leads the same except ADMIN, everyone else none.
func (u User) AssignableRoles() []CompanyRole {
	switch {
	case u.Role == CompanyOwner:
		return append([]CompanyRole{CompanyOwner}, assignableRoles...)
	case u.Role == CompanyAdmin:
		return slices.Clone(assignableRoles)
	case u.Role == CompanyRecruiter || u.Role == CompanyHR || u.IsLead():
		return slices.DeleteFunc(slices.Clone(assignableRoles), func(r CompanyRole) bool { return r == CompanyAdmin })
	}
	return nil
}

// CheckRoleChange validates that actor may change target's role to role (and, when additions is not
// nil, their addition roles). Someone's role can only be changed by a person who could have given them
// their current one, so HR can't demote an admin, only owners change owners, and nobody changes their
// own role. Keeping at least one owner is the organization use case's job.
func CheckRoleChange(actor, target User, role CompanyRole, additions []AdditionRole) error {
	if !role.Valid() {
		return ErrCompanyRoleInvalid
	}
	for _, a := range additions {
		if !a.Valid() {
			return ErrCompanyRoleInvalid
		}
	}
	if actor.ID == target.ID {
		return ErrRoleChangeForbidden
	}
	allowed := actor.AssignableRoles()
	if role != target.Role && !slices.Contains(allowed, role) {
		return ErrRoleChangeForbidden
	}
	if target.Role != CompanyCandidate && !slices.Contains(allowed, target.Role) {
		return ErrRoleChangeForbidden
	}
	if additions != nil && !slices.Equal(additions, target.AdditionRoles) && !actor.Can(PermChangeAdditionRole) {
		return ErrRoleChangeForbidden
	}
	return nil
}
