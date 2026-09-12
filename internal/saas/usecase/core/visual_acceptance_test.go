package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLUnavailableDefaultAppPreservedOnlyForSameExistingMembership(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.write(func(u ports.Unit) error {
		f.extension.Status = 0
		if err := u.Save(ports.Apps, &f.extension); err != nil {
			return err
		}
		member, err := first[tenant.Member](u, ports.Members, filter("tenant_id", f.ta.ID, "user_id", f.shared.ID))
		if err != nil {
			return err
		}
		member.DefaultAppID = f.extension.ID
		return u.Save(ports.Members, &member)
	})
	member := &c.Member{TenantID: f.ta.ID, User: userDTO(f.shared), AppID: f.extension.ID, OrgIDs: []int64{f.orgA.ID}}
	if _, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Member: member}); err != nil {
		t.Fatal("tenant administrator cannot preserve old default while changing organization", err)
	}
	member.User.Name = "renamed legacy member"
	if _, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: member}); err != nil {
		t.Fatal("cross-tenant platform edit cannot preserve old default", err)
	}
	// Another membership of the same global account has no such old default.
	foreign := *member
	foreign.TenantID, foreign.OrgIDs = f.tb.ID, nil
	_, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: &foreign})
	wantCode(t, err, 400)
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Member: &foreign})
	wantCode(t, err, 403)
	// Even a platform administrator cannot introduce an unavailable default in
	// its current tenant; preserving historical state is the only exception.
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: &c.Member{TenantID: f.platformTenant.ID, User: userDTO(f.platform), AppID: f.extension.ID}})
	wantCode(t, err, 400)
	before := f.count(ports.Users, ports.Filter{})
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Password: "integration-password", Member: &c.Member{TenantID: f.ta.ID, User: &c.User{Name: "new", Phone: "13988889999"}, AppID: f.extension.ID}})
	wantCode(t, err, 400)
	if f.count(ports.Users, ports.Filter{}) != before {
		t.Fatal("rejected default created a global account")
	}
	// Still-active apps with no entitlement are also not valid new selections.
	f.write(func(u ports.Unit) error {
		f.extension.Status = 1
		return u.Save(ports.Apps, &f.extension)
	})
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: &foreign})
	wantCode(t, err, 400)
	if _, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: member}); err != nil {
		t.Fatal("unchanged but unentitled historical default was rejected", err)
	}
	// It remains unavailable for authentication, regardless of preserved data.
	selected := *f.sc
	selected.AppID = f.extension.ID
	_, err = f.svc.Auth(ctx, &selected)
	wantCode(t, err, 403)
	// A deliberate switch to an available app succeeds and ends the exception.
	member.AppID = f.app.ID
	if _, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: member}); err != nil {
		t.Fatal(err)
	}
	member.AppID = f.extension.ID
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, Member: member})
	wantCode(t, err, 400)
	f.read(func(u ports.Unit) error {
		stored, err := first[tenant.Member](u, ports.Members, filter("tenant_id", f.ta.ID, "user_id", f.shared.ID))
		if err != nil {
			return err
		}
		if stored.DefaultAppID != f.app.ID {
			t.Fatal("rejected change altered stored default")
		}
		foreignMember, err := first[tenant.Member](u, ports.Members, filter("tenant_id", f.tb.ID, "user_id", f.shared.ID))
		if err != nil {
			return err
		}
		if foreignMember.DefaultAppID != f.app.ID {
			t.Fatal("foreign membership was changed")
		}
		var entitlements []catalog.Entitlement
		if err = u.Find(ports.Entitlements, filter("tenant_id", f.ta.ID, "app_id", f.extension.ID), &entitlements); err != nil {
			return err
		}
		if len(entitlements) != 0 {
			t.Fatal("preserving a default accidentally granted application access")
		}
		return nil
	})
}

func TestMySQLTenantCreatedRangeFiltersBeforePaginationAndCounts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := time.Date(2026, 9, 11, 0, 0, 0, 0, time.FixedZone("CST", 8*3600)).Unix()
	end := start + 86400
	var ids []int64
	f.write(func(u ports.Unit) error {
		for i, stamp := range []int64{start - 1, start, start + 3600, end - 1, end} {
			row := tenant.Tenant{Name: fmt.Sprintf("date-fixture-%d", i), Status: 1, AddressJSON: "[]", CreatedAt: stamp}
			if err := u.Save(ports.Tenants, &row); err != nil {
				return err
			}
			ids = append(ids, row.ID)
		}
		return nil
	})
	for _, test := range []struct {
		name           string
		from, before   int64
		page, pageSize int32
		want           []int64
		total          int64
	}{
		{"inclusive-start-exclusive-end", start, end, 1, 100, ids[1:4], 3},
		{"range-before-pagination", start, end, 2, 1, ids[2:3], 3},
		{"page-after-end", start, end, 4, 1, nil, 3},
		{"lower-bound-only", start, 0, 1, 100, ids[1:], 4},
		{"upper-bound-only", 0, end, 1, 100, ids[:4], 4},
		{"one-second-last-day", end - 1, end, 1, 100, ids[3:4], 1},
		{"empty-future-window", end + 1, end + 2, 1, 100, nil, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := &c.QueryRequest{Context: f.pc, Kind: c.KindTenants, Search: "date-fixture", CreatedFrom: test.from, CreatedBefore: test.before, Page: test.page, PageSize: test.pageSize}
			out, err := f.svc.Query(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if out.Total != test.total || len(out.Tenants) != len(test.want) {
				t.Fatalf("page/count mismatch: got %d/%d, want %d/%d", len(out.Tenants), out.Total, len(test.want), test.total)
			}
			for i, id := range test.want {
				if out.Tenants[i].ID != id {
					t.Fatal("wrong tenant survived time range")
				}
			}
		})
	}
	for _, bounds := range [][2]int64{{-1, end}, {start, -1}, {end, start}, {start, start}} {
		_, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.pc, Kind: c.KindTenants, CreatedFrom: bounds[0], CreatedBefore: bounds[1]})
		wantCode(t, err, 400)
	}
	_, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindTenants, CreatedFrom: start, CreatedBefore: end})
	wantCode(t, err, 403)
}
