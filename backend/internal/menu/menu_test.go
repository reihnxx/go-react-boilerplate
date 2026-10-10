package menu

import (
	"testing"

	"github.com/google/uuid"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
)

func ptr[T any](v T) *T { return &v }

func TestSlugFromName(t *testing.T) {
	cases := map[string]string{
		"Data Siswa":        "data_siswa",
		"  Laporan -- 2024": "laporan_2024",
		"123 Angka":         "angka",
		"Ünïcode Ménu":      "n_code_m_nu",
	}
	for in, want := range cases {
		if got := SlugFromName(in); got != want {
			t.Errorf("SlugFromName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildNavigation(t *testing.T) {
	gHome, gAdmin, gEmpty := uuid.New(), uuid.New(), uuid.New()
	groups := []Group{{ID: gHome, Name: "Beranda"}, {ID: gAdmin, Name: "Manajemen"}, {ID: gEmpty, Name: "Kosong"}}

	dashboard := Menu{ID: uuid.New(), GroupID: &gHome, Slug: "dashboard", Name: "Beranda", Path: ptr("/dashboard"), IsActive: true}
	reports := Menu{ID: uuid.New(), GroupID: &gAdmin, Slug: "reports", Name: "Laporan", IsActive: true} // parent, no path
	reportA := Menu{ID: uuid.New(), ParentID: &reports.ID, Slug: "report_a", Name: "A", Path: ptr("/reports/a"), IsActive: true}
	reportB := Menu{ID: uuid.New(), ParentID: &reports.ID, Slug: "report_b", Name: "B", Path: ptr("/reports/b"), IsActive: true}
	inactive := Menu{ID: uuid.New(), GroupID: &gAdmin, Slug: "old", Name: "Lama", Path: ptr("/old"), IsActive: false}
	hidden := Menu{ID: uuid.New(), GroupID: &gEmpty, Slug: "secret", Name: "Rahasia", Path: ptr("/secret"), IsActive: true}
	orphanParent := Menu{ID: uuid.New(), GroupID: &gAdmin, Slug: "tools", Name: "Tools", IsActive: true}
	menus := []Menu{dashboard, reports, reportA, reportB, inactive, hidden, orphanParent}

	perms := rbac.NewPermissions(uuid.New(), false, map[string][]rbac.Action{
		"dashboard": {rbac.View},
		"reports":   {rbac.View},
		"report_a":  {rbac.View},
		"report_b":  {rbac.Create}, // no view → hidden
		"old":       {rbac.View},   // inactive → hidden
		"tools":     {rbac.View},   // no path, no visible children → hidden
	})

	nav := BuildNavigation(groups, menus, perms)
	if len(nav) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(nav), nav)
	}
	if nav[0].Name != "Beranda" || len(nav[0].Items) != 1 || nav[0].Items[0].Slug != "dashboard" {
		t.Fatalf("unexpected first group %+v", nav[0])
	}
	admin := nav[1]
	if len(admin.Items) != 1 || admin.Items[0].Slug != "reports" {
		t.Fatalf("unexpected admin group %+v", admin)
	}
	if kids := admin.Items[0].Children; len(kids) != 1 || kids[0].Slug != "report_a" {
		t.Fatalf("unexpected children %+v", kids)
	}
}

func TestBuildNavigationChildRequiresVisibleParent(t *testing.T) {
	g := uuid.New()
	parent := Menu{ID: uuid.New(), GroupID: &g, Slug: "parent", Name: "P", IsActive: true}
	child := Menu{ID: uuid.New(), ParentID: &parent.ID, Slug: "child", Name: "C", Path: ptr("/c"), IsActive: true}
	perms := rbac.NewPermissions(uuid.New(), false, map[string][]rbac.Action{"child": {rbac.View}})
	if nav := BuildNavigation([]Group{{ID: g, Name: "G"}}, []Menu{parent, child}, perms); len(nav) != 0 {
		t.Fatalf("child without visible parent must be hidden, got %+v", nav)
	}
}

func TestBuildNavigationSuperAdminSkipsInactive(t *testing.T) {
	g := uuid.New()
	menus := []Menu{
		{ID: uuid.New(), GroupID: &g, Slug: "a", Name: "A", Path: ptr("/a"), IsActive: true},
		{ID: uuid.New(), GroupID: &g, Slug: "b", Name: "B", Path: ptr("/b"), IsActive: false},
	}
	nav := BuildNavigation([]Group{{ID: g, Name: "G"}}, menus, rbac.NewPermissions(uuid.New(), true, nil))
	if len(nav) != 1 || len(nav[0].Items) != 1 || nav[0].Items[0].Slug != "a" {
		t.Fatalf("unexpected %+v", nav)
	}
}
