package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	mysqlstore "kerthus/internal/saas/adapters/mysql"
	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/dictionary"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

// Integration tests create and drop only a fresh randomly named database.
// KERTHUS_TEST_MYSQL_DSN must authorize CREATE/DROP DATABASE; never a production DSN.
// The normal test suite skips these tests when the explicit test setting is absent.
type memorySessions struct {
	mu     sync.Mutex
	values map[string]identity.Session
}

func (m *memorySessions) Put(_ context.Context, k string, v identity.Session, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[k] = v
	return nil
}
func (m *memorySessions) Get(_ context.Context, k string) (identity.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[k]
	if !ok {
		return v, fault.Unauthorized
	}
	return v, nil
}
func (m *memorySessions) Delete(_ context.Context, k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, k)
	return nil
}
func (m *memorySessions) Ping(context.Context) error { return nil }

type fixture struct {
	t                                *testing.T
	db                               *mysqlstore.Store
	svc                              *Service
	platform, a, b, shared           identity.User
	platformTenant, ta, tb           tenant.Tenant
	app, extension                   catalog.App
	resources                        map[string]catalog.Resource
	ops                              map[string]catalog.Operation
	adminA, adminB, viewerA, viewerB access.Role
	orgA, orgB                       organization.Org
	positionA                        organization.Position
	pc, ac, bc, sc, sharedB          *c.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("KERTHUS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set KERTHUS_TEST_MYSQL_DSN to run isolated real-MySQL integration tests")
	}
	cfg, e := driver.ParseDSN(dsn)
	if e != nil {
		t.Fatal("invalid test DSN")
	}
	cfg.DBName = ""
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	suffix, e := randomToken()
	if e != nil {
		t.Fatal(e)
	}
	name := "kerthus_test_" + suffix[:16]
	if _, e = admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		admin.Close()
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if _, e := admin.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error(e)
		}
		admin.Close()
	})
	cfg.DBName = name
	cfg.ParseTime = true
	db, e := mysqlstore.Open(cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = db.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = db.Migrate(context.Background()); e != nil {
		t.Fatalf("idempotent migration: %v", e)
	}
	f := &fixture{t: t, db: db, resources: map[string]catalog.Resource{}, ops: map[string]catalog.Operation{}}
	f.svc = New(db, &memorySessions{values: map[string]identity.Session{}}, nil)
	hash, e := passwordHash("integration-password")
	if e != nil {
		t.Fatal(e)
	}
	f.write(func(u ports.Unit) error {
		for i, user := range []*identity.User{&f.platform, &f.a, &f.b, &f.shared} {
			*user = identity.User{Name: fmt.Sprintf("user-%d", i), Phone: fmt.Sprintf("1390000000%d", i), Status: 1, PasswordHash: hash, AuthVersion: 1, PlatformAdmin: i == 0}
			if e := u.Save(ports.Users, user); e != nil {
				return e
			}
		}
		for i, ten := range []*tenant.Tenant{&f.platformTenant, &f.ta, &f.tb} {
			*ten = tenant.Tenant{Name: fmt.Sprintf("tenant-%d", i), Status: 1, VerifyStatus: 1, BootstrapVersion: 1, AddressJSON: "[]"}
			if e := u.Save(ports.Tenants, ten); e != nil {
				return e
			}
		}
		f.app = catalog.App{Code: "basic", Name: "Foundation", Status: 1}
		if e := u.Save(ports.Apps, &f.app); e != nil {
			return e
		}
		f.extension = catalog.App{Code: "notes", Name: "Notes", Status: 1, ServiceKey: "kerthus.app.notes", RoutePrefix: "/api/apps/notes/v1"}
		if e := u.Save(ports.Apps, &f.extension); e != nil {
			return e
		}
		for _, action := range []string{"membership.read", "membership.write", "organization.read", "organization.write", "access.read", "access.write", "audit.read"} {
			r := catalog.Resource{AppID: f.app.ID, Code: "basic:" + action, Name: action, Type: "view", Status: 1, MetaJSON: "{}"}
			if e := u.Save(ports.Resources, &r); e != nil {
				return e
			}
			f.resources[action] = r
			op := catalog.Operation{AppID: f.app.ID, ResourceID: r.ID, OperationID: action, Method: "POST", Path: "/api/apps/basic/v1/" + action, Action: action}
			if e := u.Save(ports.Operations, &op); e != nil {
				return e
			}
			f.ops[action] = op
		}
		for _, ten := range []tenant.Tenant{f.ta, f.tb} {
			ids := []int64{}
			for _, r := range f.resources {
				ids = append(ids, r.ID)
			}
			if e := f.svc.grantApplication(u, ten.ID, &c.AppGrant{AppID: f.app.ID, ResourceIDs: ids}); e != nil {
				return e
			}
		}
		for _, pair := range []struct {
			ten  tenant.Tenant
			user identity.User
		}{{f.platformTenant, f.platform}, {f.ta, f.a}, {f.tb, f.b}, {f.ta, f.shared}, {f.tb, f.shared}} {
			m := tenant.Member{TenantID: pair.ten.ID, UserID: pair.user.ID, DefaultAppID: f.app.ID, Status: 1, IsDefault: pair.ten.ID != f.tb.ID}
			if e := u.Save(ports.Members, &m); e != nil {
				return e
			}
		}
		for _, pair := range []struct {
			role  *access.Role
			ten   tenant.Tenant
			user  identity.User
			admin bool
		}{{&f.adminA, f.ta, f.a, true}, {&f.adminB, f.tb, f.b, true}, {&f.viewerA, f.ta, f.shared, false}, {&f.viewerB, f.tb, f.shared, false}} {
			*pair.role = access.Role{TenantID: pair.ten.ID, Code: fmt.Sprintf("role-%d-%t", pair.ten.ID, pair.admin), Name: "Role", Status: 1, Administrator: pair.admin}
			if e := u.Save(ports.Roles, pair.role); e != nil {
				return e
			}
			if e := u.Save(ports.RoleMembers, &access.RoleMember{TenantID: pair.ten.ID, RoleID: pair.role.ID, UserID: pair.user.ID}); e != nil {
				return e
			}
			for action, r := range f.resources {
				if !pair.admin && action != "membership.read" {
					continue
				}
				scope := int32(5)
				if pair.admin {
					scope = 0
				}
				if e := u.Save(ports.Grants, &access.Grant{TenantID: pair.ten.ID, RoleID: pair.role.ID, AppID: f.app.ID, ResourceID: r.ID, DataScope: scope}); e != nil {
					return e
				}
			}
		}
		for _, pair := range []struct {
			org  *organization.Org
			ten  tenant.Tenant
			user identity.User
		}{{&f.orgA, f.ta, f.a}, {&f.orgB, f.tb, f.b}} {
			*pair.org = organization.Org{TenantID: pair.ten.ID, Name: "Root", Type: "unit", Status: 1}
			if e := u.Save(ports.Orgs, pair.org); e != nil {
				return e
			}
			pair.org.UnitID = pair.org.ID
			if e := u.Save(ports.Orgs, pair.org); e != nil {
				return e
			}
			if e := u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: pair.ten.ID, UserID: pair.user.ID, OrgID: pair.org.ID}); e != nil {
				return e
			}
		}
		f.positionA = organization.Position{TenantID: f.ta.ID, OrgID: f.orgA.ID, Name: "Administrator", Status: 1}
		if e := u.Save(ports.Positions, &f.positionA); e != nil {
			return e
		}
		return u.Save(ports.MemberPositions, &organization.MemberPosition{TenantID: f.ta.ID, UserID: f.a.ID, PositionID: f.positionA.ID})
	})
	f.pc = f.session(f.platform, f.platformTenant)
	f.ac = f.session(f.a, f.ta)
	f.bc = f.session(f.b, f.tb)
	f.sc = f.session(f.shared, f.ta)
	f.sharedB = f.session(f.shared, f.tb)
	return f
}
func (f *fixture) write(fn func(ports.Unit) error) {
	f.t.Helper()
	if e := f.db.Write(context.Background(), fn); e != nil {
		f.t.Fatal(e)
	}
}
func (f *fixture) read(fn func(ports.Unit) error) {
	f.t.Helper()
	if e := f.db.Read(context.Background(), fn); e != nil {
		f.t.Fatal(e)
	}
}
func (f *fixture) session(user identity.User, ten tenant.Tenant) *c.Context {
	token, e := randomToken()
	if e != nil {
		f.t.Fatal(e)
	}
	f.svc.Sessions.Put(context.Background(), token, identity.Session{UserID: user.ID, AuthVersion: user.AuthVersion, ExpiresAt: time.Now().Add(time.Hour).Unix()}, time.Hour)
	return &c.Context{Token: token, TenantID: ten.ID, AppID: f.app.ID}
}
func (f *fixture) count(table ports.Table, filter ports.Filter) int64 {
	var n int64
	f.read(func(u ports.Unit) error { var e error; n, e = u.Count(table, filter); return e })
	return n
}
func wantCode(t *testing.T, e error, code int32) {
	t.Helper()
	if e == nil {
		t.Fatalf("wanted error %d, got success", code)
	}
	got, _ := fault.Code(e)
	if got != code {
		t.Fatalf("wanted error %d, got %d: %v", code, got, e)
	}
}
func (f *fixture) check(req *c.Context, action string) (*c.AccessReply, error) {
	return f.svc.CheckAccess(context.Background(), &c.AccessRequest{Context: req, AppCode: "basic", Method: "POST", Path: f.ops[action].Path})
}

func TestMySQLTenantIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindMembers, TargetTenantID: f.tb.ID})
	wantCode(t, e, 403)
	forged := *f.ac
	forged.TenantID = f.tb.ID
	_, e = f.svc.Auth(ctx, &forged)
	wantCode(t, e, 403)
	forged = *f.ac
	forged.UnitID = f.orgB.ID
	_, e = f.svc.Auth(ctx, &forged)
	wantCode(t, e, 403)
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Org: &c.Org{ID: f.orgB.ID, TenantID: f.ta.ID, Name: "stolen", Type: "unit"}})
	wantCode(t, e, 403)
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Role: &c.Role{ID: f.adminB.ID, TenantID: f.ta.ID, Name: "stolen"}})
	wantCode(t, e, 403)
	before := f.count(ports.Users, ports.Filter{})
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Member: &c.Member{TenantID: f.ta.ID, User: &c.User{Name: "new", Phone: "13999999999"}, OrgIDs: []int64{f.orgB.ID}}, Password: "integration-password"})
	wantCode(t, e, 400)
	if got := f.count(ports.Users, ports.Filter{}); got != before {
		t.Fatal("cross-tenant member write created an identity")
	}
	f.read(func(u ports.Unit) error {
		var org organization.Org
		if e := u.Get(ports.Orgs, f.orgB.ID, &org); e != nil {
			return e
		}
		if org.Name != "Root" {
			t.Fatal("foreign organization changed")
		}
		return nil
	})
}
func TestMySQLIdentityAndMembershipAreSeparate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, e := f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Member: &c.Member{TenantID: f.ta.ID, User: &c.User{ID: f.shared.ID, Phone: f.shared.Phone, Name: "tenant-local overwrite"}}})
	wantCode(t, e, 400)
	_, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.ac, Kind: c.KindUsers, ID: f.shared.ID, Status: 0})
	wantCode(t, e, 403)
	_, e = f.svc.ChangePassword(ctx, &c.PasswordRequest{Context: f.ac, UserID: f.shared.ID, NewPassword: "replacement-password"})
	wantCode(t, e, 403)
	_, e = f.svc.Delete(ctx, &c.DeleteRequest{Context: f.ac, Kind: c.KindMembers, ID: f.shared.ID})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.Auth(ctx, f.sc)
	wantCode(t, e, 403)
	if _, e = f.svc.Auth(ctx, f.sharedB); e != nil {
		t.Fatalf("other tenant membership was affected: %v", e)
	}
	f.read(func(u ports.Unit) error {
		var user identity.User
		if e := u.Get(ports.Users, f.shared.ID, &user); e != nil {
			return e
		}
		if user.Status != 1 || user.AuthVersion != 1 || user.Name != f.shared.Name {
			t.Fatal("member deletion modified shared identity")
		}
		return nil
	})
	login, e := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.shared.Phone, Password: "integration-password"})
	if e != nil {
		t.Fatal(e)
	}
	if login.TenantID != f.tb.ID {
		t.Fatal("login selected disabled default membership")
	}
}
func TestMySQLLastAdministratorAndConcurrentRemoval(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, e := f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindUsers, ID: f.platform.ID, Status: 0})
	wantCode(t, e, 409)
	_, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.ac, Kind: c.KindMembers, ID: f.a.ID, Status: 0})
	wantCode(t, e, 409)
	_, e = f.svc.Delete(ctx, &c.DeleteRequest{Context: f.ac, Kind: c.KindRoles, ID: f.adminA.ID})
	wantCode(t, e, 409)
	_, e = f.svc.SetRoleMembers(ctx, &c.RoleMembersRequest{Context: f.ac, RoleID: f.adminA.ID, UserIDs: []int64{f.a.ID}, Remove: true})
	wantCode(t, e, 409)
	_, e = f.svc.SetRoleMembers(ctx, &c.RoleMembersRequest{Context: f.ac, RoleID: f.adminA.ID, UserIDs: []int64{f.shared.ID}})
	if e != nil {
		t.Fatal(e)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []int64{f.a.ID, f.shared.ID} {
		go func(id int64) {
			<-start
			_, e := f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindMembers, TargetTenantID: f.ta.ID, ID: id, Status: 0})
			results <- e
		}(id)
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		e := <-results
		if e == nil {
			successes++
		} else {
			code, _ := fault.Code(e)
			if code == 409 {
				conflicts++
			} else {
				t.Fatal(e)
			}
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("last admin race: success=%d conflict=%d", successes, conflicts)
	}
}
func TestMySQLRoleEntitlementIntersectionAndImmediateRevocation(t *testing.T) {
	f := newFixture(t)
	read := f.resources["membership.read"]
	reply, e := f.check(f.sc, "membership.read")
	if e != nil {
		t.Fatal(e)
	}
	if reply.AllWithinTenant || len(reply.UserIDs) != 1 || reply.UserIDs[0] != f.shared.ID {
		t.Fatalf("unexpected self scope: %+v", reply)
	}
	_, e = f.check(f.sc, "membership.write")
	wantCode(t, e, 403)
	f.write(func(u ports.Unit) error {
		return u.Delete(ports.TenantResources, filter("tenant_id", f.ta.ID, "resource_id", read.ID))
	})
	_, e = f.check(f.sc, "membership.read")
	wantCode(t, e, 403)
	if _, e = f.check(f.sharedB, "membership.read"); e != nil {
		t.Fatal(e)
	}
	f.write(func(u ports.Unit) error {
		return u.Save(ports.TenantResources, &catalog.TenantResource{TenantID: f.ta.ID, AppID: f.app.ID, ResourceID: read.ID})
	})
	f.write(func(u ports.Unit) error { f.viewerA.Status = 0; return u.Save(ports.Roles, &f.viewerA) })
	_, e = f.check(f.sc, "membership.read")
	wantCode(t, e, 403)
	f.write(func(u ports.Unit) error { f.viewerA.Status = 1; return u.Save(ports.Roles, &f.viewerA) })
	f.write(func(u ports.Unit) error { r := read; r.Status = 0; return u.Save(ports.Resources, &r) })
	_, e = f.check(f.sc, "membership.read")
	wantCode(t, e, 403)
}
func TestMySQLBulkMutationRollback(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := f.resources["membership.read"]
	before := f.count(ports.Audits, ports.Filter{})
	grant := &c.ResourceGrant{AppID: f.app.ID, ResourceID: r.ID, DataScope: 0}
	_, e := f.svc.SetRoleResources(ctx, &c.RoleResourcesRequest{Context: f.ac, RoleID: f.viewerA.ID, Grants: []*c.ResourceGrant{grant, grant}})
	wantCode(t, e, 400)
	got, e := f.check(f.sc, "membership.read")
	if e != nil {
		t.Fatal(e)
	}
	if got.AllWithinTenant {
		t.Fatal("failed replace changed original self scope")
	}
	if f.count(ports.Audits, ports.Filter{}) != before {
		t.Fatal("rollback persisted audit")
	}
	_, e = f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID, 99999999}, Apps: []*c.AppGrant{{AppID: f.app.ID, ResourceIDs: []int64{r.ID}, ExpiresAt: time.Now().Add(time.Hour).Unix()}}})
	wantCode(t, e, 404)
	if f.count(ports.TenantResources, eq("tenant_id", f.ta.ID)) != int64(len(f.resources)) {
		t.Fatal("partial bulk entitlement replacement committed")
	}
	if _, e = f.check(f.ac, "access.write"); e != nil {
		t.Fatal("bulk rollback removed administrator grant", e)
	}
}
func TestMySQLExpiryAndPasswordRevocation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.write(func(u ports.Unit) error {
		ent, e := first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", f.ta.ID, "app_id", f.app.ID))
		if e != nil {
			return e
		}
		ent.ExpiresAt = time.Now().Unix() - 1
		return u.Save(ports.Entitlements, &ent)
	})
	_, e := f.svc.Auth(ctx, f.sc)
	wantCode(t, e, 403)
	f.write(func(u ports.Unit) error {
		ent, e := first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", f.ta.ID, "app_id", f.app.ID))
		if e != nil {
			return e
		}
		ent.ExpiresAt = 0
		return u.Save(ports.Entitlements, &ent)
	})
	f.write(func(u ports.Unit) error { f.ta.ExpiresAt = time.Now().Unix() - 1; return u.Save(ports.Tenants, &f.ta) })
	_, e = f.svc.Auth(ctx, f.sc)
	wantCode(t, e, 403)
	if _, e = f.svc.Auth(ctx, f.sharedB); e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.ChangePassword(ctx, &c.PasswordRequest{Context: f.sharedB, OldPassword: "integration-password", NewPassword: "replacement-password"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.Auth(ctx, f.sharedB)
	wantCode(t, e, 401)
	_, e = f.svc.Profile(ctx, f.sc)
	wantCode(t, e, 401)
}
func TestMySQLResourceOperationReplacement(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := f.resources["membership.read"]
	_, e := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: &c.Resource{ID: r.ID, AppID: r.AppID, Name: r.Name, Code: r.Code, Type: r.Type, MetaJSON: r.MetaJSON}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.check(f.sc, "membership.read")
	wantCode(t, e, 403)
	f.read(func(u ports.Unit) error {
		var op catalog.Operation
		if e := u.Get(ports.Operations, f.ops["membership.read"].ID, &op); e != nil {
			return e
		}
		if op.ResourceID != 0 {
			t.Fatal("omitted operation binding survived replace")
		}
		return nil
	})
	op := f.ops["membership.read"]
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: &c.Resource{ID: r.ID, AppID: r.AppID, Name: r.Name, Code: r.Code, Type: r.Type, Operations: []*c.Operation{{Method: op.Method, Path: op.Path}}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.check(f.sc, "membership.read"); e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: &c.Resource{ID: r.ID, AppID: r.AppID, Name: "should rollback", Code: r.Code, Type: r.Type, Operations: []*c.Operation{{Method: "POST", Path: "/unregistered"}}}})
	wantCode(t, e, 400)
	f.read(func(u ports.Unit) error {
		var saved catalog.Resource
		if e := u.Get(ports.Resources, r.ID, &saved); e != nil {
			return e
		}
		if saved.Name != r.Name {
			t.Fatal("invalid operation persisted resource mutation")
		}
		return nil
	})
}
func TestMySQLOrganizationReferences(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, e := f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Org: &c.Org{ID: f.orgA.ID, Name: "Root", Type: "unit", Status: ptr(0)}})
	wantCode(t, e, 409)
	_, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.ac, Kind: c.KindPositions, ID: f.positionA.ID, Status: 0})
	wantCode(t, e, 409)
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Position: &c.Position{ID: f.positionA.ID, OrgID: f.orgA.ID, Name: "Admin", Status: ptr(0)}})
	wantCode(t, e, 409)
	_, e = f.svc.Delete(ctx, &c.DeleteRequest{Context: f.ac, Kind: c.KindOrgs, ID: f.orgA.ID})
	wantCode(t, e, 409)
}
func TestMySQLApproveIsAtomicAndIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pending := tenant.Tenant{Name: "New Tenant", ContactPhone: "13711112222", Status: 1, AddressJSON: "[]"}
	f.write(func(u ports.Unit) error { return u.Save(ports.Tenants, &pending) })
	usersBefore := f.count(ports.Users, ports.Filter{})
	f.write(func(u ports.Unit) error { f.app.Status = 0; return u.Save(ports.Apps, &f.app) })
	req := &c.ApproveRequest{Context: f.pc, TenantID: pending.ID, Approved: true, AdminPassword: "integration-password"}
	_, e := f.svc.ApproveTenant(ctx, req)
	wantCode(t, e, 400)
	if f.count(ports.Users, ports.Filter{}) != usersBefore || f.count(ports.Orgs, eq("tenant_id", pending.ID)) != 0 || f.count(ports.Members, eq("tenant_id", pending.ID)) != 0 {
		t.Fatal("failed approval partially committed")
	}
	f.write(func(u ports.Unit) error { f.app.Status = 1; return u.Save(ports.Apps, &f.app) })
	if _, e = f.svc.ApproveTenant(ctx, req); e != nil {
		t.Fatal(e)
	}
	counts := map[ports.Table]int64{}
	for _, table := range []ports.Table{ports.Orgs, ports.Positions, ports.Roles, ports.RoleMembers, ports.Members, ports.TenantResources, ports.Grants} {
		counts[table] = f.count(table, eq("tenant_id", pending.ID))
	}
	if _, e = f.svc.ApproveTenant(ctx, req); e != nil {
		t.Fatal(e)
	}
	for table, n := range counts {
		if got := f.count(table, eq("tenant_id", pending.ID)); got != n {
			t.Fatalf("repeat approval duplicated %s", table)
		}
	}
	if got := f.count(ports.Users, ports.Filter{}); got != usersBefore+1 {
		t.Fatal("approval duplicated identity")
	}
}
func TestRouteMatching(t *testing.T) {
	for _, path := range []string{"/api/notes/", "/api/notes/..", "/api/notes/x/y"} {
		if RouteMatches("/api/notes/{id}", path) {
			t.Errorf("invalid path matched: %q", path)
		}
	}
	if !RouteMatches("/api/notes/{id}", "/api/notes/123") {
		t.Fatal("parameter route failed")
	}
	if RouteMatches("/api/notes/{id}", strings.Repeat("/", 5)) {
		t.Fatal("empty segments matched")
	}
}

