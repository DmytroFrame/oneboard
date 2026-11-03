package domain

import (
	"errors"
	"testing"
)

func user(id int64, role CompanyRole, additions ...AdditionRole) User {
	return User{ID: id, Role: role, AdditionRoles: additions}
}

func TestPermissionMatrix(t *testing.T) {
	cases := []struct {
		u    User
		perm Permission
		want bool
	}{
		{user(1, CompanyOwner), PermOpenBackwoods, true},
		{user(1, CompanyAdmin), PermOpenBackwoods, true},
		{user(1, CompanyManager), PermOpenBackwoods, false},
		{user(1, CompanyHR), PermViewCandidates, true},
		{user(1, CompanyEmployee), PermViewCandidates, false},
		{user(1, CompanyEmployee), PermAddProject, false},
		{user(1, CompanyEmployee, AdditionLead), PermAddProject, true},  // LEAD widens
		{user(1, CompanyEmployee, AdditionLead), PermEditBudget, false}, // …but not everywhere
		{user(1, CompanyContractor), PermEditProfile, false},
		{user(1, CompanyCandidate), PermAddOwnDocs, true},
		{user(1, CompanyCandidate), PermViewGlobalSearch, true},
		{user(1, CompanyContractor), PermViewGlobalSearch, false},
		{user(1, CompanyOwner), PermShowBirthdays, false}, // hidden from the owner on purpose
		{user(1, CompanyOwner), PermDeleteStatisticsItems, true},
		{user(1, CompanyEmployee), PermEditUserFields, false},
		{user(1, CompanyAccountant), PermEditUserFields, true},
		{user(1, CompanyOwner), Permission("nonsense"), false},
	}
	for _, c := range cases {
		if got := c.u.Can(c.perm); got != c.want {
			t.Errorf("%s %v Can(%s) = %v, want %v", c.u.Role, c.u.AdditionRoles, c.perm, got, c.want)
		}
	}
}

func TestEveryPermissionHasARole(t *testing.T) {
	for _, p := range Permissions() {
		if len(permissions[p].roles) == 0 {
			t.Errorf("%s is granted to nobody", p)
		}
	}
}

func TestCheckRoleChange(t *testing.T) {
	owner, admin, hr := user(1, CompanyOwner), user(2, CompanyAdmin), user(3, CompanyHR)
	employee, lead := user(4, CompanyEmployee), user(5, CompanyEmployee, AdditionLead)
	candidate := user(6, CompanyCandidate)

	cases := []struct {
		name      string
		actor     User
		target    User
		role      CompanyRole
		additions []AdditionRole
		want      error
	}{
		{"owner promotes to admin", owner, employee, CompanyAdmin, nil, nil},
		{"hr can't make admins", hr, employee, CompanyAdmin, nil, ErrRoleChangeForbidden},
		{"hr can't demote an admin", hr, admin, CompanyEmployee, nil, ErrRoleChangeForbidden},
		{"hr hires a candidate", hr, candidate, CompanyEmployee, nil, nil},
		{"lead may assign manager", lead, employee, CompanyManager, nil, nil},
		{"employee can't change roles", employee, candidate, CompanyEmployee, nil, ErrRoleChangeForbidden},
		{"nobody changes their own role", admin, admin, CompanyManager, nil, ErrRoleChangeForbidden},
		{"admins don't change owners", admin, owner, CompanyAdmin, nil, ErrRoleChangeForbidden},
		{"admins don't make owners", admin, employee, CompanyOwner, nil, ErrRoleChangeForbidden},
		{"an owner makes a co-owner", owner, employee, CompanyOwner, nil, nil},
		{"an owner demotes another owner", owner, User{ID: 99, Role: CompanyOwner}, CompanyAdmin, nil, nil},
		{"hr grants LEAD", hr, employee, CompanyEmployee, []AdditionRole{AdditionLead}, nil},
		{"lead can't grant LEAD", lead, employee, CompanyEmployee, []AdditionRole{AdditionLead}, ErrRoleChangeForbidden},
		{"unknown role", owner, employee, "PIRATE", nil, ErrCompanyRoleInvalid},
		{"unknown addition", owner, employee, CompanyEmployee, []AdditionRole{"KING"}, ErrCompanyRoleInvalid},
	}
	for _, c := range cases {
		if got := CheckRoleChange(c.actor, c.target, c.role, c.additions); !errors.Is(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
