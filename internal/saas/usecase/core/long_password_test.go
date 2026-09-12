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

// Public synthetic fixtures independently calculated from the original PHP
// md5(sha1(password) . md5(password)) byte algorithm; never real credentials.
var longLegacyPassword = "legacy-" + strings.Repeat("x", 80) + "-正确后缀"

const longLegacyFixture = LegacyPasswordPrefix + "378f146e5b8c4ee03f6e2fd4f2183373"

func TestLongLegacyPasswordFullByteCompatibility(t *testing.T) {
	for _, tc := range []struct{ password, stored string }{
		{longLegacyPassword, longLegacyFixture},
		{strings.Repeat("长", 30) + "a", LegacyPasswordPrefix + "13fe36209adb351078095487ebc0f82a"},
		{"nul-\x00" + strings.Repeat("x", 80), LegacyPasswordPrefix + "5595e72a10332dbd73d80bd8dd107f41"},
	} {
		if !verifyPassword(tc.stored, tc.password) {
			t.Fatal("original PHP long-password fixture rejected")
		}
		upgraded, err := upgradeLegacyPassword(tc.password)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(upgraded), longPasswordPrefix+longPasswordParameters) {
			t.Fatal("missing explicit long-password algorithm/version")
		}
		if !verifyPassword(string(upgraded), tc.password) {
			t.Fatal("upgraded password rejected")
		}
		for _, invalid := range []string{tc.password[:72], tc.password + "different-suffix", tc.password[:len(tc.password)-1] + "!"} {
			if verifyPassword(tc.stored, invalid) || verifyPassword(string(upgraded), invalid) {
				t.Fatal("password suffix ignored before or after upgrade")
			}
		}
		another, err := upgradeLegacyPassword(tc.password)
		if err != nil {
			t.Fatal(err)
		}
		if string(another) == string(upgraded) {
			t.Fatal("upgrades did not use fresh random salts")
		}
	}
	plain, err := bcrypt.GenerateFromPassword([]byte(longLegacyPassword[:72]), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if verifyPassword(string(plain), longLegacyPassword) {
		t.Fatal("plain bcrypt accepted an extended password")
	}
	if !verifyPassword(string(plain), longLegacyPassword[:72]) {
		t.Fatal("ordinary bcrypt compatibility lost")
	}
}

func TestLongPasswordFormatRejectsMalformedOrWrongVersions(t *testing.T) {
	good, err := upgradeLegacyPassword(longLegacyPassword)
	if err != nil {
		t.Fatal(err)
	}
	stored := string(good)
	for _, bad := range []string{
		strings.TrimPrefix(stored, longPasswordPrefix),
		strings.Replace(stored, "argon2id", "argon2i", 1),
		strings.Replace(stored, "v=19", "v=20", 1),
		strings.Replace(stored, "m=19456", "m=999999999", 1),
		strings.Replace(stored, "t=2", "t=2000000000", 1),
		strings.Replace(stored, "p=1", "p=0", 1),
		stored + "$extra", stored[:len(stored)-1],
		longPasswordPrefix + longPasswordParameters + "bad-salt$bad-key",
		longPasswordPrefix + string(dummyHash),
		LegacyPasswordPrefix + stored,
	} {
		if verifyPassword(bad, longLegacyPassword) {
			t.Fatal("malformed or unknown algorithm version accepted")
		}
	}
	if verifyPassword(stored, "") {
		t.Fatal("empty password accepted")
	}
	if _, err := passwordHash(longLegacyPassword); err == nil {
		t.Fatal("new-password policy silently expanded")
	}
}

func TestMySQLLongLegacyLoginUpgradePreservesSessionPolicy(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "multiple", true: "single"}[single], func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.write(func(u ports.Unit) error {
				f.a.PasswordHash = longLegacyFixture
				f.a.SingleLogin = single
				f.a.Email = "long-login@example.invalid"
				return u.Save(ports.Users, &f.a)
			})
			_, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: longLegacyPassword + "!"})
			wantCode(t, err, 401)
			firstLogin, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: longLegacyPassword})
			if err != nil {
				t.Fatal(err)
			}
			var upgraded identity.User
			f.read(func(u ports.Unit) error { return u.Get(ports.Users, f.a.ID, &upgraded) })
			if !strings.HasPrefix(upgraded.PasswordHash, longPasswordPrefix) {
				t.Fatal("successful legacy login did not upgrade")
			}
			_, err = f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: f.a.Email, Password: longLegacyPassword[:72]})
			wantCode(t, err, 401)
			secondLogin, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: f.a.Email, Password: longLegacyPassword})
			if err != nil {
				t.Fatal(err)
			}
			firstContext := &c.Context{Token: firstLogin.AccessToken, TenantID: firstLogin.TenantID}
			_, err = f.svc.Profile(ctx, firstContext)
			if single {
				wantCode(t, err, 401)
			} else if err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.Profile(ctx, &c.Context{Token: secondLogin.AccessToken, TenantID: secondLogin.TenantID}); err != nil {
				t.Fatal(err)
			}
			f.read(func(u ports.Unit) error {
				var current identity.User
				if err := u.Get(ports.Users, f.a.ID, &current); err != nil {
					return err
				}
				if current.PasswordHash != upgraded.PasswordHash || current.SingleLogin != single {
					t.Fatal("subsequent login changed upgraded hash or policy")
				}
				return nil
			})
		})
	}
}

