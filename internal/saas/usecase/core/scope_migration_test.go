package core

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLMigratedUnitScopeDoesNotIncludeDepartmentMembers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var departmentA, departmentB, childDepartment, childUnit organization.Org
	var departmentUser, childDepartmentUser, childUnitUser identity.User
	f.write(func(u ports.Unit) error {
		for i, org := range []*organization.Org{&departmentA, &departmentB, &childDepartment, &childUnit} {
			*org = organization.Org{TenantID: f.ta.ID, ParentID: f.orgA.ID, UnitID: f.orgA.ID, Name: fmt.Sprintf("scope-org-%d", i), Type: "section", Status: 1}
			if i == 2 {
				org.ParentID = departmentA.ID
			}
			if i == 3 {
				org.Type = "unit"
			}
			if err := u.Save(ports.Orgs, org); err != nil {
				return err
			}
			if i == 3 {
				org.UnitID = org.ID
				if err := u.Save(ports.Orgs, org); err != nil {
					return err
				}
			}
		}
		for i, pair := range []struct {
			user *identity.User
			org  organization.Org
		}{{&departmentUser, departmentB}, {&childDepartmentUser, childDepartment}, {&childUnitUser, childUnit}} {
			*pair.user = identity.User{Name: fmt.Sprintf("scope-user-%d", i), Phone: fmt.Sprintf("1388888800%d", i), Status: 1, PasswordHash: f.a.PasswordHash}
			if err := u.Save(ports.Users, pair.user); err != nil {
				return err
			}
			if err := u.Save(ports.Members, &tenant.Member{TenantID: f.ta.ID, UserID: pair.user.ID, Status: 1}); err != nil {
				return err
			}
			if err := u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: f.ta.ID, UserID: pair.user.ID, OrgID: pair.org.ID}); err != nil {
				return err
			}
		}
		return u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: f.ta.ID, UserID: f.shared.ID, OrgID: departmentA.ID})
	})
	check := func(code int32, want []int64) {
		t.Helper()
		f.write(func(u ports.Unit) error {
			grant, err := first[access.Grant](u, ports.Grants, filter("role_id", f.viewerA.ID, "resource_id", f.resources["membership.read"].ID))
			if err != nil {
				return err
			}
			grant.DataScope = code
			return u.Save(ports.Grants, &grant)
		})
		op := f.ops["membership.read"]
		got, err := f.svc.CheckAccess(ctx, &c.AccessRequest{Context: f.sc, AppCode: f.app.Code, Method: op.Method, Path: op.Path})
		if err != nil {
			t.Fatal(err)
		}
		actual := append([]int64{}, got.UserIDs...)
		expected := append([]int64{}, want...)
		sort.Slice(actual, func(i, j int) bool { return actual[i] < actual[j] })
		sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })
		if got.AllWithinTenant || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("scope %d got all=%v users=%v; want %v", code, got.AllWithinTenant, actual, expected)
		}
	}
	// Legacy scope 2 grants direct membership of the unit node, not all departments.
	check(2, []int64{f.a.ID})
	// Legacy scope 1 includes same-unit departments, not independently owned child units.
	check(1, []int64{f.a.ID, f.shared.ID, departmentUser.ID, childDepartmentUser.ID})
	check(3, []int64{f.shared.ID, childDepartmentUser.ID})
	check(4, []int64{f.shared.ID})
	check(5, []int64{f.shared.ID})
	f.write(func(u ports.Unit) error {
		return u.Delete(ports.MemberOrgs, filter("tenant_id", f.ta.ID, "user_id", f.shared.ID))
	})
	// Never reproduce the legacy empty-org-list => all-tenant-users fallback.
	for _, code := range []int32{1, 2, 3, 4} {
		check(code, nil)
	}
}
