package core

import (
	"context"
	"errors"
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

func (s *Service) ApproveTenant(ctx context.Context, req *c.ApproveRequest) (*c.Result, error) {
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
		var t tenant.Tenant
		if e = u.Get(ports.Tenants, req.TenantID, &t); e != nil {
			return e
		}
		if !req.Approved {
			if t.BootstrapVersion > 0 {
				return fault.Conflict("已开通租户请使用停用操作")
			}
			t.VerifyStatus = 2
			if e = u.Save(ports.Tenants, &t); e != nil {
				return e
			}
			return s.record(u, a, "tenant.reject", t.ID, req.Context)
		}
		if t.BootstrapVersion > 0 {
			return nil
		}
		if t.Status != 1 {
			return fault.Invalid("租户已停用")
		}
		if t.ContactPhone == "" {
			return fault.Invalid("开户需要联系人手机号")
		}
		user, e := first[identity.User](u, ports.Users, eq("phone", t.ContactPhone))
		if e != nil && !errors.Is(e, fault.NotFound) {
			return e
		}
		if errors.Is(e, fault.NotFound) {
			hash, e := passwordHash(req.AdminPassword)
			if e != nil {
				return fault.Invalid("新管理员开户需要通过 POST 提供10至72字节初始密码")
			}
			if e := ensureAvailableEmail(u, strings.TrimSpace(t.ContactEmail), identity.User{}); e != nil {
				return e
			}
			user = identity.User{Phone: t.ContactPhone, Email: strings.TrimSpace(t.ContactEmail), Name: t.ContactPerson, Status: 1, PasswordHash: hash, SingleLogin: true, AuthVersion: 1}
			if user.Name == "" {
				user.Name = t.Name + "管理员"
			}
			if e = u.Save(ports.Users, &user); e != nil {
				return e
			}
		}
		if user.Status != 1 {
			return fault.Invalid("联系人账号已停用")
		}
		app, e := first[catalog.App](u, ports.Apps, eq("code", "basic"))
		if e != nil {
			return e
		}
		org := organization.Org{TenantID: t.ID, Type: "unit", Name: t.Name, ShortName: t.Name, Status: 1}
		if e = u.Save(ports.Orgs, &org); e != nil {
			return e
		}
		org.UnitID = org.ID
		if e = u.Save(ports.Orgs, &org); e != nil {
			return e
		}
		position := organization.Position{TenantID: t.ID, OrgID: org.ID, Name: "管理员", Status: 1}
		if e = u.Save(ports.Positions, &position); e != nil {
			return e
		}
		m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", t.ID, "user_id", user.ID))
		if e != nil && !errors.Is(e, fault.NotFound) {
			return e
		}
		if errors.Is(e, fault.NotFound) {
			n, e := u.Count(ports.Members, eq("user_id", user.ID))
			if e != nil {
				return e
			}
			m = tenant.Member{TenantID: t.ID, UserID: user.ID, IsDefault: n == 0}
		}
		m.Status = 1
		m.DefaultAppID = app.ID
		m.DefaultUnitID = org.ID
		m.DefaultOrgID = org.ID
		if e = u.Save(ports.Members, &m); e != nil {
			return e
		}
		if e = u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: t.ID, UserID: user.ID, OrgID: org.ID}); e != nil {
			return e
		}
		if e = u.Save(ports.MemberPositions, &organization.MemberPosition{TenantID: t.ID, UserID: user.ID, PositionID: position.ID}); e != nil {
			return e
		}
		role := access.Role{TenantID: t.ID, Code: "tenant-admin", Name: "租户管理员", Status: 1, Administrator: true}
		if e = u.Save(ports.Roles, &role); e != nil {
			return e
		}
		if e = u.Save(ports.RoleMembers, &access.RoleMember{TenantID: t.ID, RoleID: role.ID, UserID: user.ID}); e != nil {
			return e
		}
		resources, e := all[catalog.Resource](u, ports.Resources, filter("app_id", app.ID, "status", 1))
		if e != nil {
			return e
		}
		ids := []int64{}
		for _, r := range resources {
			ids = append(ids, r.ID)
		}
		if e = s.grantApplication(u, t.ID, &c.AppGrant{AppID: app.ID, ResourceIDs: ids}); e != nil {
			return e
		}
		for _, r := range resources {
			if e = u.Save(ports.Grants, &access.Grant{TenantID: t.ID, RoleID: role.ID, AppID: app.ID, ResourceID: r.ID, DataScope: 0}); e != nil {
				return e
			}
		}
		t.VerifyStatus = 1
		t.BootstrapVersion = 1
		if e = u.Save(ports.Tenants, &t); e != nil {
			return e
		}
		return s.record(u, a, "tenant.approve", t.ID, req.Context)
	})
	return &c.Result{Success: e == nil}, e
}
func (s *Service) protectAdmin(u ports.Unit, tid, userID, roleID int64) error {
	roles, e := all[access.Role](u, ports.Roles, filter("tenant_id", tid, "administrator", true, "status", 1))
	if e != nil {
		return e
	}
	remaining := map[int64]bool{}
	affected := false
	for _, r := range roles {
		links, e := all[access.RoleMember](u, ports.RoleMembers, filter("tenant_id", tid, "role_id", r.ID))
		if e != nil {
			return e
		}
		for _, link := range links {
			if (roleID > 0 && r.ID == roleID) || (userID > 0 && link.UserID == userID) {
				affected = true
				continue
			}
			m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", link.UserID, "status", 1))
			if errors.Is(e, fault.NotFound) {
				continue
			}
			if e != nil {
				return e
			}
			var user identity.User
			if e = u.Get(ports.Users, m.UserID, &user); e != nil {
				return e
			}
			if user.Status == 1 {
				remaining[user.ID] = true
			}
		}
	}
	if affected && len(remaining) == 0 {
		return fault.Conflict("至少保留一名有效租户管理员")
	}
	return nil
}
func (s *Service) SetStatus(ctx context.Context, req *c.StatusRequest) (*c.Result, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	if e := validStatus(req.Status); e != nil {
		return nil, e
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		tid, e := targetTenant(a, req.TargetTenantID)
		if e != nil {
			return e
		}
		switch req.Kind {
		case c.KindUsers:
			if e = requirePlatform(a); e != nil {
				return e
			}
			var user identity.User
			if e = u.Get(ports.Users, req.ID, &user); e != nil {
				return e
			}
			if user.PlatformAdmin && req.Status == 0 {
				n, e := u.Count(ports.Users, filter("platform_admin", true, "status", 1))
				if e != nil {
					return e
				}
				if n <= 1 {
					return fault.Conflict("至少保留一名平台管理员")
				}
			}
			if req.Status == 0 {
				members, e := all[tenant.Member](u, ports.Members, eq("user_id", user.ID))
				if e != nil {
					return e
				}
				for _, m := range members {
					if e = s.protectAdmin(u, m.TenantID, user.ID, 0); e != nil {
						return e
					}
				}
			}
			if user.Status != req.Status {
				user.Status = req.Status
				user.AuthVersion++
			}
			if e = u.Save(ports.Users, &user); e != nil {
				return e
			}
		case c.KindTenants:
			if e = requirePlatform(a); e != nil {
				return e
			}
			var t tenant.Tenant
			if e = u.Get(ports.Tenants, req.ID, &t); e != nil {
				return e
			}
			if t.ID == a.Tenant.ID && req.Status == 0 {
				return fault.Conflict("不能停用当前平台租户")
			}
			t.Status = req.Status
			if e = u.Save(ports.Tenants, &t); e != nil {
				return e
			}
		case c.KindMembers:
			if e = s.require(u, a, "membership.write"); e != nil {
				return e
			}
			m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", req.ID))
			if e != nil {
				return e
			}
			if req.Status == 0 {
				if e = s.protectAdmin(u, tid, m.UserID, 0); e != nil {
					return e
				}
			}
			m.Status = req.Status
			if e = u.Save(ports.Members, &m); e != nil {
				return e
			}
		case c.KindRoles:
			if e = s.require(u, a, "access.write"); e != nil {
				return e
			}
			var r access.Role
			if e = u.Get(ports.Roles, req.ID, &r); e != nil {
				return e
			}
			if r.TenantID != tid {
				return fault.Forbidden
			}
			if req.Status == 0 {
				if e = s.protectAdmin(u, tid, 0, r.ID); e != nil {
					return e
				}
			}
			r.Status = req.Status
			if e = u.Save(ports.Roles, &r); e != nil {
				return e
			}
		case c.KindPositions:
			if e = s.require(u, a, "organization.write"); e != nil {
				return e
			}
			var p organization.Position
			if e = u.Get(ports.Positions, req.ID, &p); e != nil {
				return e
			}
			if p.TenantID != tid {
				return fault.Forbidden
			}
			if p.Status == 1 && req.Status == 0 {
				n, e := u.Count(ports.MemberPositions, filter("tenant_id", tid, "position_id", p.ID))
				if e != nil {
					return e
				}
				if n > 0 {
					return fault.Conflict("岗位仍有成员，不能停用")
				}
			}
			p.Status = req.Status
			if e = u.Save(ports.Positions, &p); e != nil {
				return e
			}
		case c.KindApps:
			if e = requirePlatform(a); e != nil {
				return e
			}
			var app catalog.App
			if e = u.Get(ports.Apps, req.ID, &app); e != nil {
				return e
			}
			if app.Code == "basic" || app.Code == "system" {
				return fault.Conflict("基础平台应用不能停用")
			}
			if req.Status == 1 {
				if catalog.AppType(app.Type) == "third" {
					if !catalog.SafeWebURL(app.URL) || app.Managed || app.ServiceKey != "" || app.RoutePrefix != "" {
						return fault.Conflict("第三方应用需要有效地址且不能绑定后端服务")
					}
				} else if !app.Managed || app.ServiceKey == "" || app.RoutePrefix == "" || app.Home == "" {
					return fault.Conflict("应用需先通过受控定义注册服务、路由和首页")
				}
			}
			app.Status = req.Status
			if e = u.Save(ports.Apps, &app); e != nil {
				return e
			}
		default:
			return fault.Invalid("不支持的状态操作")
		}
		action := auditEntity(req.Kind) + ".disable"
		if req.Status == 1 {
			action = auditEntity(req.Kind) + ".enable"
		}
		return s.record(u, a, action, req.ID, req.Context)
	})
	return &c.Result{Success: e == nil}, e
}
func (s *Service) Delete(ctx context.Context, req *c.DeleteRequest) (*c.Result, error) {
	if req == nil {
		return nil, fault.Invalid("缺少请求参数")
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, true)
		if e != nil {
			return e
		}
		tid, e := targetTenant(a, req.TargetTenantID)
		if e != nil {
			return e
		}
		f := filter("tenant_id", tid, "id", req.ID)
		switch req.Kind {
		case c.KindTenants:
			if e = requirePlatform(a); e != nil {
				return e
			}
			var t tenant.Tenant
			if e = u.Get(ports.Tenants, req.ID, &t); e != nil {
				return e
			}
			if t.BootstrapVersion > 0 {
				return fault.Conflict("已开通租户请先停用并执行独立数据清理流程")
			}
			for _, table := range []ports.Table{ports.Members, ports.Entitlements, ports.Orgs, ports.Roles} {
				n, e := u.Count(table, eq("tenant_id", t.ID))
				if e != nil {
					return e
				}
				if n > 0 {
					return fault.Conflict("租户仍有数据，不能直接删除")
				}
			}
			if e = u.Delete(ports.Tenants, eq("id", t.ID)); e != nil {
				return e
			}
		case c.KindMembers:
			if e = s.require(u, a, "membership.write"); e != nil {
				return e
			}
			m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", req.ID))
			if e != nil {
				return e
			}
			if e = s.protectAdmin(u, tid, m.UserID, 0); e != nil {
				return e
			}
			for _, table := range []ports.Table{ports.RoleMembers, ports.MemberOrgs, ports.MemberPositions} {
				if e = u.Delete(table, filter("tenant_id", tid, "user_id", m.UserID)); e != nil {
					return e
				}
			}
			m.Status = 0
			m.DefaultAppID = 0
			m.DefaultOrgID = 0
			m.DefaultUnitID = 0
			m.IsDefault = false
			if e = u.Save(ports.Members, &m); e != nil {
				return e
			}
		case c.KindOrgs:
			if e = s.require(u, a, "organization.write"); e != nil {
				return e
			}
			var org organization.Org
			if e = u.Get(ports.Orgs, req.ID, &org); e != nil {
				return e
			}
			if org.TenantID != tid {
				return fault.Forbidden
			}
			checks := map[ports.Table]string{ports.Orgs: "parent_id", ports.Positions: "org_id", ports.MemberOrgs: "org_id", ports.Members: "default_org_id"}
			for table, column := range checks {
				n, e := u.Count(table, filter("tenant_id", tid, column, org.ID))
				if e != nil {
					return e
				}
				if n > 0 {
					return fault.Conflict("组织仍有子节点或引用")
				}
			}
			if e = u.Delete(ports.Orgs, f); e != nil {
				return e
			}
		case c.KindPositions:
			if e = s.require(u, a, "organization.write"); e != nil {
				return e
			}
			var p organization.Position
			if e = u.Get(ports.Positions, req.ID, &p); e != nil {
				return e
			}
			if p.TenantID != tid {
				return fault.Forbidden
			}
			n, e := u.Count(ports.MemberPositions, filter("tenant_id", tid, "position_id", p.ID))
			if e != nil {
				return e
			}
			if n > 0 {
				return fault.Conflict("岗位仍有成员")
			}
			if e = u.Delete(ports.Positions, f); e != nil {
				return e
			}
		case c.KindRoles:
			if e = s.require(u, a, "access.write"); e != nil {
				return e
			}
			var r access.Role
			if e = u.Get(ports.Roles, req.ID, &r); e != nil {
				return e
			}
			if r.TenantID != tid {
				return fault.Forbidden
			}
			if e = s.protectAdmin(u, tid, 0, r.ID); e != nil {
				return e
			}
			for _, table := range []ports.Table{ports.RoleMembers, ports.Grants} {
				if e = u.Delete(table, filter("tenant_id", tid, "role_id", r.ID)); e != nil {
					return e
				}
			}
			if e = u.Delete(ports.Roles, f); e != nil {
				return e
			}
		case c.KindResources:
			if e = requirePlatform(a); e != nil {
				return e
			}
			var r catalog.Resource
			if e = u.Get(ports.Resources, req.ID, &r); e != nil {
				return e
			}
			n, e := u.Count(ports.Resources, eq("parent_id", r.ID))
			if e != nil {
				return e
			}
			if n > 0 {
				return fault.Conflict("资源仍有子节点")
			}
			n, e = u.Count(ports.Operations, eq("resource_id", r.ID))
			if e != nil {
				return e
			}
			if n > 0 {
				return fault.Conflict("资源仍有关联操作，请先修改应用注册定义")
			}
			for _, table := range []ports.Table{ports.Grants, ports.TenantResources} {
				if e = u.Delete(table, eq("resource_id", r.ID)); e != nil {
					return e
				}
			}
			if e = u.Delete(ports.Resources, eq("id", r.ID)); e != nil {
				return e
			}
		case c.KindApps:
			if e = requirePlatform(a); e != nil {
				return e
			}
			var app catalog.App
			if e = u.Get(ports.Apps, req.ID, &app); e != nil {
				return e
			}
			if app.Managed {
				return fault.Conflict("已注册应用请通过发布流程管理")
			}
			for _, table := range []ports.Table{ports.Resources, ports.Entitlements} {
				n, e := u.Count(table, eq("app_id", app.ID))
				if e != nil {
					return e
				}
				if n > 0 {
					return fault.Conflict("应用仍有资源或开通记录")
				}
			}
			if e = u.Delete(ports.Apps, eq("id", app.ID)); e != nil {
				return e
			}
		default:
			return fault.Invalid("不支持的删除操作")
		}
		return s.record(u, a, auditEntity(req.Kind)+".delete", req.ID, req.Context)
	})
	return &c.Result{Success: e == nil}, e
}
