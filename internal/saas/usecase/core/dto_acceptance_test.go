package core

import (
	"context"
	"testing"

	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLDisplayTimestampsAreServerOwned(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userCreated := f.shared.CreatedAt
	positionCreated := f.positionA.CreatedAt
	if userCreated <= 0 || positionCreated <= 0 {
		t.Fatal("server did not initialize entity timestamps")
	}
	_, e := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, User: &c.User{ID: f.shared.ID, Name: f.shared.Name, Email: f.shared.Email, CreatedAt: 1, UpdatedAt: 2}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Position: &c.Position{ID: f.positionA.ID, OrgID: f.orgA.ID, Name: "Updated position", CreatedAt: 1, UpdatedAt: 2}})
	if e != nil {
		t.Fatal(e)
	}
	f.read(func(u ports.Unit) error {
		var user identity.User
		var position organization.Position
		if e := u.Get(ports.Users, f.shared.ID, &user); e != nil {
			return e
		}
		if e := u.Get(ports.Positions, f.positionA.ID, &position); e != nil {
			return e
		}
		if user.CreatedAt != userCreated || position.CreatedAt != positionCreated || user.UpdatedAt <= 2 || position.UpdatedAt <= 2 {
			t.Fatal("client overwrote server-owned timestamps")
		}
		return nil
	})
	for _, kind := range []c.Kind{c.KindUsers, c.KindMembers, c.KindPositions, c.KindTenantApps} {
		q := &c.QueryRequest{Context: f.pc, Kind: kind, TargetTenantID: f.ta.ID}
		if kind == c.KindUsers || kind == c.KindMembers {
			q.ID = f.shared.ID
		}
		if kind == c.KindPositions {
			q.ID = f.positionA.ID
		}
		out, e := f.svc.Query(ctx, q)
		if e != nil {
			t.Fatal(e)
		}
		var created, updated int64
		switch kind {
		case c.KindUsers:
			created, updated = out.Users[0].CreatedAt, out.Users[0].UpdatedAt
		case c.KindMembers:
			created, updated = out.Members[0].User.CreatedAt, out.Members[0].User.UpdatedAt
		case c.KindPositions:
			created, updated = out.Positions[0].CreatedAt, out.Positions[0].UpdatedAt
		case c.KindTenantApps:
			created, updated = out.TenantApps[0].CreatedAt, out.TenantApps[0].UpdatedAt
		}
		if created <= 2 || updated <= 2 {
			t.Fatalf("query kind %d omitted display timestamps", kind)
		}
	}
	created, e := f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Position: &c.Position{OrgID: f.orgA.ID, Name: "New position", CreatedAt: 1, UpdatedAt: 2}})
	if e != nil {
		t.Fatal(e)
	}
	f.read(func(u ports.Unit) error {
		var position organization.Position
		if e := u.Get(ports.Positions, created.ID, &position); e != nil {
			return e
		}
		if position.CreatedAt <= 2 || position.UpdatedAt <= 2 {
			t.Fatal("new entity accepted client timestamps")
		}
		return nil
	})
}