func TestMySQLEntitlementRevokeUsesEntitlementID(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var ent catalog.Entitlement
	f.read(func(u ports.Unit) error {
		var e error
		ent, e = first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", f.ta.ID, "app_id", f.app.ID))
		return e
	})
	if ent.ID == f.ta.ID {
		t.Fatal("fixture must distinguish entitlement ID from tenant ID")
	}
	_, e := f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, RevokeIDs: []int64{ent.ID}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.Auth(ctx, f.sc)
	wantCode(t, e, 403)
	if _, e = f.svc.Auth(ctx, f.sharedB); e != nil {
		t.Fatal("revocation affected unrelated tenant", e)
	}
	if n := f.count(ports.Grants, eq("tenant_id", f.ta.ID)); n != 0 {
		t.Fatal("revoked grants remain")
	}
	f.read(func(u ports.Unit) error {
		m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", f.ta.ID, "user_id", f.shared.ID))
		if e != nil {
			return e
		}
		if m.DefaultAppID != 0 {
			t.Fatal("revoked default app remains")
		}
		return nil
	})
}
func TestMySQLScopeUnionAndApplicationIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var second access.Role
	var notesResource catalog.Resource
	f.write(func(u ports.Unit) error {
		second = access.Role{TenantID: f.ta.ID, Code: "second", Name: "second", Status: 1}
		if e := u.Save(ports.Roles, &second); e != nil {
			return e
		}
		if e := u.Save(ports.RoleMembers, &access.RoleMember{TenantID: f.ta.ID, RoleID: second.ID, UserID: f.shared.ID}); e != nil {
			return e
		}
		if e := u.Save(ports.Grants, &access.Grant{TenantID: f.ta.ID, RoleID: second.ID, AppID: f.app.ID, ResourceID: f.resources["membership.read"].ID, DataScope: 0}); e != nil {
			return e
		}
		notesResource = catalog.Resource{AppID: f.extension.ID, Code: "notes:read", Name: "Notes", Type: "action", Status: 1, MetaJSON: "{}"}
		return u.Save(ports.Resources, &notesResource)
	})
	reply, e := f.check(f.sc, "membership.read")
	if e != nil {
		t.Fatal(e)
	}
	if !reply.AllWithinTenant || reply.TenantID != f.ta.ID {
		t.Fatal("all scope must union within current tenant only")
	}
	_, e = f.svc.SetRoleResources(ctx, &c.RoleResourcesRequest{Context: f.ac, RoleID: f.viewerA.ID, Grants: []*c.ResourceGrant{{AppID: f.extension.ID, ResourceID: notesResource.ID}}})
	wantCode(t, e, 400)
	_, e = f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID}, Apps: []*c.AppGrant{{AppID: f.app.ID, ResourceIDs: []int64{notesResource.ID}}}})
	wantCode(t, e, 400)
	forged := *f.sc
	forged.AppID = f.extension.ID
	_, e = f.svc.Auth(ctx, &forged)
	wantCode(t, e, 403)
	_, e = f.svc.CheckAccess(ctx, &c.AccessRequest{Context: f.sc, AppCode: "notes", Method: "POST", Path: f.ops["membership.read"].Path})
	wantCode(t, e, 403)
}
func TestMySQLQuerySearchAndSort(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	reply, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindOperations, Search: "membership.read"})
	if e != nil {
		t.Fatal(e)
	}
	if len(reply.Operations) != 1 {
		t.Fatal("operation directory search failed")
	}
	reply, e = f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, Search: "membership.read", Sort: "created_at", Desc: true})
	if e != nil {
		t.Fatal(e)
	}
	if len(reply.Resources) != 1 {
		t.Fatal("resource search failed")
	}
	_, e = f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, Sort: "id DESC; DROP TABLE users"})
	wantCode(t, e, 400)
	if f.count(ports.Users, ports.Filter{}) != 4 {
		t.Fatal("unsafe sort affected records")
	}
}

