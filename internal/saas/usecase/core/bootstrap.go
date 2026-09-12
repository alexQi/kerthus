package core

import (
	"context"
	"errors"
	"fmt"
	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/dictionary"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
)

// Definition is trusted, validated deployment configuration, never an HTTP payload.
type Definition struct {
	App                catalog.App
	Resources          []catalog.Resource
	Parents            map[string]string
	Operations         []catalog.Operation
	OperationResources map[string]string
}

// RegisterApplication retains database IDs so upgrades do not invalidate grants.
// Removed resources are disabled; new resources are never automatically entitled.
func (s *Service) RegisterApplication(ctx context.Context, d Definition) error {
	return s.DB.Write(ctx, func(u ports.Unit) error {
		app, e := first[catalog.App](u, ports.Apps, eq("code", d.App.Code))
		fresh := errors.Is(e, fault.NotFound)
		if e != nil && !fresh {
			return e
		}
		if catalog.AppType(d.App.Type) != "self" || (!fresh && catalog.AppType(app.Type) != "self") {
			return fault.Conflict("第三方链接不能注册为后端服务应用")
		}
		if !fresh && app.ResourceVersion > d.App.ResourceVersion {
			return fault.Conflict("应用资源版本不允许回退")
		}
		if !fresh && app.ResourceVersion == d.App.ResourceVersion {
			if app.ManifestHash != d.App.ManifestHash {
				return fault.Conflict("应用定义已变化，请增加 resource_version")
			}
			return nil
		}
		id, created := app.ID, app.CreatedAt
		previousStatus, wasManaged := app.Status, app.Managed
		icon, description, remark, isPublic := app.Icon, app.Description, app.Remark, app.IsPublic
		app = d.App
		app.Type, app.URL = "self", ""
		if !fresh {
			// These fields belong to catalog editors, not application releases.
			app.Icon, app.Description, app.Remark = icon, description, remark
			app.IsPublic = isPublic
		}
		app.ID = id
		app.CreatedAt = created
		app.Managed = true
		app.Status = 1
		if !fresh && wasManaged {
			app.Status = previousStatus
		}
		if e = u.Save(ports.Apps, &app); e != nil {
			return e
		}
		existing, e := all[catalog.Resource](u, ports.Resources, eq("app_id", app.ID))
		if e != nil {
			return e
		}
		byCode := map[string]catalog.Resource{}
		for _, r := range existing {
			byCode[r.Code] = r
		}
		desired := map[string]catalog.Resource{}
		for _, v := range d.Resources {
			old := byCode[v.Code]
			v.ID = old.ID
			v.CreatedAt = old.CreatedAt
			v.AppID = app.ID
			v.Status = 1
			v.ParentID = 0
			if e = u.Save(ports.Resources, &v); e != nil {
				return e
			}
			desired[v.Code] = v
		}
		for code, v := range desired {
			if parent := d.Parents[code]; parent != "" {
				p, ok := desired[parent]
				if !ok {
					return fault.Invalid("资源父级不存在")
				}
				v.ParentID = p.ID
				if e = u.Save(ports.Resources, &v); e != nil {
					return e
				}
				desired[code] = v
			}
		}
		for code, v := range byCode {
			if _, ok := desired[code]; !ok {
				v.Status = 0
				if e = u.Save(ports.Resources, &v); e != nil {
					return e
				}
			}
		}
		if e = u.Delete(ports.Operations, eq("app_id", app.ID)); e != nil {
			return e
		}
		for _, op := range d.Operations {
			resource, ok := desired[d.OperationResources[op.OperationID]]
			if !ok {
				return fault.Invalid("API资源不存在")
			}
			op.ID = 0
			op.AppID = app.ID
			op.ResourceID = resource.ID
			if e = u.Save(ports.Operations, &op); e != nil {
				return e
			}
		}
		return nil
	})
}

// Bootstrap creates one explicit platform account. A repeated seed never changes
// existing passwords, promotes an existing user, or restores revoked permissions.
func (s *Service) Bootstrap(ctx context.Context, phone, password string) error {
	if e := required(phone, "管理员手机号"); e != nil {
		return e
	}
	return s.DB.Write(ctx, func(u ports.Unit) error {
		count, e := u.Count(ports.Users, eq("platform_admin", true))
		if e != nil {
			return e
		}
		if count > 0 {
			return nil
		}
		existing, e := first[identity.User](u, ports.Users, eq("phone", phone))
		if e == nil {
			return fmt.Errorf("bootstrap refuses to promote existing user %d", existing.ID)
		}
		if !errors.Is(e, fault.NotFound) {
			return e
		}
		system, e := first[catalog.App](u, ports.Apps, eq("code", "system"))
		if e != nil {
			return fmt.Errorf("register built-in applications first: %w", e)
		}
		basic, e := first[catalog.App](u, ports.Apps, eq("code", "basic"))
		if e != nil {
			return e
		}
		hash, e := passwordHash(password)
		if e != nil {
			return e
		}
		user := identity.User{Phone: phone, Name: "平台管理员", PasswordHash: hash, Status: 1, PlatformAdmin: true, SingleLogin: true, AuthVersion: 1}
		if e = u.Save(ports.Users, &user); e != nil {
			return e
		}
		t := tenant.Tenant{Name: "平台运营", ContactPerson: user.Name, ContactPhone: phone, AddressJSON: "[]", Status: 1, VerifyStatus: 1, BootstrapVersion: 1}
		if e = u.Save(ports.Tenants, &t); e != nil {
			return e
		}
		org := organization.Org{TenantID: t.ID, Name: t.Name, Type: "unit", Status: 1}
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
		member := tenant.Member{TenantID: t.ID, UserID: user.ID, Status: 1, DefaultAppID: system.ID, DefaultOrgID: org.ID, DefaultUnitID: org.ID, IsDefault: true}
		if e = u.Save(ports.Members, &member); e != nil {
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
		for _, app := range []catalog.App{system, basic} {
			ent := catalog.Entitlement{TenantID: t.ID, AppID: app.ID, Status: 1}
			if e = u.Save(ports.Entitlements, &ent); e != nil {
				return e
			}
			resources, e := all[catalog.Resource](u, ports.Resources, eq("app_id", app.ID))
			if e != nil {
				return e
			}
			for _, r := range resources {
				if e = u.Save(ports.TenantResources, &catalog.TenantResource{TenantID: t.ID, AppID: app.ID, ResourceID: r.ID}); e != nil {
					return e
				}
				if e = u.Save(ports.Grants, &access.Grant{TenantID: t.ID, RoleID: role.ID, AppID: app.ID, ResourceID: r.ID}); e != nil {
					return e
				}
			}
		}
		return nil
	})
}

// ImportDistricts upserts a validated full hierarchy without deleting old IDs
// that may already occur in tenant addresses.
func (s *Service) ImportDistricts(ctx context.Context, rows []dictionary.District) error {
	return s.DB.Write(ctx, func(u ports.Unit) error {
		for _, row := range rows {
			var old dictionary.District
			e := u.Get(ports.Districts, row.ID, &old)
			if errors.Is(e, fault.NotFound) {
				e = u.Insert(ports.Districts, &row)
			} else if e == nil {
				e = u.Save(ports.Districts, &row)
			}
			if e != nil {
				return e
			}
		}
		return nil
	})
}
