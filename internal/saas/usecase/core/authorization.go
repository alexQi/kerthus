package core

import (
	"context"
	"errors"
	"sort"
	"strings"

	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

type actor struct {
	User   identity.User
	Tenant tenant.Tenant
	Member tenant.Member
	App    catalog.App
}

func (s *Service) authenticate(ctx context.Context, u ports.Unit, req *c.Context) (identity.User, error) {
	if req == nil || len(req.Token) != 64 {
		return identity.User{}, fault.Unauthorized
	}
	if req.TenantID < 0 || req.AppID < 0 || req.UnitID < 0 || req.SectionID < 0 || len(req.RequestID) > 128 {
		return identity.User{}, fault.Invalid("请求上下文不合法")
	}
	session, e := s.Sessions.Get(ctx, req.Token)
	if e != nil {
		return identity.User{}, e
	}
	if session.ExpiresAt <= s.now() {
		return identity.User{}, fault.Unauthorized
	}
	var user identity.User
	if e = u.Get(ports.Users, session.UserID, &user); e != nil {
		if errors.Is(e, fault.NotFound) {
			return user, fault.Unauthorized
		}
		return user, e
	}
	if user.Status != 1 || user.AuthVersion != session.AuthVersion {
		return user, fault.Unauthorized
	}
	return user, nil
}
func (s *Service) resolve(ctx context.Context, u ports.Unit, req *c.Context, needApp bool) (*actor, error) {
	user, e := s.authenticate(ctx, u, req)
	if e != nil {
		return nil, e
	}
	members, e := all[tenant.Member](u, ports.Members, filter("user_id", user.ID, "status", 1))
	if e != nil {
		return nil, e
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].IsDefault && !members[j].IsDefault })
	var a *actor
	for _, m := range members {
		if req.TenantID > 0 && req.TenantID != m.TenantID {
			continue
		}
		var t tenant.Tenant
		if e = u.Get(ports.Tenants, m.TenantID, &t); e != nil {
			return nil, e
		}
		if t.Status != 1 || t.VerifyStatus != 1 || (t.ExpiresAt > 0 && t.ExpiresAt <= s.now()) {
			continue
		}
		a = &actor{User: user, Tenant: t, Member: m}
		break
	}
	if a == nil {
		return nil, fault.Forbidden
	}
	if !needApp {
		return a, nil
	}
	appID := req.AppID
	if appID == 0 {
		appID = a.Member.DefaultAppID
	}
	if appID == 0 {
		return nil, fault.Forbidden
	}
	if e = u.Get(ports.Apps, appID, &a.App); e != nil {
		return nil, e
	}
	if a.App.Status != 1 || catalog.AppType(a.App.Type) == "third" {
		return nil, fault.Forbidden
	}
	if !user.PlatformAdmin {
		ent, e := first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", a.Tenant.ID, "app_id", appID, "status", 1))
		if e != nil {
			if errors.Is(e, fault.NotFound) {
				return nil, fault.Forbidden
			}
			return nil, e
		}
		if ent.ExpiresAt > 0 && ent.ExpiresAt <= s.now() {
			return nil, fault.Forbidden
		}
	}
	if e = s.selectOrganization(u, a, req); e != nil {
		return nil, e
	}
	return a, nil
}