func TestMySQLApplicationRegistrationUpgrade(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := Definition{App: catalog.App{Code: "tasks", Name: "Tasks", Version: "1.0.0", ResourceVersion: 1, ManifestHash: "version-one", Home: "/tasks/dashboard", FrontendEntry: "bundled", ServiceKey: "kerthus.app.tasks", RoutePrefix: "/api/apps/tasks/v1"}, Resources: []catalog.Resource{{Code: "tasks:read", Name: "Read tasks", Type: "action", MetaJSON: "{}"}}, Operations: []catalog.Operation{{OperationID: "listTasks", Method: "GET", Path: "/api/apps/tasks/v1/tasks", Action: "tasks.read"}}, OperationResources: map[string]string{"listTasks": "tasks:read"}}
	if e := f.svc.RegisterApplication(ctx, d); e != nil {
		t.Fatal(e)
	}
	if e := f.svc.RegisterApplication(ctx, d); e != nil {
		t.Fatal("idempotent registration", e)
	}
	var app catalog.App
	var read catalog.Resource
	f.read(func(u ports.Unit) error {
		var e error
		app, e = first[catalog.App](u, ports.Apps, eq("code", "tasks"))
		if e != nil {
			return e
		}
		read, e = first[catalog.Resource](u, ports.Resources, filter("app_id", app.ID, "code", "tasks:read"))
		return e
	})
	_, e := f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID}, Apps: []*c.AppGrant{{AppID: app.ID, ResourceIDs: []int64{read.ID}}}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.SetRoleResources(ctx, &c.RoleResourcesRequest{Context: f.ac, RoleID: f.viewerA.ID, Grants: []*c.ResourceGrant{{AppID: app.ID, ResourceID: read.ID, DataScope: 5}}})
	if e != nil {
		t.Fatal(e)
	}
	ac := *f.sc
	ac.AppID = app.ID
	bc := *f.sharedB
	bc.AppID = app.ID
	check := func(actor *c.Context, path string) (*c.AccessReply, error) {
		return f.svc.CheckAccess(ctx, &c.AccessRequest{Context: actor, AppCode: "tasks", Method: "GET", Path: path})
	}
	got, e := check(&ac, "/api/apps/tasks/v1/tasks")
	if e != nil {
		t.Fatal(e)
	}
	if got.ServiceKey != "kerthus.app.tasks" || got.TenantID != f.ta.ID {
		t.Fatal("application routing escaped tenant")
	}
	_, e = check(&bc, "/api/apps/tasks/v1/tasks")
	wantCode(t, e, 403)
	d.App.ManifestHash = "changed-without-version"
	e = f.svc.RegisterApplication(ctx, d)
	wantCode(t, e, 409)
	d.App.ResourceVersion = 2
	d.App.ManifestHash = "version-two"
	d.Resources = append(d.Resources, catalog.Resource{Code: "tasks:export", Name: "Export tasks", Type: "action", MetaJSON: "{}"})
	d.Operations = append(d.Operations, catalog.Operation{OperationID: "exportTasks", Method: "GET", Path: "/api/apps/tasks/v1/export", Action: "tasks.export"})
	d.OperationResources["exportTasks"] = "tasks:export"
	if e = f.svc.RegisterApplication(ctx, d); e != nil {
		t.Fatal(e)
	}
	if _, e = check(&ac, "/api/apps/tasks/v1/tasks"); e != nil {
		t.Fatal("upgrade invalidated existing resource grant", e)
	}
	_, e = check(&ac, "/api/apps/tasks/v1/export")
	wantCode(t, e, 403)
	if n := f.count(ports.TenantResources, filter("tenant_id", f.ta.ID, "app_id", app.ID)); n != 1 {
		t.Fatal("new application resource was automatically entitled")
	}
	d.App.ResourceVersion = 3
	d.App.ManifestHash = "version-three"
	d.OperationResources["exportTasks"] = "missing-resource"
	e = f.svc.RegisterApplication(ctx, d)
	wantCode(t, e, 400)
	f.read(func(u ports.Unit) error {
		var got catalog.App
		if e := u.Get(ports.Apps, app.ID, &got); e != nil {
			return e
		}
		if got.ResourceVersion != 2 {
			t.Fatal("invalid upgrade was partially committed")
		}
		return nil
	})
	if _, e = check(&ac, "/api/apps/tasks/v1/tasks"); e != nil {
		t.Fatal("failed upgrade lost route registration", e)
	}
	if _, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: app.ID, Status: 0}); e != nil {
		t.Fatal(e)
	}
	d.OperationResources["exportTasks"] = "tasks:export"
	if e = f.svc.RegisterApplication(ctx, d); e != nil {
		t.Fatal(e)
	}
	_, e = check(&ac, "/api/apps/tasks/v1/tasks")
	wantCode(t, e, 403)
	f.read(func(u ports.Unit) error {
		var got catalog.App
		if e := u.Get(ports.Apps, app.ID, &got); e != nil {
			return e
		}
		if got.Status != 0 || got.ResourceVersion != 3 {
			t.Fatal("application upgrade restored disabled status")
		}
		return nil
	})
	if _, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: app.ID, Status: 1}); e != nil {
		t.Fatal(e)
	}
	if _, e = check(&ac, "/api/apps/tasks/v1/tasks"); e != nil {
		t.Fatal("explicit app reactivation failed", e)
	}
}

