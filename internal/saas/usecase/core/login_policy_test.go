package core

import (
	"context"
	"strings"
	"testing"

	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLLoginPreservesImportedSessionPolicy(t *testing.T) {
	for _, single := range []bool{false, true} {
		name := "multiple"
		if single {
			name = "single"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.write(func(u ports.Unit) error {
				var user identity.User
				if e := u.Get(ports.Users, f.a.ID, &user); e != nil {
					return e
				}
				user.PasswordHash, user.SingleLogin = legacyFixture, single
				user.Email = "login@example.invalid"
				return u.Save(ports.Users, &user)
			})
			if _, e := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: "wrong"}); e == nil {
				t.Fatal("wrong credential accepted")
			}
			if _, e := f.svc.Profile(ctx, f.ac); e != nil {
				t.Fatalf("failed login revoked existing session: %v", e)
			}
			phone, e := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: " " + f.a.Phone + " ", Password: "123456"})
			if e != nil {
				t.Fatal(e)
			}
			phoneContext := &c.Context{Token: phone.AccessToken, TenantID: phone.TenantID}
			if _, e := f.svc.Profile(ctx, phoneContext); e != nil {
				t.Fatalf("legacy upgrade issued invalid session: %v", e)
			}
			email, e := f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: " login@example.invalid ", Password: "123456"})
			if e != nil {
				t.Fatalf("email login after password upgrade: %v", e)
			}
			if _, e := f.svc.Profile(ctx, &c.Context{Token: email.AccessToken, TenantID: email.TenantID}); e != nil {
				t.Fatalf("newest session invalid: %v", e)
			}
			for _, old := range []*c.Context{f.ac, phoneContext} {
				_, e := f.svc.Profile(ctx, old)
				if single {
					wantCode(t, e, 401)
				} else if e != nil {
					t.Fatalf("multi-session account was restricted: %v", e)
				}
			}
			f.read(func(u ports.Unit) error {
				var user identity.User
				if e := u.Get(ports.Users, f.a.ID, &user); e != nil {
					return e
				}
				wantVersion := f.a.AuthVersion
				if single {
					wantVersion += 2
				}
				if user.AuthVersion != wantVersion || user.SingleLogin != single || strings.HasPrefix(user.PasswordHash, LegacyPasswordPrefix) || !verifyPassword(user.PasswordHash, "123456") {
					t.Fatal("login changed policy, credential or version incorrectly")
				}
				return nil
			})
		})
	}
}

func TestMySQLConcurrentSingleLoginCannotReissueOldVersion(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name, password := "bcrypt", "integration-password"
		if legacy {
			name, password = "legacy", "123456"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.write(func(u ports.Unit) error {
				var user identity.User
				if e := u.Get(ports.Users, f.a.ID, &user); e != nil {
					return e
				}
				user.SingleLogin = true
				if legacy {
					user.PasswordHash = legacyFixture
				}
				return u.Save(ports.Users, &user)
			})
			request := &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: password}
			var winner *c.LoginReply
			wrapped := &beforeUpgradeDB{Database: f.db, afterRead: func() error {
				var e error
				winner, e = f.svc.Login(ctx, request)
				return e
			}}
			before := len(f.svc.Sessions.(*memorySessions).values)
			_, e := New(wrapped, f.svc.Sessions, nil).Login(ctx, request)
			wantCode(t, e, 401)
			if len(f.svc.Sessions.(*memorySessions).values) != before+1 {
				t.Fatal("concurrent login issued an extra session")
			}
			if _, e := f.svc.Profile(ctx, &c.Context{Token: winner.AccessToken, TenantID: winner.TenantID}); e != nil {
				t.Fatalf("winning login invalidated: %v", e)
			}
			_, e = f.svc.Profile(ctx, f.ac)
			wantCode(t, e, 401)
		})
	}
}
