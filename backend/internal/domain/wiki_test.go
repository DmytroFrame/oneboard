package domain

import "testing"

func TestWikiAccess(t *testing.T) {
	owner, reader, grouped, outsider := int64(1), int64(2), int64(3), int64(4)
	root, child, grandchild, project := int64(10), int64(11), int64(12), int64(20)
	pid := int64(7)
	tree := NewWikiTree([]WikiArticle{
		{ID: root, OwnerID: &owner, CompanyAccess: WikiNone, Grants: []WikiGrant{{UserID: reader, Level: WikiRead}, {GroupID: 5, Level: WikiEdit}}},
		{ID: child, ParentID: &root, OwnerID: &owner, InheritAccess: true},
		{ID: grandchild, ParentID: &child, OwnerID: &owner, InheritAccess: true},
		{ID: project, ProjectID: &pid, OwnerID: &owner, CompanyAccess: WikiRead},
	})
	staff := func(id int64) WikiViewer { return WikiViewer{User: User{ID: id, Role: CompanyEmployee}} }
	cases := []struct {
		name    string
		article int64
		viewer  WikiViewer
		want    WikiLevel
	}{
		{"owner", root, staff(owner), WikiEdit},
		{"reader", root, staff(reader), WikiRead},
		{"reader inherits two levels down", grandchild, staff(reader), WikiRead},
		{"group member edits", grandchild, WikiViewer{User: User{ID: grouped, Role: CompanyEmployee}, Groups: []int64{5}}, WikiEdit},
		{"outsider", child, staff(outsider), WikiNone},
		{"company read", project, staff(outsider), WikiRead},
		{"project member edits", project, WikiViewer{User: User{ID: outsider, Role: CompanyEmployee}, Projects: []int64{pid}}, WikiEdit},
		{"contractors have no wiki", project, WikiViewer{User: User{ID: outsider, Role: CompanyContractor}, Projects: []int64{pid}}, WikiNone},
	}
	for _, c := range cases {
		if got := tree.Level(tree[c.article], c.viewer); got != c.want {
			t.Errorf("%s: level = %q, want %q", c.name, got, c.want)
		}
	}
	if !tree.WouldCycle(root, grandchild) || tree.WouldCycle(grandchild, root) {
		t.Error("moving root under its grandchild must be a cycle, the opposite must not")
	}
	if p := tree.Path(tree[grandchild]); len(p) != 2 || p[0].ID != root || p[1].ID != child {
		t.Errorf("path = %v", p)
	}
}
