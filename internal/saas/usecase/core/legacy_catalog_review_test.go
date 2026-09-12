package core

import (
	"context"
	"testing"

	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLLegacyCatalogFiltersBeforePagination(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		req   c.QueryRequest
		total int64
	}{
		{c.QueryRequest{Kind: c.KindApps, Code: "bas", Search: "Foundation"}, 1},
		{c.QueryRequest{Kind: c.KindApps, Code: "notes", Search: "Foundation"}, 0},
		{c.QueryRequest{Kind: c.KindApps, Code: "%"}, 0},
		{c.QueryRequest{Kind: c.KindTenantApps, TenantName: f.ta.Name, AppName: "Found"}, 1},
		{c.QueryRequest{Kind: c.KindTenantApps, TenantName: f.ta.Name, AppName: "Notes"}, 0},
		{c.QueryRequest{Kind: c.KindTenantApps, TenantName: "%"}, 0},
		{c.QueryRequest{Kind: c.KindTenantApps, TargetTenantID: f.ta.ID, TenantName: f.tb.Name}, 0},
		{c.QueryRequest{Kind: c.KindTenantApps, AppName: "Foundation", Page: 2, PageSize: 1}, 2},
	} {
		tc.req.Context = f.pc
		out, err := f.svc.Query(ctx, &tc.req)
		if err != nil || out.Total != tc.total {
			t.Fatalf("catalog filters/count mismatch: %+v result=%+v err=%v", tc.req, out, err)
		}
		if tc.req.Page == 2 && (len(out.TenantApps) != 1 || out.TenantApps[0].TenantID != f.tb.ID) {
			t.Fatal("filtered entitlement page returned wrong row")
		}
	}
}

func TestMySQLLegacyResourceFieldsAndTreeSortPersist(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var added []int64
	for _, priority := range []int32{9, 2, 2} {
		req := &c.Resource{AppID: f.app.ID, Code: "basic:review:" + string(rune('a'+len(added))), Name: "Review", Type: "menu", Path: "/basic/review", Component: "LAYOUT", Redirect: "/basic/home", Remark: "Original note", Sort: priority}
		saved, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: req})
		if err != nil {
			t.Fatal(err)
		}
		added = append(added, saved.ID)
		out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, ID: saved.ID})
		if err != nil || len(out.Resources) != 1 || out.Resources[0].Redirect != req.Redirect || out.Resources[0].Remark != req.Remark {
			t.Fatal("saved resource display fields did not round trip", err)
		}
	}
	for i, id := range []int64{added[1], added[2], added[0]} {
		out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindResources, Search: "Review", Page: int32(i + 1), PageSize: 1})
		if err != nil || out.Total != 3 || len(out.Resources) != 1 || out.Resources[0].ID != id {
			t.Fatal("sort priority or stable page ordering lost", err)
		}
	}
	_, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Resource: &c.Resource{AppID: f.app.ID, Code: "basic:outside", Name: "Unsupported", Type: "view", OpenWith: "outside"}})
	wantCode(t, err, 400)
	var first, second organization.Org
	f.write(func(u ports.Unit) error {
		first = organization.Org{TenantID: f.ta.ID, Type: "unit", Name: "Sort First ID", Status: 1, Sort: 20}
		second = organization.Org{TenantID: f.ta.ID, Type: "unit", Name: "Sort Second ID", Status: 1, Sort: 10}
		if err := u.Save(ports.Orgs, &first); err != nil {
			return err
		}
		return u.Save(ports.Orgs, &second)
	})
	out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindOrgs, TargetTenantID: f.ta.ID, Search: "Sort", PageSize: 1})
	if err != nil || out.Total != 2 || len(out.Orgs) != 1 || out.Orgs[0].ID != second.ID {
		t.Fatal("organization sort ignored", err)
	}
	// New fields do not add grants or alter resource identities.
	if f.count(ports.Resources, ports.Filter{In: map[string][]int64{"id": added}}) != 3 {
		t.Fatal("resource edit duplicated identities")
	}
}
