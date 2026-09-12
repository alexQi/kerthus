package core

import (
	"context"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("kerthus-timing-comparison-only"), bcrypt.DefaultCost)

func (s *Service) Login(ctx context.Context, req *c.LoginRequest) (*c.LoginReply, error) {
	if req == nil || len(req.Username) > 191 || (req.Scene != "phone" && req.Scene != "email") {
		return nil, fault.Invalid("登录参数不正确")
	}
	out := &c.LoginReply{}
	var session identity.Session
	var authenticated identity.User
	e := s.DB.Read(ctx, func(u ports.Unit) error {
		f := eq(req.Scene, strings.TrimSpace(req.Username))
		f.PageSize = 2
		users, e := all[identity.User](u, ports.Users, f)
		if e != nil {
			return e
		}
		// Imported email addresses are not necessarily unique. Never select an
		// arbitrary account, even when only one duplicate account is enabled.
		if len(users) != 1 {
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
			return fault.New(401, "账号或密码不正确，请使用手机号登录")
		}
		user := users[0]
		if !verifyPassword(user.PasswordHash, req.Password) || user.Status != 1 {
			return fault.New(401, "账号或密码不正确")
		}
		members, e := all[tenant.Member](u, ports.Members, filter("user_id", user.ID, "status", 1))
		if e != nil {
			return e
		}
		sort.SliceStable(members, func(i, j int) bool { return members[i].IsDefault && !members[j].IsDefault })
		for _, m := range members {
			var t tenant.Tenant
			if e = u.Get(ports.Tenants, m.TenantID, &t); e != nil {
				return e
			}
			if t.Status != 1 || t.VerifyStatus != 1 || (t.ExpiresAt > 0 && t.ExpiresAt <= s.now()) {
				continue
			}
			apps, e := s.availableApps(u, t.ID, user.PlatformAdmin)
			if e != nil {
				return e
			}
			localApps := apps[:0]
			for _, candidate := range apps {
				if catalog.AppType(candidate.Type) != "third" {
					localApps = append(localApps, candidate)
				}
			}
			apps = localApps
			if len(apps) == 0 {
				continue
			}
			app := apps[0]
			for _, a := range apps {
				if a.ID == m.DefaultAppID {
					app = a
					break
				}
			}
			a := &actor{User: user, Tenant: t, Member: m, App: app}
			if e = s.selectOrganization(u, a, &c.Context{}); e != nil {
				return e
			}
			out = &c.LoginReply{UserID: user.ID, TenantID: t.ID, AppID: app.ID, AppCode: app.Code, Home: app.Home, UnitID: a.Member.DefaultUnitID, SectionID: a.Member.DefaultOrgID, ExpiresTime: s.now() + int64((12 * time.Hour).Seconds())}
			session = identity.Session{UserID: user.ID, AuthVersion: user.AuthVersion, ExpiresAt: out.ExpiresTime}
			authenticated = user
			return nil
		}
		return fault.New(403, "没有有效的租户或已开通应用")
	})
	if e != nil {
		return nil, e
	}
	if e = s.finalizeLogin(ctx, &authenticated, req.Password); e != nil {
		return nil, e
	}
	session.AuthVersion = authenticated.AuthVersion
	out.AccessToken, e = randomToken()
	if e != nil {
		return nil, e
	}
	if e = s.Sessions.Put(ctx, out.AccessToken, session, 12*time.Hour); e != nil {
		return nil, e
	}
	return out, nil
}
func (s *Service) Logout(ctx context.Context, req *c.Context) (*c.Result, error) {
	if req == nil || req.Token == "" {
		return nil, fault.Unauthorized
	}
	if e := s.Sessions.Delete(ctx, req.Token); e != nil {
		return nil, e
	}
	return &c.Result{Success: true}, nil
}
func (s *Service) Profile(ctx context.Context, req *c.Context) (*c.User, error) {
	var out *c.User
	e := s.DB.Read(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req, false)
		if e != nil {
			return e
		}
		out = userDTO(a.User)
		return nil
	})
	return out, e
}
func (s *Service) availableApps(u ports.Unit, tenantID int64, platform bool) ([]catalog.App, error) {
	apps, e := all[catalog.App](u, ports.Apps, eq("status", 1))
	if e != nil {
		return nil, e
	}
	if platform {
		return apps, nil
	}
	ents, e := all[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", tenantID, "status", 1))
	if e != nil {
		return nil, e
	}
	ids := map[int64]bool{}
	for _, ent := range ents {
		if ent.ExpiresAt == 0 || ent.ExpiresAt > s.now() {
			ids[ent.AppID] = true
		}
	}
	out := []catalog.App{}
	for _, a := range apps {
		if ids[a.ID] {
			out = append(out, a)
		}
	}
	return out, nil
}
func (s *Service) Auth(ctx context.Context, req *c.Context) (*c.AuthReply, error) {
	out := &c.AuthReply{}
	e := s.DB.Read(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req, true)
		if e != nil {
			return e
		}
		roles, _, allowed, e := s.effective(u, a)
		if e != nil {
			return e
		}
		out.PlatformAdmin = a.User.PlatformAdmin
		if a.User.PlatformAdmin {
			out.Roles = []string{"admin"}
		} else {
			for _, r := range roles {
				code := r.Code
				// The reused console reserves admin for platform-wide UI access.
				if code == "admin" {
					code = "tenant:admin"
				}
				out.Roles = append(out.Roles, code)
			}
		}
		for _, r := range allowed {
			out.Permissions = append(out.Permissions, r.Code)
		}
		sort.Strings(out.Permissions)
		allResources, e := all[catalog.Resource](u, ports.Resources, filter("app_id", a.App.ID, "status", 1))
		if e != nil {
			return e
		}
		byID := map[int64]catalog.Resource{}
		for _, r := range allResources {
			byID[r.ID] = r
		}
		navigation := map[int64]catalog.Resource{}
		for _, r := range allowed {
			navigation[r.ID] = r
			visited := map[int64]bool{r.ID: true}
			parent := r.ParentID
			for parent > 0 && !visited[parent] {
				visited[parent] = true
				p, ok := byID[parent]
				if !ok {
					break
				}
				navigation[p.ID] = p
				parent = p.ParentID
			}
		}
		for _, r := range navigation {
			out.Resources = append(out.Resources, resourceDTO(r))
		}
		sort.Slice(out.Resources, func(i, j int) bool {
			if out.Resources[i].Sort == out.Resources[j].Sort {
				return out.Resources[i].ID < out.Resources[j].ID
			}
			return out.Resources[i].Sort < out.Resources[j].Sort
		})
		out.Context = &c.LoginReply{UserID: a.User.ID, TenantID: a.Tenant.ID, AppID: a.App.ID, AppCode: a.App.Code, Home: a.App.Home, UnitID: a.Member.DefaultUnitID, SectionID: a.Member.DefaultOrgID}
		return nil
	})
	return out, e
}
func (s *Service) ChangePassword(ctx context.Context, req *c.PasswordRequest) (*c.Result, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	hash, e := passwordHash(req.NewPassword)
	if e != nil {
		return nil, e
	}
	e = s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, false)
		if e != nil {
			return e
		}
		id := req.UserID
		if id == 0 {
			id = a.User.ID
		}
		var user identity.User
		if e = u.Get(ports.Users, id, &user); e != nil {
			return e
		}
		if id == a.User.ID {
			if !verifyPassword(user.PasswordHash, req.OldPassword) {
				return fault.Invalid("原密码不正确")
			}
		} else if e = requirePlatform(a); e != nil {
			return e
		}
		user.PasswordHash = hash
		user.AuthVersion++
		if e = u.Save(ports.Users, &user); e != nil {
			return e
		}
		return s.record(u, a, "identity.password.change", id, req.Context)
	})
	return &c.Result{Success: e == nil}, e
}
