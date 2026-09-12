package core

import (
	"context"
	"testing"

	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLThirdPartyAppLifecycleAndTenantBoundary(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	input := &c.App{Code: "partner", Name: "Partner", Type: "thrid", URL: "https://partner.example/start", IsPublic: true}
	saved, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, App: input})
	if err != nil {
		t.Fatal(err)
	}
	input.ID = saved.ID
	if _, err = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: saved.ID, Status: 1}); err != nil {
		t.Fatal(err)
	}
	find := func(actor *c.Context, want int64) {
		t.Helper()
		out, err := f.svc.Query(ctx, &c.QueryRequest{Context: actor, Kind: c.KindApps, AvailableOnly: true, ID: saved.ID})
		if want == 0 {
			wantCode(t, err, 404)
			return
		}
		if err != nil || out.Total != want {
			t.Fatalf("third-party availability: want=%d out=%+v err=%v", want, out, err)
		}
		if want > 0 && (out.Apps[0].Type != "third" || out.Apps[0].URL != input.URL || !out.Apps[0].IsPublic || out.Apps[0].ServiceKey != "") {
			t.Fatal("third-party metadata or routing changed")
		}
	}
	find(f.pc, 1)
	find(f.ac, 0) // Public describes publication; it never grants tenant access.
	if _, err = f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID}, Apps: []*c.AppGrant{{AppID: saved.ID}}}); err != nil {
		t.Fatal(err)
	}
	find(f.ac, 1)
	find(f.bc, 0)
	selected := *f.ac
	selected.AppID = saved.ID
	_, err = f.svc.Auth(ctx, &selected)
	wantCode(t, err, 403)
	_, err = f.svc.CheckAccess(ctx, &c.AccessRequest{Context: &selected, AppCode: input.Code, Method: "GET", Path: "/"})
	wantCode(t, err, 403)
	// An imported preference for a link must not route the next login to nowhere.
	f.write(func(u ports.Unit) error {
		m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", f.ta.ID, "user_id", f.a.ID))
		if e != nil {
			return e
		}
		m.DefaultAppID = saved.ID
		return u.Save(ports.Members, &m)
	})
	login, err := f.svc.Login(ctx, &c.LoginRequest{Username: f.a.Phone, Scene: "phone", Password: "integration-password"})
	if err != nil || login.AppID == saved.ID {
		t.Fatal("third-party link selected as login context", err)
	}
	input.URL = "javascript:alert(1)"
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, App: input})
	wantCode(t, err, 400)
	input.URL, input.Type = "", "self"
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, App: input})
	wantCode(t, err, 400)
	if _, err = f.svc.SetEntitlements(ctx, &c.EntitlementsRequest{Context: f.pc, TenantIDs: []int64{f.ta.ID}, Apps: []*c.AppGrant{{AppID: saved.ID, ExpiresAt: 1}}}); err != nil {
		t.Fatal(err)
	}
	find(f.ac, 0)
	if _, err = f.svc.SetStatus(ctx, &c.StatusRequest{Context: f.pc, Kind: c.KindApps, ID: saved.ID, Status: 0}); err != nil {
		t.Fatal(err)
	}
	find(f.pc, 0)
}

func TestMySQLLinkResourcesPreserveNavigationAndAuthorization(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	inside := &c.Resource{AppID: f.app.ID, Code: "basic:embedded", Name: "Embedded", Type: "menu", OpenWith: "inside", Path: "/basic/embedded", Component: "https://partner.example/embed"}
	outside := &c.Resource{AppID: f.app.ID, Code: "basic:external", Name: "External", Type: "menu", OpenWith: "outside", Path: "https://partner.example/open"}
	for _, input := range []*c.Resource{inside, outside} {
		saved, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: input})
		if err != nil {
			t.Fatal(err)
		}
		input.ID = saved.ID
		result, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, ID: saved.ID})
		if err != nil || len(result.Resources) != 1 || result.Resources[0].OpenWith != input.OpenWith || result.Resources[0].Path != input.Path || result.Resources[0].Component != input.Component {
			t.Fatal("link resource did not round trip", err)
		}
	}
	out, err := f.svc.Auth(ctx, f.ac)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range out.Resources {
		if r.ID == inside.ID || r.ID == outside.ID {
			t.Fatal("link resource auto-granted")
		}
	}
	child := &c.Resource{AppID: f.app.ID, ParentID: outside.ID, Code: "basic:child", Name: "Child", Type: "action"}
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: child})
	wantCode(t, err, 400)
	child.ParentID = inside.ID
	if _, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: child}); err != nil {
		t.Fatal(err)
	}
	inside.OpenWith, inside.Path, inside.Component = "outside", "https://partner.example", ""
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: inside})
	wantCode(t, err, 400)
	outside.Path = "//partner.example"
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: outside})
	wantCode(t, err, 400)
	home := &c.Resource{AppID: f.app.ID, Code: "basic:launch-home", Name: "Home", Type: "view", Path: "/basic/launch-home", Component: "LAYOUT"}
	saved, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: home})
	if err != nil {
		t.Fatal(err)
	}
	home.ID = saved.ID
	f.write(func(u ports.Unit) error {
		var app catalog.App
		if e := u.Get(ports.Apps, f.app.ID, &app); e != nil {
			return e
		}
		app.Home = home.Path
		return u.Save(ports.Apps, &app)
	})
	home.OpenWith, home.Path, home.Component = "outside", "https://partner.example", ""
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: home})
	wantCode(t, err, 400)
}
