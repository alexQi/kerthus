package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLAppDisplayMetadataPersistsClearsAndSurvivesRegistration(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	app := &c.App{Code: "display-test", Name: "Display app", Version: "1", Icon: "public/1/2/icon.png", Description: "应用说明", Remark: "管理备注"}
	created, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, App: app})
	if err != nil {
		t.Fatal(err)
	}
	app.ID = created.ID
	read := func() *c.App {
		t.Helper()
		out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindApps, ID: app.ID})
		if err != nil || len(out.Apps) != 1 {
			t.Fatal("app query", err)
		}
		return out.Apps[0]
	}
	if got := read(); got.Icon != app.Icon || got.Description != app.Description || got.Remark != app.Remark || *got.Status != 0 {
		t.Fatal("new app lost presentation or escaped draft state")
	}
	definition := Definition{App: catalog.App{Code: app.Code, Name: app.Name, Version: "2", ResourceVersion: 1, ManifestHash: "display-v1", ServiceKey: "kerthus.app.display-test", RoutePrefix: "/api/apps/display-test/v1", Home: "/display-test/dashboard", FrontendEntry: "bundled"}}
	if err = f.svc.RegisterApplication(ctx, definition); err != nil {
		t.Fatal(err)
	}
	if got := read(); got.Icon != app.Icon || got.Description != app.Description || got.Remark != app.Remark || !got.Managed {
		t.Fatal("registration overwrote editor-owned display metadata")
	}
	app.Icon, app.Description, app.Remark = "", "", ""
	if _, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, App: app}); err != nil {
		t.Fatal(err)
	}
	if got := read(); got.Icon != "" || got.Description != "" || got.Remark != "" || got.ServiceKey != definition.App.ServiceKey {
		t.Fatal("explicit clear failed or modified protected routing")
	}
}

func TestMySQLTenantAppsKeepEntitlementIdentityExpiryAndPageFilters(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	expiry := time.Now().Add(48 * time.Hour).Unix()
	var apps []catalog.App
	entitlementIDs := map[int64]int64{}
	f.write(func(u ports.Unit) error {
		for i := 0; i < 6; i++ {
			app := catalog.App{Code: fmt.Sprintf("owned-%d", i), Name: fmt.Sprintf("Tenant demo %d", i), Status: 1, Icon: "icon.png", Description: "Demo"}
			if i == 3 {
				app.Status = 0
			}
			if err := u.Save(ports.Apps, &app); err != nil {
				return err
			}
			entitlement := catalog.Entitlement{TenantID: f.ta.ID, AppID: app.ID, Status: 1, ExpiresAt: expiry}
			if i == 4 {
				entitlement.ExpiresAt = time.Now().Add(-time.Hour).Unix()
			}
			if i == 5 {
				entitlement.TenantID = f.tb.ID
			}
			if err := u.Save(ports.Entitlements, &entitlement); err != nil {
				return err
			}
			apps = append(apps, app)
			entitlementIDs[app.ID] = entitlement.ID
		}
		return nil
	})
	for _, test := range []struct {
		name, code string
		page       int32
		id         int64
		total      int64
	}{
		{"second page", "", 2, apps[1].ID, 3},
		{"code intersection", "owned-2", 1, apps[2].ID, 1},
		{"code miss", "missing", 1, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindApps, TargetTenantID: f.ta.ID, AvailableOnly: true, TenantAppsOnly: true, Search: "Tenant demo", Code: test.code, Page: test.page, PageSize: 1})
			if err != nil || out.Total != test.total {
				t.Fatalf("query total: %v %+v", err, out)
			}
			if test.id == 0 {
				if len(out.Apps) != 0 {
					t.Fatal("nonmatching app returned")
				}
				return
			}
			if len(out.Apps) != 1 {
				t.Fatal("application pagination ignored")
			}
			app := out.Apps[0]
			if app.ID != test.id || app.TenantAppID != entitlementIDs[app.ID] || app.ExpiresAt != expiry || app.Icon != "icon.png" || app.Description != "Demo" {
				t.Fatal("app lost its target-tenant entitlement or metadata")
			}
		})
	}
	// Platform catalog visibility must not invent revocable tenant-app records.
	out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindApps, AvailableOnly: true, TenantAppsOnly: true})
	if err != nil || out.Total != 0 {
		t.Fatal("tenant application list included an unowned platform catalog entry", err)
	}
	_, err = f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindApps, TargetTenantID: f.tb.ID, AvailableOnly: true, TenantAppsOnly: true})
	wantCode(t, err, 403)
}

func TestMySQLPositionOptionsExcludeDisabledBeforePagination(t *testing.T) {
	f := newFixture(t)
	var disabled organization.Position
	f.write(func(u ports.Unit) error {
		disabled = organization.Position{TenantID: f.ta.ID, OrgID: f.orgA.ID, Name: "Disabled option", Status: 0}
		return u.Save(ports.Positions, &disabled)
	})
	for _, available := range []bool{false, true} {
		out, err := f.svc.Query(context.Background(), &c.QueryRequest{Context: f.ac, Kind: c.KindPositions, AvailableOnly: available, Page: 2, PageSize: 1})
		if err != nil {
			t.Fatal(err)
		}
		if available {
			if out.Total != 1 || len(out.Positions) != 0 {
				t.Fatal("disabled position is still selectable or counted")
			}
		} else if out.Total != 2 || len(out.Positions) != 1 || out.Positions[0].ID != disabled.ID {
			t.Fatal("management list lost disabled position")
		}
	}
}

func TestMySQLTenantOptionsExcludePendingRejectedAndDisabled(t *testing.T) {
	f := newFixture(t)
	f.write(func(u ports.Unit) error {
		for _, row := range []tenant.Tenant{{Name: "Pending", Status: 1, VerifyStatus: 0, AddressJSON: "[]"}, {Name: "Rejected", Status: 1, VerifyStatus: 2, AddressJSON: "[]"}, {Name: "Disabled", Status: 0, VerifyStatus: 1, AddressJSON: "[]"}} {
			if err := u.Save(ports.Tenants, &row); err != nil {
				return err
			}
		}
		return nil
	})
	for _, available := range []bool{true, false} {
		out, err := f.svc.Query(context.Background(), &c.QueryRequest{Context: f.pc, Kind: c.KindTenants, AvailableOnly: available})
		if err != nil {
			t.Fatal(err)
		}
		want := int64(6)
		if available {
			want = 3
		}
		if out.Total != want || len(out.Tenants) != int(want) {
			t.Fatal("tenant option lifecycle filter changed ordinary management listing")
		}
		if available {
			for _, tenant := range out.Tenants {
				if tenant.VerifyStatus != 1 || *tenant.Status != 1 {
					t.Fatal("unusable tenant option")
				}
			}
		}
	}
}