// Stored defaults are preferences only. Always resolve them against current,
// active membership links before returning a context to the caller.
func (s *Service) selectOrganization(u ports.Unit, a *actor, req *c.Context) error {
	links, e := all[organization.MemberOrg](u, ports.MemberOrgs, filter("tenant_id", a.Tenant.ID, "user_id", a.User.ID))
	if e != nil {
		return e
	}
	var selected *organization.Org
	for _, link := range links {
		var org organization.Org
		if e = u.Get(ports.Orgs, link.OrgID, &org); e != nil {
			if errors.Is(e, fault.NotFound) {
				continue
			}
			return e
		}
		if org.TenantID != a.Tenant.ID || org.Status != 1 {
			continue
		}
		if (req.SectionID > 0 && org.ID != req.SectionID) || (req.UnitID > 0 && org.UnitID != req.UnitID) {
			continue
		}
		if selected == nil || org.ID == a.Member.DefaultOrgID {
			copy := org
			selected = &copy
		}
	}
	if selected == nil {
		if req.UnitID > 0 || req.SectionID > 0 {
			return fault.Forbidden
		}
		a.Member.DefaultOrgID = 0
		a.Member.DefaultUnitID = 0
		return nil
	}
	a.Member.DefaultOrgID = selected.ID
	a.Member.DefaultUnitID = selected.UnitID
	return nil
}
func targetTenant(a *actor, target int64) (int64, error) {
	if target == 0 {
		target = a.Tenant.ID
	}
	if target != a.Tenant.ID && !a.User.PlatformAdmin {
		return 0, fault.Forbidden
	}
	return target, nil
}
func requirePlatform(a *actor) error {
	if !a.User.PlatformAdmin {
		return fault.Forbidden
	}
	return nil
}
func (s *Service) effective(u ports.Unit, a *actor) ([]access.Role, []access.Grant, map[int64]catalog.Resource, error) {
	resources, e := all[catalog.Resource](u, ports.Resources, filter("app_id", a.App.ID, "status", 1))
	if e != nil {
		return nil, nil, nil, e
	}
	eligible := map[int64]catalog.Resource{}
	if a.User.PlatformAdmin {
		for _, r := range resources {
			eligible[r.ID] = r
		}
		return nil, nil, eligible, nil
	}
	ent, e := all[catalog.TenantResource](u, ports.TenantResources, filter("tenant_id", a.Tenant.ID, "app_id", a.App.ID))
	if e != nil {
		return nil, nil, nil, e
	}
	entIDs := map[int64]bool{}
	for _, r := range ent {
		entIDs[r.ResourceID] = true
	}
	links, e := all[access.RoleMember](u, ports.RoleMembers, filter("tenant_id", a.Tenant.ID, "user_id", a.User.ID))
	if e != nil {
		return nil, nil, nil, e
	}
	ids := []int64{}
	for _, r := range links {
		ids = append(ids, r.RoleID)
	}
	f := filter("tenant_id", a.Tenant.ID, "status", 1)
	f.In = map[string][]int64{"id": ids}
	roles, e := all[access.Role](u, ports.Roles, f)
	if e != nil {
		return nil, nil, nil, e
	}
	ids = nil
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	f = filter("tenant_id", a.Tenant.ID, "app_id", a.App.ID)
	f.In = map[string][]int64{"role_id": ids}
	grants, e := all[access.Grant](u, ports.Grants, f)
	if e != nil {
		return nil, nil, nil, e
	}
	granted := map[int64]bool{}
	for _, g := range grants {
		granted[g.ResourceID] = true
	}
	for _, r := range resources {
		if entIDs[r.ID] && granted[r.ID] {
			eligible[r.ID] = r
		}
	}
	return roles, grants, eligible, nil
}
func (s *Service) require(u ports.Unit, a *actor, operation string) error {
	if a.User.PlatformAdmin {
		return nil
	}
	ops, e := all[catalog.Operation](u, ports.Operations, filter("app_id", a.App.ID, "action", operation))
	if e != nil {
		return e
	}
	_, _, allowed, e := s.effective(u, a)
	if e != nil {
		return e
	}
	for _, op := range ops {
		if _, ok := allowed[op.ResourceID]; ok {
			return nil
		}
	}
	return fault.Forbidden
}
func RouteMatches(pattern, path string) bool {
	p := strings.Split(pattern, "/")
	v := strings.Split(path, "/")
	if len(p) != len(v) {
		return false
	}
	for i := range p {
		if strings.HasPrefix(p[i], "{") && strings.HasSuffix(p[i], "}") {
			if v[i] == "" || v[i] == "." || v[i] == ".." {
				return false
			}
			continue
		}
		if p[i] != v[i] {
			return false
		}
	}
	return true
}
func (s *Service) CheckAccess(ctx context.Context, req *c.AccessRequest) (*c.AccessReply, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	out := &c.AccessReply{}
	e := s.DB.Read(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		if a.App.Code != req.AppCode {
			return fault.Forbidden
		}
		ops, e := all[catalog.Operation](u, ports.Operations, eq("app_id", a.App.ID))
		if e != nil {
			return e
		}
		var matched *catalog.Operation
		for i := range ops {
			op := &ops[i]
			if op.Method == strings.ToUpper(req.Method) && RouteMatches(op.Path, req.Path) {
				if matched != nil {
					return fault.Conflict("路由匹配存在歧义")
				}
				matched = op
			}
		}
		if matched == nil {
			return fault.Forbidden
		}
		_, grants, allowed, e := s.effective(u, a)
		if e != nil {
			return e
		}
		if _, ok := allowed[matched.ResourceID]; !ok {
			return fault.Forbidden
		}
		scope := access.Scope{All: a.User.PlatformAdmin}
		if !scope.All {
			for _, g := range grants {
				if g.ResourceID == matched.ResourceID {
					v, e := s.scope(u, a, g.DataScope)
					if e != nil {
						return e
					}
					scope = access.Union(scope, v)
				}
			}
		}
		out = &c.AccessReply{UserID: a.User.ID, TenantID: a.Tenant.ID, AppID: a.App.ID, AllWithinTenant: scope.All, UserIDs: scope.UserIDs, ServiceKey: a.App.ServiceKey, RoutePrefix: a.App.RoutePrefix}
		return nil
	})
	return out, e
}
func (s *Service) scope(u ports.Unit, a *actor, code int32) (access.Scope, error) {
	if code == 0 {
		return access.Scope{All: true}, nil
	}
	if code == 5 {
		return access.Scope{UserIDs: []int64{a.User.ID}}, nil
	}
	if code < 0 || code > 5 {
		return access.Scope{}, fault.Invalid("无效数据范围")
	}
	orgs, e := all[organization.Org](u, ports.Orgs, filter("tenant_id", a.Tenant.ID, "status", 1))
	if e != nil {
		return access.Scope{}, e
	}
	links, e := all[organization.MemberOrg](u, ports.MemberOrgs, eq("tenant_id", a.Tenant.ID))
	if e != nil {
		return access.Scope{}, e
	}
	allowed := map[int64]bool{}
	for _, link := range links {
		if link.UserID == a.User.ID {
			for _, org := range orgs {
				if org.ID == link.OrgID {
					if code == 1 || code == 2 {
						allowed[org.UnitID] = true
					} else {
						allowed[org.ID] = true
					}
				}
			}
		}
	}
	if code == 1 {
		// Preserve Beehive's unit_id boundary: departments in the same unit
		// are included, but independently owned child units are not.
		units := map[int64]bool{}
		for id := range allowed {
			units[id] = true
		}
		for _, org := range orgs {
			if units[org.UnitID] {
				allowed[org.ID] = true
			}
		}
	}
	if code == 3 {
		changed := true
		for changed {
			changed = false
			for _, org := range orgs {
				if allowed[org.ParentID] && !allowed[org.ID] {
					allowed[org.ID] = true
					changed = true
				}
			}
		}
	}
	// Legacy scope 2 means direct members of the unit node. Expanding to
	// department nodes here silently broadens existing "本单位" grants.
	members, e := all[tenant.Member](u, ports.Members, filter("tenant_id", a.Tenant.ID, "status", 1))
	if e != nil {
		return access.Scope{}, e
	}
	valid := map[int64]bool{}
	for _, m := range members {
		valid[m.UserID] = true
	}
	out := access.Scope{}
	for _, link := range links {
		if allowed[link.OrgID] && valid[link.UserID] {
			out.UserIDs = append(out.UserIDs, link.UserID)
		}
	}
	return access.Union(out), nil
}