type noSQL struct{}

func (noSQL) Read(_ context.Context, fn func(ports.Unit) error) error  { return fn(nil) }
func (noSQL) Write(_ context.Context, fn func(ports.Unit) error) error { return fn(nil) }
func (noSQL) Ping(context.Context) error                               { return nil }
func TestMissingRPCRequestsAndContexts(t *testing.T) {
	s := New(noSQL{}, &memorySessions{values: map[string]identity.Session{}}, nil)
	ctx := context.Background()
	nilCalls := map[string]func() error{
		"login": func() error { _, e := s.Login(ctx, nil); return e }, "save": func() error { _, e := s.Save(ctx, nil); return e }, "query": func() error { _, e := s.Query(ctx, nil); return e }, "approve": func() error { _, e := s.ApproveTenant(ctx, nil); return e }, "status": func() error { _, e := s.SetStatus(ctx, nil); return e }, "delete": func() error { _, e := s.Delete(ctx, nil); return e }, "entitlements": func() error { _, e := s.SetEntitlements(ctx, nil); return e }, "role-resources": func() error { _, e := s.SetRoleResources(ctx, nil); return e }, "role-members": func() error { _, e := s.SetRoleMembers(ctx, nil); return e }, "access": func() error { _, e := s.CheckAccess(ctx, nil); return e }, "password": func() error { _, e := s.ChangePassword(ctx, nil); return e }, "upload": func() error { _, e := s.CreateUpload(ctx, nil); return e }, "confirm-upload": func() error { _, e := s.ConfirmUpload(ctx, nil); return e },
	}
	for name, call := range nilCalls {
		t.Run("nil-request/"+name, func(t *testing.T) { wantCode(t, call(), 400) })
	}
	contextCalls := map[string]func() error{
		"profile": func() error { _, e := s.Profile(ctx, nil); return e }, "auth": func() error { _, e := s.Auth(ctx, nil); return e }, "logout": func() error { _, e := s.Logout(ctx, nil); return e }, "save": func() error { _, e := s.Save(ctx, &c.SaveRequest{Member: &c.Member{}}); return e }, "query": func() error { _, e := s.Query(ctx, &c.QueryRequest{}); return e }, "approve": func() error { _, e := s.ApproveTenant(ctx, &c.ApproveRequest{}); return e }, "status": func() error { _, e := s.SetStatus(ctx, &c.StatusRequest{}); return e }, "delete": func() error { _, e := s.Delete(ctx, &c.DeleteRequest{}); return e }, "entitlements": func() error { _, e := s.SetEntitlements(ctx, &c.EntitlementsRequest{}); return e }, "role-resources": func() error { _, e := s.SetRoleResources(ctx, &c.RoleResourcesRequest{}); return e }, "role-members": func() error { _, e := s.SetRoleMembers(ctx, &c.RoleMembersRequest{}); return e }, "access": func() error { _, e := s.CheckAccess(ctx, &c.AccessRequest{}); return e }, "password": func() error {
			_, e := s.ChangePassword(ctx, &c.PasswordRequest{NewPassword: "replacement-password"})
			return e
		}, "upload": func() error {
			_, e := s.CreateUpload(ctx, &c.UploadRequest{Size: 1, ContentType: "image/png"})
			return e
		}, "confirm-upload": func() error { _, e := s.ConfirmUpload(ctx, &c.ConfirmUploadRequest{}); return e },
	}
	for name, call := range contextCalls {
		t.Run("nil-context/"+name, func(t *testing.T) { wantCode(t, call(), 401) })
	}
}
func TestMySQLDefaultOrganizationIsRevalidated(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var extra organization.Org
	f.write(func(u ports.Unit) error {
		m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", f.ta.ID, "user_id", f.a.ID))
		if e != nil {
			return e
		}
		m.DefaultOrgID = f.orgB.ID
		m.DefaultUnitID = f.orgB.ID
		if e = u.Save(ports.Members, &m); e != nil {
			return e
		}
		extra = organization.Org{TenantID: f.ta.ID, ParentID: f.orgA.ID, UnitID: f.orgA.ID, Name: "Second department", Type: "section", Status: 1}
		if e = u.Save(ports.Orgs, &extra); e != nil {
			return e
		}
		return u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: f.ta.ID, UserID: f.a.ID, OrgID: extra.ID})
	})
	reply, e := f.svc.Auth(ctx, f.ac)
	if e != nil {
		t.Fatal(e)
	}
	if reply.Context.SectionID != f.orgA.ID || reply.Context.UnitID != f.orgA.ID {
		t.Fatal("stale foreign default organization leaked into session context")
	}
	chosen := *f.ac
	chosen.SectionID = extra.ID
	chosen.UnitID = f.orgA.ID
	reply, e = f.svc.Auth(ctx, &chosen)
	if e != nil {
		t.Fatal(e)
	}
	if reply.Context.SectionID != extra.ID {
		t.Fatal("explicit valid organization selection was ignored")
	}
	login, e := f.svc.Login(ctx, &c.LoginRequest{Username: f.a.Phone, Password: "integration-password", Scene: "phone"})
	if e != nil {
		t.Fatal(e)
	}
	if login.SectionID != f.orgA.ID {
		t.Fatal("login used stale organization default")
	}
	invalid := *f.ac
	invalid.TenantID = -1
	_, e = f.svc.Auth(ctx, &invalid)
	wantCode(t, e, 400)
}
func TestMySQLAuthorizationDetailQueries(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	reply, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindMembers, RoleID: f.adminA.ID, ForAuthorization: true, PageSize: 1})
	if e != nil {
		t.Fatal(e)
	}
	if reply.Total != 1 || len(reply.Members) != 1 || reply.Members[0].User.ID != f.a.ID || !reply.Members[0].HasAdd {
		t.Fatal("role member detail includes candidates or has wrong count")
	}
	reply, e = f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindMembers, RoleID: f.adminA.ID})
	if e != nil {
		t.Fatal(e)
	}
	if reply.Total != 2 {
		t.Fatal("role candidate query unexpectedly filtered")
	}
	reply, e = f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindMembers, RoleID: f.adminA.ID, ForAuthorization: true, Search: f.shared.Name})
	if e != nil {
		t.Fatal(e)
	}
	if reply.Total != 0 {
		t.Fatal("search did not intersect role membership")
	}
	f.write(func(u ports.Unit) error {
		return u.Delete(ports.TenantResources, filter("tenant_id", f.ta.ID, "resource_id", f.resources["audit.read"].ID))
	})
	reply, e = f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, ForAuthorization: true, TargetTenantID: f.ta.ID})
	if e != nil {
		t.Fatal(e)
	}
	if reply.Total != int64(len(f.resources)-1) {
		t.Fatal("platform target tenant resource picker exceeds entitlement")
	}
	reply, e = f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, ForAuthorization: true})
	if e != nil {
		t.Fatal(e)
	}
	if reply.Total != int64(len(f.resources)) {
		t.Fatal("platform global resource picker was narrowed")
	}
}
func TestMySQLAuthorizationDoesNotTruncateAtQueryPageSize(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var last catalog.Resource
	f.write(func(u ports.Unit) error {
		for i := 0; i < 1001; i++ {
			r := catalog.Resource{AppID: f.app.ID, Code: fmt.Sprintf("basic:bulk:%d", i), Name: "Bulk permission", Type: "action", Status: 1, MetaJSON: "{}"}
			if e := u.Save(ports.Resources, &r); e != nil {
				return e
			}
			if e := u.Save(ports.TenantResources, &catalog.TenantResource{TenantID: f.ta.ID, AppID: f.app.ID, ResourceID: r.ID}); e != nil {
				return e
			}
			if e := u.Save(ports.Grants, &access.Grant{TenantID: f.ta.ID, RoleID: f.viewerA.ID, AppID: f.app.ID, ResourceID: r.ID, DataScope: 5}); e != nil {
				return e
			}
			last = r
		}
		return u.Save(ports.Operations, &catalog.Operation{AppID: f.app.ID, ResourceID: last.ID, OperationID: "lastBulkPermission", Method: "GET", Path: "/api/apps/basic/v1/last", Action: "bulk.read"})
	})
	reply, e := f.svc.Auth(ctx, f.sc)
	if e != nil {
		t.Fatal(e)
	}
	if len(reply.Permissions) != 1002 {
		t.Fatalf("authorization silently truncated: %d", len(reply.Permissions))
	}
	_, e = f.svc.CheckAccess(ctx, &c.AccessRequest{Context: f.sc, AppCode: "basic", Method: "GET", Path: "/api/apps/basic/v1/last"})
	if e != nil {
		t.Fatal("last permission omitted by pagination", e)
	}
	q, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, PageSize: 1000})
	if e != nil {
		t.Fatal(e)
	}
	if q.Total != 1008 || len(q.Resources) != 1000 {
		t.Fatal("resource page must report total separately")
	}
}

