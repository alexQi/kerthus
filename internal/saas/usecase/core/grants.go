package core

import (
	"context"
	"errors"
	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

func (s *Service) grantApplication(u ports.Unit, tid int64, g *c.AppGrant) error {
	if g == nil || g.ExpiresAt < 0 {
		return fault.Invalid("应用开通参数不正确")
	}
	var app catalog.App
	if e := u.Get(ports.Apps, g.AppID, &app); e != nil {
		return e
	}
	if app.Status != 1 {
		return fault.Invalid("应用已停用")
	}
	if app.Code == "system" {
		return fault.Invalid("平台管理应用不能向普通租户开通")
	}
	var t tenant.Tenant
	if e := u.Get(ports.Tenants, tid, &t); e != nil {
		return e
	}
	unique := map[int64]bool{}
	for _, id := range g.ResourceIDs {
		var r catalog.Resource
		if e := u.Get(ports.Resources, id, &r); e != nil {
			return e
		}
		if r.AppID != app.ID || r.Status != 1 {
			return fault.Invalid("授权资源不属于该应用或已停用")
		}
		unique[id] = true
	}
	ent, e := first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", tid, "app_id", app.ID))
	if e != nil && !errors.Is(e, fault.NotFound) {
		return e
	}
	if errors.Is(e, fault.NotFound) {
		ent = catalog.Entitlement{TenantID: tid, AppID: app.ID}
	}
	ent.Status = 1
	ent.ExpiresAt = g.ExpiresAt
	if e = u.Save(ports.Entitlements, &ent); e != nil {
		return e
	}
	if e = u.Delete(ports.TenantResources, filter("tenant_id", tid, "app_id", app.ID)); e != nil {
		return e
	}
	for id := range unique {
		if e = u.Save(ports.TenantResources, &catalog.TenantResource{TenantID: tid, AppID: app.ID, ResourceID: id}); e != nil {
			return e
		}
	}
	old, e := all[access.Grant](u, ports.Grants, filter("tenant_id", tid, "app_id", app.ID))
	if e != nil {
		return e
	}
	for _, grant := range old {
		if !unique[grant.ResourceID] {
			if e = u.Delete(ports.Grants, eq("id", grant.ID)); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Service) SetEntitlements(ctx context.Context, req *c.EntitlementsRequest) (*c.Result, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, false)
		if e != nil {
			return e
		}
		if e = requirePlatform(a); e != nil {
			return e
		}
		if len(req.TenantIDs)+len(req.RevokeIDs) == 0 {
			return fault.Invalid("请选择租户或开通记录")
		}
		for _, id := range req.RevokeIDs {
			ent, e := first[catalog.Entitlement](u, ports.Entitlements, eq("id", id))
			if e != nil {
				return e
			}
			for _, table := range []ports.Table{ports.Grants, ports.TenantResources} {
				if e = u.Delete(table, filter("tenant_id", ent.TenantID, "app_id", ent.AppID)); e != nil {
					return e
				}
			}
			if e = u.Delete(ports.Entitlements, eq("id", ent.ID)); e != nil {
				return e
			}
			members, e := all[tenant.Member](u, ports.Members, filter("tenant_id", ent.TenantID, "default_app_id", ent.AppID))
			if e != nil {
				return e
			}
			for _, m := range members {
				m.DefaultAppID = 0
				if e = u.Save(ports.Members, &m); e != nil {
					return e
				}
			}
			if e = s.record(u, a, "catalog.revoke", ent.ID, req.Context); e != nil {
				return e
			}
		}
		seen := map[int64]bool{}
		for _, tid := range req.TenantIDs {
			if seen[tid] {
				continue
			}
			seen[tid] = true
			for _, g := range req.Apps {
				if e = s.grantApplication(u, tid, g); e != nil {
					return e
				}
			}
			if e = s.record(u, a, "catalog.grant", tid, req.Context); e != nil {
				return e
			}
		}
		return nil
	})
	return &c.Result{Success: e == nil}, e
}
func (s *Service) SetRoleResources(ctx context.Context, req *c.RoleResourcesRequest) (*c.Result, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		if e = s.require(u, a, "access.write"); e != nil {
			return e
		}
		tid, e := targetTenant(a, req.TenantID)
		if e != nil {
			return e
		}
		var role access.Role
		if e = u.Get(ports.Roles, req.RoleID, &role); e != nil {
			return e
		}
		if role.TenantID != tid {
			return fault.Forbidden
		}
		if role.Administrator {
			return fault.Conflict("基础管理员角色由开户模板维护，请新建业务角色")
		}
		allowed := map[int64]map[int64]bool{}
		apps, e := s.availableApps(u, tid, false)
		if e != nil {
			return e
		}
		for _, app := range apps {
			rs, e := all[catalog.TenantResource](u, ports.TenantResources, filter("tenant_id", tid, "app_id", app.ID))
			if e != nil {
				return e
			}
			allowed[app.ID] = map[int64]bool{}
			for _, r := range rs {
				var resource catalog.Resource
				if e = u.Get(ports.Resources, r.ResourceID, &resource); e != nil {
					return e
				}
				if resource.Status == 1 && resource.AppID == app.ID {
					allowed[app.ID][r.ResourceID] = true
				}
			}
		}
		for _, g := range req.Grants {
			if g == nil || !allowed[g.AppID][g.ResourceID] || g.DataScope < 0 || g.DataScope > 5 {
				return fault.Invalid("角色授权不能超出租户的有效应用资源")
			}
		}
		if e = u.Delete(ports.Grants, filter("tenant_id", tid, "role_id", role.ID)); e != nil {
			return e
		}
		seen := map[int64]bool{}
		for _, g := range req.Grants {
			if seen[g.ResourceID] {
				return fault.Invalid("资源授权重复")
			}
			seen[g.ResourceID] = true
			if e = u.Save(ports.Grants, &access.Grant{TenantID: tid, RoleID: role.ID, AppID: g.AppID, ResourceID: g.ResourceID, DataScope: g.DataScope}); e != nil {
				return e
			}
		}
		return s.record(u, a, "access.resources.replace", role.ID, req.Context)
	})
	return &c.Result{Success: e == nil}, e
}
func (s *Service) SetRoleMembers(ctx context.Context, req *c.RoleMembersRequest) (*c.Result, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		if e = s.require(u, a, "access.write"); e != nil {
			return e
		}
		tid, e := targetTenant(a, req.TenantID)
		if e != nil {
			return e
		}
		var role access.Role
		if e = u.Get(ports.Roles, req.RoleID, &role); e != nil {
			return e
		}
		if role.TenantID != tid {
			return fault.Forbidden
		}
		for _, id := range req.UserIDs {
			if _, e = first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", id, "status", 1)); e != nil {
				return fault.Invalid("角色成员必须属于当前租户且有效")
			}
			f := filter("tenant_id", tid, "role_id", role.ID, "user_id", id)
			if req.Remove {
				if role.Administrator {
					if e = s.protectAdmin(u, tid, id, 0); e != nil {
						return e
					}
				}
				if e = u.Delete(ports.RoleMembers, f); e != nil {
					return e
				}
			} else {
				n, e := u.Count(ports.RoleMembers, f)
				if e != nil {
					return e
				}
				if n == 0 {
					if e = u.Save(ports.RoleMembers, &access.RoleMember{TenantID: tid, RoleID: role.ID, UserID: id}); e != nil {
						return e
					}
				}
			}
		}
		return s.record(u, a, "access.members.change", role.ID, req.Context)
	})
	return &c.Result{Success: e == nil}, e
}
