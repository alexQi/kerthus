package core

import (
	"context"
	"slices"
	"testing"

	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLTenantRoleCannotClaimPlatformIdentity(t *testing.T) {
	f := newFixture(t)
	f.write(func(u ports.Unit) error {
		f.viewerA.Code = "admin"
		return u.Save(ports.Roles, &f.viewerA)
	})
	auth, e := f.svc.Auth(context.Background(), f.sc)
	if e != nil {
		t.Fatal(e)
	}
	if auth.PlatformAdmin || slices.Contains(auth.Roles, "admin") || !slices.Contains(auth.Roles, "tenant:admin") {
		t.Fatal("tenant role exposed the console's reserved platform identity")
	}
	_, e = f.svc.Query(context.Background(), &c.QueryRequest{Context: f.sc, Kind: c.KindUsers})
	wantCode(t, e, 403)
	auth, e = f.svc.Auth(context.Background(), f.pc)
	if e != nil || !auth.PlatformAdmin || !slices.Contains(auth.Roles, "admin") {
		t.Fatal("explicit platform identity was not preserved", e)
	}
}

// Exercise revocation through public commands with existing sessions, rather
// than mutating policy tables directly. Every tenant-local revocation also
// checks that the same account's other tenant remains usable.
func TestMySQLLifecycleCommandsRevalidateExistingSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	check := func(c *c.Context, code int32) {
		t.Helper()
		_, e := f.check(c, "membership.read")
		if code == 0 {
			must(e)
		} else {
			wantCode(t, e, code)
		}
	}
	setStatus := func(actor *c.Context, kind c.Kind, id int64, status int32) {
		t.Helper()
		_, e := f.svc.SetStatus(ctx, &c.StatusRequest{Context: actor, Kind: kind, ID: id, Status: status})
		must(e)
	}
	check(f.sc, 0)
	check(f.sharedB, 0)

	t.Run("account-disable-revokes-all-sessions-permanently", func(t *testing.T) {
		oldA, oldB := f.sc, f.sharedB
		setStatus(f.pc, c.KindUsers, f.shared.ID, 0)
		check(oldA, 401)
		check(oldB, 401)
		setStatus(f.pc, c.KindUsers, f.shared.ID, 1)
		check(oldA, 401)
		check(oldB, 401)
		f.read(func(u ports.Unit) error { return u.Get(ports.Users, f.shared.ID, &f.shared) })
		f.sc = f.session(f.shared, f.ta)
		f.sharedB = f.session(f.shared, f.tb)
		check(f.sc, 0)
		check(f.sharedB, 0)
	})
	for _, state := range []struct {
		name  string
		actor *c.Context
		kind  c.Kind
		id    int64
	}{{"member", f.ac, c.KindMembers, f.shared.ID}, {"role", f.ac, c.KindRoles, f.viewerA.ID}, {"tenant", f.pc, c.KindTenants, f.ta.ID}} {
		t.Run(state.name+"-disable-is-tenant-local", func(t *testing.T) {
			setStatus(state.actor, state.kind, state.id, 0)
			check(f.sc, 403)
			check(f.sharedB, 0)
			if state.kind == c.KindRoles {
				auth, e := f.svc.Auth(ctx, f.sc)
				must(e)
				if len(auth.Permissions) != 0 {
					t.Fatal("disabled role still exposed permissions")
				}
			}
			setStatus(state.actor, state.kind, state.id, 1)
			check(f.sc, 0)
		})
	}
	t.Run("role-unbind-revokes-permissions", func(t *testing.T) {
		for _, remove := range []bool{true, false} {
			_, e := f.svc.SetRoleMembers(ctx, &c.RoleMembersRequest{Context: f.ac, RoleID: f.viewerA.ID, UserIDs: []int64{f.shared.ID}, Remove: remove})
			must(e)
			code := int32(0)
			if remove {
				code = 403
			}
			check(f.sc, code)
			check(f.sharedB, 0)
		}
	})
	t.Run("entitlement-revoke-does-not-resurrect-role-grants", func(t *testing.T) {
		var ent catalog.Entitlement
		f.read(func(u ports.Unit) error {
			var e error
			ent, e = first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", f.ta.ID, "app_id", f.app.ID))
			return e
		})
		_, e := f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, RevokeIDs: []int64{ent.ID}})
		must(e)
		check(f.sc, 403)
		check(f.sharedB, 0)
		resources := []int64{}
		for _, r := range f.resources {
			resources = append(resources, r.ID)
		}
		_, e = f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID}, Apps: []*c.AppGrant{{AppID: f.app.ID, ResourceIDs: resources}}})
		must(e)
		check(f.sc, 403)
		auth, e := f.svc.Auth(ctx, f.sc)
		must(e)
		if len(auth.Permissions) != 0 {
			t.Fatal("restoring entitlement resurrected revoked role grants")
		}
	})
}

func TestMySQLApplicationDisableBlocksExistingSessionAndSwitcher(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var r catalog.Resource
	var op catalog.Operation
	f.write(func(u ports.Unit) error {
		f.extension.Managed = true
		f.extension.Home = "/notes/dashboard"
		if e := u.Save(ports.Apps, &f.extension); e != nil {
			return e
		}
		r = catalog.Resource{AppID: f.extension.ID, Code: "notes:read", Name: "Notes", Type: "view", Status: 1, MetaJSON: "{}"}
		if e := u.Save(ports.Resources, &r); e != nil {
			return e
		}
		op = catalog.Operation{AppID: f.extension.ID, ResourceID: r.ID, OperationID: "notes.read", Method: "GET", Path: "/api/apps/notes/v1/notes", Action: "notes.read"}
		return u.Save(ports.Operations, &op)
	})
	_, e := f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID}, Apps: []*c.AppGrant{{AppID: f.extension.ID, ResourceIDs: []int64{r.ID}}}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.SetRoleResources(ctx, &c.RoleResourcesRequest{Context: f.ac, RoleID: f.viewerA.ID, Grants: []*c.ResourceGrant{{AppID: f.app.ID, ResourceID: f.resources["membership.read"].ID, DataScope: 5}, {AppID: f.extension.ID, ResourceID: r.ID, DataScope: 5}}})
	if e != nil {
		t.Fatal(e)
	}
	selected := *f.sc
	selected.AppID = f.extension.ID
	request := &c.AccessRequest{Context: &selected, AppCode: "notes", Method: op.Method, Path: op.Path}
	if _, e = f.svc.CheckAccess(ctx, request); e != nil {
		t.Fatal(e)
	}
	for _, status := range []int32{0, 1} {
		if _, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: f.extension.ID, Status: status}); e != nil {
			t.Fatal(e)
		}
		_, e = f.svc.CheckAccess(ctx, request)
		if status == 0 {
			wantCode(t, e, 403)
		} else if e != nil {
			t.Fatal(e)
		}
		apps, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.sc, Kind: c.KindApps, AvailableOnly: true})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, app := range apps.Apps {
			found = found || app.ID == f.extension.ID
		}
		if found != (status == 1) {
			t.Fatal("application switcher ignored current application status")
		}
		if _, e = f.check(f.sc, "membership.read"); e != nil {
			t.Fatal("disabling extension blocked independent foundation access", e)
		}
	}
}
