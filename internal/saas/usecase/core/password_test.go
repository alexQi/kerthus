package core

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

// Synthetic public fixture from the original PHP algorithm, never real credentials.
const legacyFixture = "legacy-beehive$22b8a3d1df47ac2f82b46e9f10cfb1c2"

func TestLegacyPasswordCompatibility(t *testing.T) {
	if !verifyPassword(legacyFixture, "123456") {
		t.Fatal("legacy fixture rejected")
	}
	for _, tc := range []struct{ stored, password string }{
		{legacyFixture, "wrong"}, {legacyFixture, ""}, {legacyFixture, strings.Repeat("x", 73)},
		{strings.TrimPrefix(legacyFixture, LegacyPasswordPrefix), "123456"},
		{LegacyPasswordPrefix + strings.Repeat("a", 31), "123456"},
		{LegacyPasswordPrefix + strings.Repeat("z", 32), "123456"},
		{"legacy-beehive$" + strings.ToUpper(strings.TrimPrefix(legacyFixture, LegacyPasswordPrefix)), "123456"},
	} {
		if verifyPassword(tc.stored, tc.password) {
			t.Fatal("invalid legacy credential accepted")
		}
	}
	b, e := bcrypt.GenerateFromPassword([]byte("new-valid-password"), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	if !verifyPassword(string(b), "new-valid-password") || verifyPassword(string(b), "wrong") {
		t.Fatal("bcrypt compatibility failed")
	}
}

type passwordUnit struct {
	ports.Unit
	user   identity.User
	users  []identity.User
	saved  bool
	filter ports.Filter
}

func (u *passwordUnit) Get(_ ports.Table, _ int64, out any) error {
	*out.(*identity.User) = u.user
	return nil
}
func (u *passwordUnit) Save(_ ports.Table, in any) error {
	u.saved = true
	u.user = *in.(*identity.User)
	return nil
}
func (u *passwordUnit) Find(_ ports.Table, f ports.Filter, out any) error {
	u.filter = f
	*out.(*[]identity.User) = u.users
	return nil
}

type passwordDB struct {
	ports.Database
	unit *passwordUnit
}

func (db *passwordDB) Read(_ context.Context, fn func(ports.Unit) error) error  { return fn(db.unit) }
func (db *passwordDB) Write(_ context.Context, fn func(ports.Unit) error) error { return fn(db.unit) }

func TestLegacyPasswordUpgradeComparesCurrentAccount(t *testing.T) {
	original := identity.User{ID: 7, Name: "old snapshot", Status: 1, AuthVersion: 9, PasswordHash: legacyFixture}
	for _, tc := range []struct {
		name    string
		change  func(*identity.User)
		allowed bool
	}{
		{"unchanged", func(u *identity.User) { u.Name = "latest profile" }, true},
		{"disabled", func(u *identity.User) { u.Status = 0 }, false},
		{"revoked", func(u *identity.User) { u.AuthVersion++ }, false},
		{"password changed", func(u *identity.User) { u.PasswordHash = "a newer stored password" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := &passwordUnit{user: original}
			tc.change(&u.user)
			before := u.user
			svc := New(&passwordDB{unit: u}, nil, nil)
			authenticated := original
			e := svc.finalizeLogin(context.Background(), &authenticated, "123456")
			if !tc.allowed {
				wantCode(t, e, 401)
				if u.saved || u.user != before {
					t.Fatal("concurrent account change overwritten")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if !u.saved || u.user.AuthVersion != original.AuthVersion || u.user.Name != "latest profile" || !verifyPassword(u.user.PasswordHash, "123456") || strings.HasPrefix(u.user.PasswordHash, LegacyPasswordPrefix) {
				t.Fatal("upgrade failed to preserve latest account fields and credential")
			}
		})
	}
}

func TestLoginRejectsAmbiguousEmail(t *testing.T) {
	u := &passwordUnit{users: []identity.User{{ID: 1, Status: 1, PasswordHash: legacyFixture}, {ID: 2, Status: 0, PasswordHash: legacyFixture}}}
	svc := New(&passwordDB{unit: u}, nil, nil)
	_, e := svc.Login(context.Background(), &c.LoginRequest{Scene: "email", Username: "duplicate@example.invalid", Password: "123456"})
	wantCode(t, e, 401)
	if u.filter.PageSize != 2 {
		t.Fatal("login must detect at least two matching accounts")
	}
	if u.saved {
		t.Fatal("ambiguous account mutated")
	}
}

func TestMySQLLegacyLoginUpgradeAndExistingPasswordOperations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	install := func() {
		f.write(func(u ports.Unit) error {
			var user identity.User
			if e := u.Get(ports.Users, f.a.ID, &user); e != nil {
				return e
			}
			user.PasswordHash = legacyFixture
			return u.Save(ports.Users, &user)
		})
	}
	install()
	reply, e := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.svc.Profile(ctx, &c.Context{Token: reply.AccessToken, TenantID: reply.TenantID}); e != nil {
		t.Fatalf("upgraded login session invalid: %v", e)
	}
	f.read(func(u ports.Unit) error {
		var user identity.User
		if e := u.Get(ports.Users, f.a.ID, &user); e != nil {
			return e
		}
		if user.AuthVersion != f.a.AuthVersion || strings.HasPrefix(user.PasswordHash, LegacyPasswordPrefix) || !verifyPassword(user.PasswordHash, "123456") {
			t.Fatal("login failed to upgrade password preserving session version")
		}
		return nil
	})
	install()
	if _, e = f.svc.ChangePassword(ctx, &c.PasswordRequest{Context: f.ac, OldPassword: "123456", NewPassword: "replacement-password"}); e != nil {
		t.Fatalf("legacy change password: %v", e)
	}
	// Profile email validation also accepts an imported password before first login.
	f.write(func(u ports.Unit) error {
		var user identity.User
		if e := u.Get(ports.Users, f.shared.ID, &user); e != nil {
			return e
		}
		user.PasswordHash = legacyFixture
		return u.Save(ports.Users, &user)
	})
	if _, e = f.svc.Save(ctx, &c.SaveRequest{Context: f.sc, Password: "123456", User: &c.User{ID: f.shared.ID, Name: f.shared.Name, Email: "changed@example.invalid"}}); e != nil {
		t.Fatalf("legacy email password verification: %v", e)
	}
}

// beforeUpgradeDB injects a committed account update after the login read
// transaction finishes, reproducing the interleaving without timing-dependent sleeps.
type beforeUpgradeDB struct {
	ports.Database
	afterRead func() error
}

func (db *beforeUpgradeDB) Read(ctx context.Context, fn func(ports.Unit) error) error {
	if e := db.Database.Read(ctx, fn); e != nil {
		return e
	}
	hook := db.afterRead
	db.afterRead = nil
	if hook != nil {
		return hook()
	}
	return nil
}

func TestMySQLLegacyUpgradeCannotOverwriteConcurrentChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*identity.User)
	}{
		{"disable", func(u *identity.User) { u.Status = 0 }},
		{"revoke", func(u *identity.User) { u.AuthVersion++ }},
		{"reset", func(u *identity.User) { u.PasswordHash = string(dummyHash) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.write(func(u ports.Unit) error {
				var user identity.User
				if e := u.Get(ports.Users, f.a.ID, &user); e != nil {
					return e
				}
				user.PasswordHash = legacyFixture
				return u.Save(ports.Users, &user)
			})
			var changed identity.User
			wrapped := &beforeUpgradeDB{Database: f.db, afterRead: func() error {
				return f.db.Write(context.Background(), func(u ports.Unit) error {
					if e := u.Get(ports.Users, f.a.ID, &changed); e != nil {
						return e
					}
					tc.change(&changed)
					return u.Save(ports.Users, &changed)
				})
			}}
			svc := New(wrapped, f.svc.Sessions, nil)
			sessionCount := len(f.svc.Sessions.(*memorySessions).values)
			_, e := svc.Login(context.Background(), &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: "123456"})
			wantCode(t, e, 401)
			if len(f.svc.Sessions.(*memorySessions).values) != sessionCount {
				t.Fatal("session issued after concurrent account change")
			}
			f.read(func(u ports.Unit) error {
				var current identity.User
				if e := u.Get(ports.Users, f.a.ID, &current); e != nil {
					return e
				}
				if current.Status != changed.Status || current.AuthVersion != changed.AuthVersion || current.PasswordHash != changed.PasswordHash {
					t.Fatal("concurrent account change was overwritten")
				}
				return nil
			})
		})
	}
}
