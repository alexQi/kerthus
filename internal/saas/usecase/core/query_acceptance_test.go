package core

import (
	"context"
	"testing"

	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLMemberSearchCombinesIdentityOrganizationAndRoleBeforePagination(t *testing.T) {
	f := newFixture(t)
	var child, grandchild, empty organization.Org
	f.write(func(u ports.Unit) error {
		f.a.Name, f.shared.Name, f.b.Name = "Alice Internal", "Alice Field", "Alice Foreign"
		f.shared.Email = "shared@example.test"
		for _, user := range []*identity.User{&f.a, &f.shared, &f.b} {
			if e := u.Save(ports.Users, user); e != nil {
				return e
			}
		}
		child = organization.Org{TenantID: f.ta.ID, UnitID: f.orgA.ID, ParentID: f.orgA.ID, Name: "Child", Type: "department", Status: 1}
		if e := u.Save(ports.Orgs, &child); e != nil {
			return e
		}
		grandchild = organization.Org{TenantID: f.ta.ID, UnitID: f.orgA.ID, ParentID: child.ID, Name: "Grandchild", Type: "department", Status: 1}
		empty = organization.Org{TenantID: f.ta.ID, UnitID: f.orgA.ID, ParentID: f.orgA.ID, Name: "Empty", Type: "department", Status: 1}
		for _, org := range []*organization.Org{&grandchild, &empty} {
			if e := u.Save(ports.Orgs, org); e != nil {
				return e
			}
		}
		for _, id := range []int64{child.ID, grandchild.ID} {
			if e := u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: f.ta.ID, UserID: f.shared.ID, OrgID: id}); e != nil {
				return e
			}
		}
		return u.Save(ports.Positions, &organization.Position{TenantID: f.ta.ID, OrgID: child.ID, Name: "Child position", Status: 1})
	})
	for _, test := range []struct {
		name  string
		req   c.QueryRequest
		ids   []int64
		total int64
	}{
		{"full-phone", c.QueryRequest{Phone: f.shared.Phone}, []int64{f.shared.ID}, 1},
		{"partial-phone", c.QueryRequest{Phone: "0003"}, []int64{f.shared.ID}, 1},
		{"name-and-phone", c.QueryRequest{Name: "Alice", Phone: "0003"}, []int64{f.shared.ID}, 1},
		{"name-phone-intersection-empty", c.QueryRequest{Name: "Internal", Phone: "0003"}, nil, 0},
		{"foreign-phone-not-visible", c.QueryRequest{Phone: f.b.Phone}, nil, 0},
		{"keyword-email", c.QueryRequest{Search: "shared@example.test"}, []int64{f.shared.ID}, 1},
		{"keyword-phone", c.QueryRequest{Search: "0003"}, []int64{f.shared.ID}, 1},
		{"literal-wildcard", c.QueryRequest{Phone: "%"}, nil, 0},
		{"literal-underscore", c.QueryRequest{Name: "_"}, nil, 0},
		{"trim-search", c.QueryRequest{Name: " Alice ", Phone: " 0003 "}, []int64{f.shared.ID}, 1},
		{"name-pagination", c.QueryRequest{Name: "Alice", Page: 2, PageSize: 1}, []int64{f.shared.ID}, 2},
		{"role-phone-intersection-empty", c.QueryRequest{RoleID: f.viewerA.ID, ForAuthorization: true, Phone: f.a.Phone}, nil, 0},
		{"role-phone", c.QueryRequest{RoleID: f.viewerA.ID, ForAuthorization: true, Phone: f.shared.Phone}, []int64{f.shared.ID}, 1},
		{"direct-organization", c.QueryRequest{OrgID: f.orgA.ID}, []int64{f.a.ID}, 1},
		{"organization-descendants", c.QueryRequest{OrgID: f.orgA.ID, IncludeChildren: true}, []int64{f.a.ID, f.shared.ID}, 2},
		{"organization-descendants-pagination", c.QueryRequest{OrgID: f.orgA.ID, IncludeChildren: true, Page: 2, PageSize: 1}, []int64{f.shared.ID}, 2},
		{"grandchild-members", c.QueryRequest{OrgID: grandchild.ID}, []int64{f.shared.ID}, 1},
		{"empty-organization", c.QueryRequest{OrgID: empty.ID}, nil, 0},
		{"organization-phone-intersection-empty", c.QueryRequest{OrgID: child.ID, Phone: f.a.Phone}, nil, 0},
		{"organization-role-name-intersection", c.QueryRequest{OrgID: f.orgA.ID, IncludeChildren: true, RoleID: f.viewerA.ID, ForAuthorization: true, Name: "Alice", PageSize: 1}, []int64{f.shared.ID}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := test.req
			req.Kind, req.Context = c.KindMembers, f.ac
			out, e := f.svc.Query(context.Background(), &req)
			if e != nil {
				t.Fatal(e)
			}
			if out.Total != test.total || len(out.Members) != len(test.ids) {
				t.Fatalf("filtered page/count mismatch: total=%d members=%d", out.Total, len(out.Members))
			}
			for i, id := range test.ids {
				if out.Members[i].User.ID != id {
					t.Fatal("unexpected member in filtered page")
				}
			}
		})
	}
	for _, kind := range []c.Kind{c.KindMembers, c.KindPositions} {
		_, e := f.svc.Query(context.Background(), &c.QueryRequest{Context: f.ac, Kind: kind, OrgID: f.orgB.ID, IncludeChildren: true})
		wantCode(t, e, 403)
		_, e = f.svc.Query(context.Background(), &c.QueryRequest{Context: f.pc, TargetTenantID: f.ta.ID, Kind: kind, OrgID: f.orgB.ID})
		wantCode(t, e, 403)
	}
	for _, children := range []bool{false, true} {
		out, e := f.svc.Query(context.Background(), &c.QueryRequest{Context: f.ac, Kind: c.KindPositions, OrgID: f.orgA.ID, IncludeChildren: children, PageSize: 1})
		want := int64(1)
		if children {
			want = 2
		}
		if e != nil || out.Total != want || len(out.Positions) != 1 {
			t.Fatal("position organization filter/page/count mismatch", e)
		}
	}
	for _, search := range []string{f.shared.Phone, f.shared.Email, f.shared.Name} {
		out, e := f.svc.Query(context.Background(), &c.QueryRequest{Context: f.pc, Kind: c.KindUsers, Search: search, PageSize: 1})
		if e != nil || out.Total != 1 || len(out.Users) != 1 || out.Users[0].ID != f.shared.ID {
			t.Fatal("platform user keyword did not match identity fields", e)
		}
	}
	out, e := f.svc.Query(context.Background(), &c.QueryRequest{Context: f.pc, Kind: c.KindUsers, Name: "Internal", Phone: f.shared.Phone})
	if e != nil || out.Total != 0 {
		t.Fatal("platform user name/phone filters did not intersect", e)
	}
}