func TestMySQLDistrictChildren(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.write(func(u ports.Unit) error {
		if e := u.Insert(ports.Districts, &dictionary.District{ID: 101, Name: "Test parent"}); e != nil {
			return e
		}
		return u.Insert(ports.Districts, &dictionary.District{ID: 102, ParentID: 101, Name: "Test leaf"})
	})
	roots, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindDistricts})
	if e != nil {
		t.Fatal(e)
	}
	if len(roots.Districts) != 1 || !roots.Districts[0].HasChildren {
		t.Fatal("district parent has no children marker")
	}
	leaves, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindDistricts, ParentID: 101})
	if e != nil {
		t.Fatal(e)
	}
	if len(leaves.Districts) != 1 || leaves.Districts[0].HasChildren {
		t.Fatal("district leaf marker incorrect")
	}
}

func TestMySQLCrossAppPersistedBindingCannotBeGranted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var resource catalog.Resource
	f.write(func(u ports.Unit) error {
		resource = catalog.Resource{AppID: f.extension.ID, Code: "notes:read", Name: "Notes", Type: "action", Status: 1, MetaJSON: "{}"}
		if e := u.Save(ports.Resources, &resource); e != nil {
			return e
		}
		// Simulate inconsistent legacy/import data; public grantApplication rejects it.
		return u.Save(ports.TenantResources, &catalog.TenantResource{TenantID: f.ta.ID, AppID: f.app.ID, ResourceID: resource.ID})
	})
	_, e := f.svc.SetRoleResources(ctx, &c.RoleResourcesRequest{Context: f.ac, RoleID: f.viewerA.ID, Grants: []*c.ResourceGrant{{AppID: f.app.ID, ResourceID: resource.ID}}})
	wantCode(t, e, 400)
	if _, e = f.check(f.sc, "membership.read"); e != nil {
		t.Fatal("rejected grant discarded existing role permissions", e)
	}
}
func TestMySQLTenantDeletionKeepsReferencedData(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pending := tenant.Tenant{Name: "Pending", Status: 1, AddressJSON: "[]"}
	f.write(func(u ports.Unit) error { return u.Save(ports.Tenants, &pending) })
	f.write(func(u ports.Unit) error { return f.svc.grantApplication(u, pending.ID, &c.AppGrant{AppID: f.app.ID}) })
	_, e := f.svc.Delete(ctx, &c.DeleteRequest{Context: f.pc, Kind: c.KindTenants, ID: pending.ID})
	wantCode(t, e, 409)
	if f.count(ports.Entitlements, eq("tenant_id", pending.ID)) != 1 {
		t.Fatal("rejected tenant deletion removed entitlement")
	}
	_, e = f.svc.Delete(ctx, &c.DeleteRequest{Context: f.pc, Kind: c.KindTenants, ID: f.ta.ID})
	wantCode(t, e, 409)
	empty := tenant.Tenant{Name: "Empty pending", Status: 1, AddressJSON: "[]"}
	f.write(func(u ports.Unit) error { return u.Save(ports.Tenants, &empty) })
	_, e = f.svc.Delete(ctx, &c.DeleteRequest{Context: f.pc, Kind: c.KindTenants, ID: empty.ID})
	if e != nil {
		t.Fatal(e)
	}
	if f.count(ports.Tenants, eq("id", empty.ID)) != 0 {
		t.Fatal("empty pending tenant was not deleted")
	}
}