func TestMySQLLongOldPasswordForEmailAndPasswordChange(t *testing.T) {
	for _, upgraded := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "argon2id"}[upgraded], func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			hash := longLegacyFixture
			if upgraded {
				encoded, err := upgradeLegacyPassword(longLegacyPassword)
				if err != nil {
					t.Fatal(err)
				}
				hash = string(encoded)
			}
			f.write(func(u ports.Unit) error {
				f.a.PasswordHash = hash
				f.shared.PasswordHash = hash
				if err := u.Save(ports.Users, &f.shared); err != nil {
					return err
				}
				return u.Save(ports.Users, &f.a)
			})
			// ChangePassword must also accept a legacy long credential before
			// that account has ever logged in or triggered automatic upgrade.
			if _, err := f.svc.ChangePassword(ctx, &c.PasswordRequest{Context: f.sc, OldPassword: longLegacyPassword, NewPassword: "direct-replacement-password"}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.shared.Phone, Password: "direct-replacement-password"}); err != nil {
				t.Fatal(err)
			}

			request := &c.SaveRequest{Context: f.ac, Password: longLegacyPassword + "!", User: &c.User{ID: f.a.ID, Name: f.a.Name, Email: "long-password-change@example.invalid"}}
			_, err := f.svc.Save(ctx, request)
			wantCode(t, err, 400)
			request.Password = longLegacyPassword
			if _, err = f.svc.Save(ctx, request); err != nil {
				t.Fatalf("long current password could not verify email change: %v", err)
			}
			login, err := f.svc.Login(ctx, &c.LoginRequest{Scene: "email", Username: request.User.Email, Password: longLegacyPassword})
			if err != nil {
				t.Fatal(err)
			}
			actor := &c.Context{Token: login.AccessToken, TenantID: login.TenantID}
			_, err = f.svc.ChangePassword(ctx, &c.PasswordRequest{Context: actor, OldPassword: longLegacyPassword + "!", NewPassword: "replacement-password"})
			wantCode(t, err, 400)
			if _, err = f.svc.ChangePassword(ctx, &c.PasswordRequest{Context: actor, OldPassword: longLegacyPassword, NewPassword: "replacement-password"}); err != nil {
				t.Fatal(err)
			}
			_, err = f.svc.Profile(ctx, actor)
			wantCode(t, err, 401)
			if _, err = f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: "replacement-password"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLLongUpgradeCannotOverwriteConcurrentAccountChanges(t *testing.T) {
	for _, change := range []string{"reset", "revoke", "disable", "single-login"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.write(func(u ports.Unit) error {
				f.a.PasswordHash = longLegacyFixture
				f.a.SingleLogin = true
				return u.Save(ports.Users, &f.a)
			})
			var winner *c.LoginReply
			wrapped := &beforeUpgradeDB{Database: f.db, afterRead: func() error {
				if change == "single-login" {
					var err error
					winner, err = f.svc.Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: longLegacyPassword})
					return err
				}
				return f.db.Write(ctx, func(u ports.Unit) error {
					var user identity.User
					if err := u.Get(ports.Users, f.a.ID, &user); err != nil {
						return err
					}
					switch change {
					case "reset":
						user.PasswordHash = string(dummyHash)
						user.AuthVersion++
					case "revoke":
						user.AuthVersion++
					case "disable":
						user.Status = 0
					}
					return u.Save(ports.Users, &user)
				})
			}}
			beforeSessions := len(f.svc.Sessions.(*memorySessions).values)
			_, err := New(wrapped, f.svc.Sessions, nil).Login(ctx, &c.LoginRequest{Scene: "phone", Username: f.a.Phone, Password: longLegacyPassword})
			wantCode(t, err, 401)
			f.read(func(u ports.Unit) error {
				var user identity.User
				if err := u.Get(ports.Users, f.a.ID, &user); err != nil {
					return err
				}
				switch change {
				case "reset":
					if user.PasswordHash != string(dummyHash) || user.AuthVersion != f.a.AuthVersion+1 {
						t.Fatal("concurrent reset overwritten")
					}
				case "revoke":
					if user.AuthVersion != f.a.AuthVersion+1 || user.PasswordHash != longLegacyFixture {
						t.Fatal("concurrent revocation overwritten")
					}
				case "disable":
					if user.Status != 0 || user.PasswordHash != longLegacyFixture {
						t.Fatal("disabled account upgraded")
					}
				}
				return nil
			})
			expectedSessions := beforeSessions
			if winner != nil {
				expectedSessions++
			}
			if len(f.svc.Sessions.(*memorySessions).values) != expectedSessions {
				t.Fatal("stale upgrade attempt issued a session")
			}
			if winner != nil {
				if _, err = f.svc.Profile(ctx, &c.Context{Token: winner.AccessToken, TenantID: winner.TenantID}); err != nil {
					t.Fatal("winning concurrent long-password login invalidated")
				}
			}
		})
	}
}
