package core

import (
	"context"
	"sync"
	"testing"

	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLNewEmailCollisionCannotBreakAnotherLogin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.write(func(u ports.Unit) error { f.b.Email = "existing@example.invalid"; return u.Save(ports.Users, &f.b) })
	for _, actor := range []*c.Context{f.ac, f.pc} {
		_, err := f.svc.Save(ctx, &c.SaveRequest{Context: actor, Password: "integration-password", User: &c.User{ID: f.a.ID, Name: "must-not-save", Email: " EXISTING@example.invalid "}})
		wantCode(t, err, 409)
	}
	f.read(func(u ports.Unit) error {
		var got identity.User
		if err := u.Get(ports.Users, f.a.ID, &got); err != nil {
			return err
		}
		if got.Name != f.a.Name || got.Email != f.a.Email || got.AuthVersion != f.a.AuthVersion {
			t.Fatal("failed email update partially changed identity")
		}
		return nil
	})
	if _, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: f.b.Email, Password: "integration-password"}); err != nil {
		t.Fatalf("victim email login failed: %v", err)
	}
	before := f.count(ports.Users, ports.Filter{})
	_, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Password: "integration-password", Member: &c.Member{TenantID: f.ta.ID, AppID: f.app.ID, User: &c.User{Name: "new member", Phone: "13888888111", Email: "existing@example.invalid"}}})
	wantCode(t, err, 409)
	if f.count(ports.Users, ports.Filter{}) != before {
		t.Fatal("conflicting member created a global identity")
	}
	var pending tenant.Tenant
	f.write(func(u ports.Unit) error {
		pending = tenant.Tenant{Name: "email-collision-tenant", Status: 1, ContactPhone: "13888888222", ContactPerson: "pending", ContactEmail: "existing@example.invalid", AddressJSON: "[]"}
		return u.Save(ports.Tenants, &pending)
	})
	_, err = f.svc.ApproveTenant(ctx, &c.ApproveRequest{Context: f.pc, TenantID: pending.ID, Approved: true, AdminPassword: "integration-password"})
	wantCode(t, err, 409)
	if f.count(ports.Users, ports.Filter{}) != before || f.count(ports.Members, eq("tenant_id", pending.ID)) != 0 {
		t.Fatal("conflicting approval partially created an account or membership")
	}
}

func TestMySQLImportedDuplicateEmailCanBeSavedUnchanged(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.write(func(u ports.Unit) error {
		for _, user := range []*identity.User{&f.a, &f.b} {
			user.Email = "legacy-duplicate@example.invalid"
			if err := u.Save(ports.Users, user); err != nil {
				return err
			}
		}
		return nil
	})
	for _, user := range []identity.User{f.a, f.b} {
		_, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.pc, User: &c.User{ID: user.ID, Name: user.Name + " renamed", Email: user.Email, Avatar: user.Avatar, Sex: user.Sex}})
		if err != nil {
			t.Fatalf("unchanged imported duplicate could not be edited: %v", err)
		}
	}
	_, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: f.a.Email, Password: "integration-password"})
	wantCode(t, err, 401)
	if _, err = f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: "integration-password"}); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Password: "integration-password", User: &c.User{ID: f.a.ID, Name: f.a.Name, Email: "unique-after-migration@example.invalid"}})
	if err != nil {
		t.Fatalf("could not resolve an imported duplicate: %v", err)
	}
	for _, email := range []string{"unique-after-migration@example.invalid", f.b.Email} {
		if _, err = f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: email, Password: "integration-password"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMySQLConcurrentEmailUpdatesKeepOneLoginOwner(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, pair := range []struct {
		actor *c.Context
		user  identity.User
	}{{f.ac, f.a}, {f.bc, f.b}} {
		wg.Add(1)
		go func(actor *c.Context, user identity.User) {
			defer wg.Done()
			_, err := f.svc.Save(ctx, &c.SaveRequest{Context: actor, Password: "integration-password", User: &c.User{ID: user.ID, Name: user.Name, Email: "simultaneous@example.invalid"}})
			results <- err
		}(pair.actor, pair.user)
	}
	wg.Wait()
	close(results)
	successful := 0
	for err := range results {
		if err == nil {
			successful++
		} else {
			wantCode(t, err, 409)
		}
	}
	if successful != 1 || f.count(ports.Users, eq("email", "simultaneous@example.invalid")) != 1 {
		t.Fatal("concurrent writes created ambiguous email ownership")
	}
}