func TestMySQLApplicationDraftActivationBoundary(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	saved, e := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, App: &c.App{Code: "draftapp", Name: "Draft application", Status: ptr(1), Managed: true}})
	if e != nil {
		t.Fatal(e)
	}
	assertUnavailable := func() {
		t.Helper()
		q, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindApps, AvailableOnly: true})
		if e != nil {
			t.Fatal(e)
		}
		for _, app := range q.Apps {
			if app.ID == saved.ID {
				t.Fatal("unregistered draft appeared in application switcher")
			}
		}
	}
	assertUnavailable()
	var app catalog.App
	f.read(func(u ports.Unit) error { return u.Get(ports.Apps, saved.ID, &app) })
	if app.Status != 0 || app.Managed {
		t.Fatal("client fields promoted an unregistered draft")
	}
	_, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: saved.ID, Status: 1})
	wantCode(t, e, 409)
	for _, state := range []catalog.App{
		{Managed: true},
		{Managed: true, ServiceKey: "kerthus.app.draftapp"},
		{Managed: true, ServiceKey: "kerthus.app.draftapp", RoutePrefix: "/api/apps/draftapp/v1"},
	} {
		f.write(func(u ports.Unit) error {
			app.Managed = state.Managed
			app.ServiceKey = state.ServiceKey
			app.RoutePrefix = state.RoutePrefix
			app.Home = state.Home
			return u.Save(ports.Apps, &app)
		})
		_, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: saved.ID, Status: 1})
		wantCode(t, e, 409)
		assertUnavailable()
	}
	f.write(func(u ports.Unit) error { app.Home = "/draftapp/dashboard"; return u.Save(ports.Apps, &app) })
	if _, e = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: saved.ID, Status: 1}); e != nil {
		t.Fatal(e)
	}
	q, e := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindApps, AvailableOnly: true, ID: saved.ID})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Apps) != 1 {
		t.Fatal("registered application did not become available after explicit activation")
	}
}
