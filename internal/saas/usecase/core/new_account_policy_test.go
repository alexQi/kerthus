package core

import (
	"context"
	"testing"

	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLNewAccountsDefaultToLegacySingleSessionPolicy(t *testing.T) {
	for _, entry := range []string{"member", "tenant-approval", "platform-bootstrap"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			const phone = "13888888999"
			const password = "integration-password"
			switch entry {
			case "member":
				_, err := f.svc.Save(ctx, &c.SaveRequest{Context: f.ac, Password: password, Member: &c.Member{TenantID: f.ta.ID, AppID: f.app.ID, User: &c.User{Name: "single-session member", Phone: phone}}})
				if err != nil {
					t.Fatal(err)
				}
			case "tenant-approval":
				var pending tenant.Tenant
				f.write(func(u ports.Unit) error {
					pending = tenant.Tenant{Name: "single-session tenant", Status: 1, ContactPhone: phone, AddressJSON: "[]"}
					return u.Save(ports.Tenants, &pending)
				})
				if _, err := f.svc.ApproveTenant(ctx, &c.ApproveRequest{Context: f.pc, TenantID: pending.ID, Approved: true, AdminPassword: password}); err != nil {
					t.Fatal(err)
				}
			case "platform-bootstrap":
				f.write(func(u ports.Unit) error {
					f.platform.PlatformAdmin = false
					if err := u.Save(ports.Users, &f.platform); err != nil {
						return err
					}
					return u.Save(ports.Apps, &catalog.App{Code: "system", Name: "Platform", Status: 1})
				})
				if err := f.svc.Bootstrap(ctx, phone, password); err != nil {
					t.Fatal(err)
				}
			}
			var created identity.User
			f.read(func(u ports.Unit) error {
				var err error
				created, err = first[identity.User](u, ports.Users, eq("phone", phone))
				return err
			})
			if !created.SingleLogin {
				t.Fatal("new account did not retain the legacy default single-session policy")
			}
			firstLogin, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: phone, Password: password})
			if err != nil {
				t.Fatal(err)
			}
			secondLogin, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: phone, Password: password})
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.svc.Profile(ctx, &c.Context{Token: firstLogin.AccessToken, TenantID: firstLogin.TenantID})
			wantCode(t, err, 401)
			if _, err = f.svc.Profile(ctx, &c.Context{Token: secondLogin.AccessToken, TenantID: secondLogin.TenantID}); err != nil {
				t.Fatalf("latest login must remain valid: %v", err)
			}
			// No migration or account-creation path changes an existing account's policy.
			f.read(func(u ports.Unit) error {
				var existing identity.User
				if err := u.Get(ports.Users, f.a.ID, &existing); err != nil {
					return err
				}
				if existing.SingleLogin {
					t.Fatal("new account creation changed an existing multi-session account")
				}
				return nil
			})
			if entry == "platform-bootstrap" {
				f.write(func(u ports.Unit) error { created.SingleLogin = false; return u.Save(ports.Users, &created) })
				if err := f.svc.Bootstrap(ctx, phone, "different-bootstrap-password"); err != nil {
					t.Fatal(err)
				}
				f.read(func(u ports.Unit) error {
					var existing identity.User
					if err := u.Get(ports.Users, created.ID, &existing); err != nil {
						return err
					}
					if existing.SingleLogin {
						t.Fatal("repeated bootstrap reset an existing session policy")
					}
					return nil
				})
			}
		})
	}
}
