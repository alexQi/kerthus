// Package legacy imports a read-only snapshot of the historical SaaS database.
package legacy

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"kerthus/internal/platform/appmanifest"
	"kerthus/internal/saas/adapters/mysql"
	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/audit"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/dictionary"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Report struct {
	ExcludedIDs     map[string][]int64 `json:"excluded_ids"`
	Source          string             `json:"source"`
	Target          string             `json:"target"`
	SnapshotSHA256  string             `json:"snapshot_sha256"`
	SourceCounts    map[string]int     `json:"source_counts"`
	Imported        map[string]int     `json:"imported"`
	Excluded        map[string]int     `json:"excluded"`
	Warnings        []string           `json:"warnings"`
	PlatformUserIDs []int64            `json:"platform_user_ids"`
	CompletedAt     string             `json:"completed_at"`
}
type Options struct{ SourceDSN, TargetDatabase, ProjectRoot, ReportPath string }

var tables = []string{"d_user", "d_tenant", "d_tenant_employee", "d_tenant_org", "d_tenant_position", "d_tenant_employee_org", "d_tenant_employee_position", "d_tenant_role", "d_tenant_role_item", "d_app", "d_app_resource", "d_app_resource_api", "d_tenant_app", "d_tenant_app_resource", "d_tenant_role_resource", "d_config_district", "d_access_log"}

func readRows(ctx context.Context, tx *sql.Tx, table string) ([]Row, error) {
	rs, e := tx.QueryContext(ctx, "SELECT * FROM `"+table+"` ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rs.Close()
	cols, e := rs.Columns()
	if e != nil {
		return nil, e
	}
	out := []Row{}
	for rs.Next() {
		values := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range values {
			ptr[i] = &values[i]
		}
		if e = rs.Scan(ptr...); e != nil {
			return nil, e
		}
		row := Row{}
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		out = append(out, row)
	}
	return out, rs.Err()
}
func index(rows []Row) map[int64]Row {
	out := map[int64]Row{}
	for _, r := range rows {
		out[Int(r, "id")] = r
	}
	return out
}
func key(ids ...int64) string {
	parts := []string{}
	for _, id := range ids {
		parts = append(parts, fmt.Sprint(id))
	}
	return strings.Join(parts, ":")
}
func state(r Row) int32 {
	if Int(r, "is_del") != 0 {
		return 0
	}
	return int32(Int(r, "status"))
}
func stamp(r Row) (int64, int64) { return Int(r, "created_at"), Int(r, "updated_at") }
func Run(ctx context.Context, opt Options) (*Report, error) {
	cfg, e := driver.ParseDSN(opt.SourceDSN)
	if e != nil {
		return nil, fmt.Errorf("invalid source DSN")
	}
	if cfg.DBName == "" || opt.TargetDatabase == cfg.DBName || !databaseName(opt.TargetDatabase) {
		return nil, fmt.Errorf("target must be a separate named database")
	}
	sourceName := cfg.DBName
	source, e := sql.Open("mysql", opt.SourceDSN)
	if e != nil {
		return nil, e
	}
	defer source.Close()
	tx, e := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	snapshot := map[string][]Row{}
	report := &Report{Source: sourceName, Target: opt.TargetDatabase, SourceCounts: map[string]int{}, Imported: map[string]int{}, Excluded: map[string]int{}, ExcludedIDs: map[string][]int64{}}
	for _, table := range tables {
		rows, e := readRows(ctx, tx, table)
		if e != nil {
			return nil, fmt.Errorf("read %s: %w", table, e)
		}
		snapshot[table] = rows
		report.SourceCounts[table] = len(rows)
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(snapshot)
	if e != nil {
		return nil, e
	}
	report.SnapshotSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	ops, e := appmanifest.LoadSpec(filepath.Join(opt.ProjectRoot, "api/http/legacy.openapi.yaml"))
	if e != nil {
		return nil, e
	}
	mapped, e := Catalog(snapshot["d_app"], snapshot["d_app_resource"], snapshot["d_app_resource_api"], ops, opt.ProjectRoot)
	if e != nil {
		return nil, e
	}
	report.Warnings = append(report.Warnings, mapped.Warnings...)
	cfg.DBName = ""
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return nil, e
	}
	defer admin.Close()
	var exists int
	if e = admin.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", opt.TargetDatabase).Scan(&exists); e != nil {
		return nil, e
	}
	if exists != 0 {
		return nil, fmt.Errorf("target database already exists; import never overwrites existing data")
	}
	if _, e = admin.ExecContext(ctx, "CREATE DATABASE `"+opt.TargetDatabase+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		return nil, e
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = admin.ExecContext(context.Background(), "DROP DATABASE `"+opt.TargetDatabase+"`")
		}
	}()
	cfg.DBName = opt.TargetDatabase
	cfg.ClientFoundRows = true
	target, e := mysql.Open(cfg.FormatDSN())
	if e != nil {
		return nil, e
	}
	defer target.Close()
	if e = target.Migrate(ctx); e != nil {
		return nil, e
	}
	e = target.Write(ctx, func(u ports.Unit) error { return importRows(u, snapshot, mapped, report) })
	if e != nil {
		return nil, fmt.Errorf("data transaction rolled back in new database %s: %w", opt.TargetDatabase, e)
	}
	committed = true
	report.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	if opt.ReportPath != "" {
		b, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			return nil, e
		}
		if e = os.WriteFile(opt.ReportPath, append(b, '\n'), 0600); e != nil {
			return nil, e
		}
	}
	return report, nil
}
func databaseName(v string) bool {
	if !strings.HasPrefix(v, "kerthus_") || len(v) > 64 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}
func importRows(u ports.Unit, s map[string][]Row, m CatalogResult, report *Report) error {
	users, tenants, orgs, positions, roles := index(s["d_user"]), index(s["d_tenant"]), index(s["d_tenant_org"]), index(s["d_tenant_position"]), index(s["d_tenant_role"])
	members := map[string]Row{}
	for _, r := range s["d_tenant_employee"] {
		members[key(Int(r, "tenant_id"), Int(r, "user_id"))] = r
	}
	adminIDs := map[int64]bool{}
	for _, link := range s["d_tenant_role_item"] {
		role := roles[Int(link, "role_id")]
		tid, uid := Int(link, "tenant_id"), Int(link, "item_id")
		member := members[key(tid, uid)]
		if Int(link, "type") == 0 && String(role, "code") == "admin" && Int(role, "tenant_id") == 1 && tid == 1 && state(role) == 1 && state(users[uid]) == 1 && member != nil && Int(member, "is_del") == 0 && state(tenants[tid]) == 1 {
			adminIDs[uid] = true
		}
	}
	if len(adminIDs) == 0 {
		return fmt.Errorf("no explicit active platform admin found; refusing inferred tenant-ID privilege")
	}
	for id := range adminIDs {
		report.PlatformUserIDs = append(report.PlatformUserIDs, id)
	}
	sort.Slice(report.PlatformUserIDs, func(i, j int) bool { return report.PlatformUserIDs[i] < report.PlatformUserIDs[j] })
	insert := func(table ports.Table, v any) error {
		if e := u.Insert(table, v); e != nil {
			return fmt.Errorf("insert %s: %w", table, e)
		}
		report.Imported[string(table)]++
		return nil
	}
	skip := func(reason string, id int64) {
		report.Excluded[reason]++
		report.ExcludedIDs[reason] = append(report.ExcludedIDs[reason], id)
	}
	for _, r := range s["d_user"] {
		created, updated := stamp(r)
		hash := String(r, "password")
		if len(hash) != 32 || strings.IndexFunc(hash, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') }) >= 0 {
			return fmt.Errorf("unsupported password format for user ID %d", Int(r, "id"))
		}
		v := identity.User{ID: Int(r, "id"), Phone: String(r, "phone"), Email: String(r, "email"), Name: String(r, "name"), Avatar: String(r, "avatar"), Sex: int32(Int(r, "sex")), Status: state(r), PasswordHash: "legacy-beehive$" + hash, PlatformAdmin: adminIDs[Int(r, "id")], SingleLogin: Int(r, "is_signal_login") != 0, AuthVersion: 1, CreatedAt: created, UpdatedAt: updated}
		if e := insert(ports.Users, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant"] {
		var labels, codes any
		if e := json.Unmarshal([]byte(String(r, "address")), &labels); e != nil {
			return fmt.Errorf("invalid address for tenant %d", Int(r, "id"))
		}
		if e := json.Unmarshal([]byte(String(r, "address_code")), &codes); e != nil {
			return fmt.Errorf("invalid address codes for tenant %d", Int(r, "id"))
		}
		addr, _ := json.Marshal(map[string]any{"labels": labels, "codes": codes, "area_code": String(r, "area_code")})
		created, updated := stamp(r)
		v := tenant.Tenant{ID: Int(r, "id"), Name: String(r, "name"), Logo: String(r, "logo"), ContactPerson: String(r, "contact_person"), ContactPhone: String(r, "contact_phone"), ContactEmail: String(r, "contact_email"), CreditCode: String(r, "credit_code"), AddressJSON: string(addr), AddressDetail: String(r, "address_detail"), Description: String(r, "desc"), Status: state(r), VerifyStatus: int32(Int(r, "verify_status")), ExpiresAt: Int(r, "expiration_time"), CreatedAt: created, UpdatedAt: updated}
		if v.VerifyStatus == 1 {
			v.BootstrapVersion = 1
		}
		if e := insert(ports.Tenants, &v); e != nil {
			return e
		}
	}
	appIDs := map[int64]bool{}
	for _, v := range m.Apps {
		if v.Code == "system" || v.Code == "basic" {
			v.Managed = true
			v.ResourceVersion = 2
			v.ManifestHash = report.SnapshotSHA256
		}
		appIDs[v.ID] = true
		if e := insert(ports.Apps, &v); e != nil {
			return e
		}
	}
	resourceIDs := map[int64]catalog.Resource{}
	for _, v := range m.Resources {
		resourceIDs[v.ID] = v
		if e := insert(ports.Resources, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_org"] {
		if tenants[Int(r, "tenant_id")] == nil {
			return fmt.Errorf("orphan org %d", Int(r, "id"))
		}
		created, updated := stamp(r)
		unit := Int(r, "unit_id")
		if String(r, "type") == "unit" {
			unit = Int(r, "id")
		}
		v := organization.Org{ID: Int(r, "id"), TenantID: Int(r, "tenant_id"), ParentID: Int(r, "parent_id"), UnitID: unit, Type: String(r, "type"), Name: String(r, "name"), ShortName: String(r, "short_name"), Status: state(r), Sort: int32(Int(r, "sort")), Remark: String(r, "remark"), CreatedAt: created, UpdatedAt: updated}
		if e := insert(ports.Orgs, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_position"] {
		resolvedOrg, e := positionOrg(r, s, orgs)
		if e != nil {
			return e
		}
		if resolvedOrg != Int(r, "org_id") {
			report.Warnings = append(report.Warnings, fmt.Sprintf("position %d: missing org %d resolved to %d using unanimous existing member-org relationships", Int(r, "id"), Int(r, "org_id"), resolvedOrg))
		}
		created, updated := stamp(r)
		v := organization.Position{ID: Int(r, "id"), TenantID: Int(r, "tenant_id"), OrgID: resolvedOrg, Name: String(r, "name"), Status: state(r), Remark: String(r, "remark"), CreatedAt: created, UpdatedAt: updated}
		if e := insert(ports.Positions, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_employee"] {
		tid, uid := Int(r, "tenant_id"), Int(r, "user_id")
		if tenants[tid] == nil || users[uid] == nil {
			return fmt.Errorf("orphan member %d", Int(r, "id"))
		}
		created, updated := stamp(r)
		status := int32(1)
		if Int(r, "is_del") != 0 {
			status = 0
		}
		v := tenant.Member{ID: Int(r, "id"), TenantID: tid, UserID: uid, Status: status, DefaultAppID: Int(r, "app_id"), DefaultUnitID: Int(r, "unit_id"), DefaultOrgID: Int(r, "section_id"), IsDefault: Int(r, "is_default") != 0, CreatedAt: created, UpdatedAt: updated}
		if v.DefaultOrgID == 0 {
			v.DefaultOrgID = v.DefaultUnitID
		}
		if e := insert(ports.Members, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_employee_org"] {
		tid, uid, oid := Int(r, "tenant_id"), Int(r, "user_id"), Int(r, "org_id")
		if Int(r, "is_del") != 0 {
			skip("deleted_member_org", Int(r, "id"))
			continue
		}
		if members[key(tid, uid)] == nil || orgs[oid] == nil || Int(orgs[oid], "tenant_id") != tid {
			return fmt.Errorf("invalid member organization %d", Int(r, "id"))
		}
		v := organization.MemberOrg{ID: Int(r, "id"), TenantID: tid, UserID: uid, OrgID: oid}
		if e := insert(ports.MemberOrgs, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_employee_position"] {
		tid, uid, pid := Int(r, "tenant_id"), Int(r, "user_id"), Int(r, "position_id")
		if Int(r, "is_del") != 0 {
			skip("deleted_member_position", Int(r, "id"))
			continue
		}
		if members[key(tid, uid)] == nil || positions[pid] == nil || Int(positions[pid], "tenant_id") != tid {
			return fmt.Errorf("invalid member position %d", Int(r, "id"))
		}
		v := organization.MemberPosition{ID: Int(r, "id"), TenantID: tid, UserID: uid, PositionID: pid}
		if e := insert(ports.MemberPositions, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_role"] {
		tid := Int(r, "tenant_id")
		if tenants[tid] == nil {
			return fmt.Errorf("orphan role %d", Int(r, "id"))
		}
		created, updated := stamp(r)
		code := String(r, "code")
		if code == "" {
			code = fmt.Sprintf("legacy-role-%d", Int(r, "id"))
		}
		v := access.Role{ID: Int(r, "id"), TenantID: tid, Code: code, Name: String(r, "name"), Remark: String(r, "remark"), Status: state(r), Administrator: code == "admin", CreatedAt: created, UpdatedAt: updated}
		if e := insert(ports.Roles, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_tenant_role_item"] {
		tid, rid, uid := Int(r, "tenant_id"), Int(r, "role_id"), Int(r, "item_id")
		if Int(r, "type") != 0 {
			return fmt.Errorf("unsupported role association type for ID %d", Int(r, "id"))
		}
		if roles[rid] == nil || Int(roles[rid], "tenant_id") != tid || members[key(tid, uid)] == nil {
			skip("orphan_role_member", Int(r, "id"))
			continue
		}
		v := access.RoleMember{ID: Int(r, "id"), TenantID: tid, RoleID: rid, UserID: uid}
		if e := insert(ports.RoleMembers, &v); e != nil {
			return e
		}
	}
	ents := map[string]bool{}
	for _, r := range s["d_tenant_app"] {
		tid, aid := Int(r, "tenant_id"), Int(r, "app_id")
		if tenants[tid] == nil || !appIDs[aid] {
			skip("orphan_entitlement", Int(r, "id"))
			continue
		}
		ents[key(tid, aid)] = true
		created, updated := stamp(r)
		v := catalog.Entitlement{ID: Int(r, "id"), TenantID: tid, AppID: aid, Status: 1, ExpiresAt: Int(r, "expiration_time"), CreatedAt: created, UpdatedAt: updated}
		if e := insert(ports.Entitlements, &v); e != nil {
			return e
		}
	}
	entitlementResources := map[string]bool{}
	for _, r := range s["d_tenant_app_resource"] {
		tid, aid, rid := Int(r, "tenant_id"), Int(r, "app_id"), Int(r, "resource_id")
		resource, ok := resourceIDs[rid]
		if !ents[key(tid, aid)] || !ok || resource.AppID != aid {
			skip("orphan_tenant_resource", Int(r, "id"))
			continue
		}
		entitlementResources[key(tid, aid, rid)] = true
		v := catalog.TenantResource{ID: Int(r, "id"), TenantID: tid, AppID: aid, ResourceID: rid}
		if e := insert(ports.TenantResources, &v); e != nil {
			return e
		}
	}
	effective := map[string]int64{}
	for _, r := range s["d_tenant_role_resource"] {
		tid, aid, rid, resid := Int(r, "tenant_id"), Int(r, "app_id"), Int(r, "role_id"), Int(r, "resource_id")
		resource, ok := resourceIDs[resid]
		if roles[rid] == nil || Int(roles[rid], "tenant_id") != tid {
			skip("orphan_grant_role", Int(r, "id"))
			continue
		}
		if !ok || resource.AppID != aid {
			skip("orphan_grant_resource", Int(r, "id"))
			continue
		}
		if !entitlementResources[key(tid, aid, resid)] {
			skip("grant_outside_entitlement", Int(r, "id"))
			continue
		}
		scope := Int(r, "data_access")
		if scope < 0 || scope > 5 {
			return fmt.Errorf("invalid grant scope %d", Int(r, "id"))
		}
		effective[key(rid, resid)] = scope
		v := access.Grant{ID: Int(r, "id"), TenantID: tid, RoleID: rid, AppID: aid, ResourceID: resid, DataScope: int32(scope)}
		if e := insert(ports.Grants, &v); e != nil {
			return e
		}
	}
	// Collapsing a historical OR binding is safe only if every extant role has
	// exactly the same effective permission and scope for all candidate resources.
	if e := validateBindings(m.BindingCandidates, roles, effective); e != nil {
		return e
	}
	for _, v := range m.Operations {
		if e := insert(ports.Operations, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_config_district"] {
		v := dictionary.District{ID: Int(r, "id"), ParentID: Int(r, "parent_id"), Name: String(r, "name")}
		if v.ID <= 0 || v.Name == "" {
			return fmt.Errorf("invalid district")
		}
		if e := insert(ports.Districts, &v); e != nil {
			return e
		}
	}
	for _, r := range s["d_access_log"] {
		v := audit.Event{ID: Int(r, "id"), ActorID: Int(r, "user_id"), TenantID: Int(r, "tenant_id"), AppID: Int(r, "app_id"), Action: String(r, "method") + " " + String(r, "uri"), RequestID: fmt.Sprintf("legacy:%d", Int(r, "id")), CreatedAt: Int(r, "created_at")}
		if e := insert(ports.Audits, &v); e != nil {
			return e
		}
	}
	for table, expected := range report.Imported {
		n, e := u.Count(ports.Table(table), ports.Filter{})
		if e != nil {
			return e
		}
		if n != int64(expected) {
			return fmt.Errorf("import row count mismatch for %s", table)
		}
	}
	report.Warnings = append(report.Warnings, "Original tenant/account status and expiry retained; unsupported business applications remain non-executable.", "Existing duplicate emails retained; ambiguous email login is rejected. Use phone login.", "Only explicit active legacy admin-role accounts receive platform_admin; tenant ID alone never elevates privilege.", "Legacy audit request payloads, extended user info and unmodeled dictionaries remain in the source backup; credentials and request payloads are not emitted in this report.")
	return nil
}

// positionOrg only repairs a missing historical FK when every assigned member
// points unambiguously to the same existing organization in the same tenant.
func positionOrg(position Row, s map[string][]Row, orgs map[int64]Row) (int64, error) {
	tid, pid, oid := Int(position, "tenant_id"), Int(position, "id"), Int(position, "org_id")
	if org := orgs[oid]; org != nil && Int(org, "tenant_id") == tid {
		return oid, nil
	}
	var chosen int64
	found := false
	for _, link := range s["d_tenant_employee_position"] {
		if Int(link, "position_id") != pid || Int(link, "is_del") != 0 {
			continue
		}
		if Int(link, "tenant_id") != tid {
			return 0, fmt.Errorf("cross-tenant historical position")
		}
		own := map[int64]bool{}
		for _, m := range s["d_tenant_employee_org"] {
			org := orgs[Int(m, "org_id")]
			if Int(m, "tenant_id") == tid && Int(m, "user_id") == Int(link, "user_id") && Int(m, "is_del") == 0 && org != nil && Int(org, "tenant_id") == tid {
				own[Int(m, "org_id")] = true
			}
		}
		if len(own) != 1 {
			return 0, fmt.Errorf("position %d has no unambiguous surviving organization", pid)
		}
		for id := range own {
			if chosen != 0 && chosen != id {
				return 0, fmt.Errorf("position %d members disagree on organization", pid)
			}
			chosen = id
			found = true
		}
	}
	if !found {
		return 0, fmt.Errorf("position %d has no surviving organization evidence", pid)
	}
	return chosen, nil
}

func validateBindings(bindings map[string][]int64, roles map[int64]Row, effective map[string]int64) error {
	for route, candidates := range bindings {
		for rid := range roles {
			var expected int64
			var has bool
			for i, resid := range candidates {
				scope, ok := effective[key(rid, resid)]
				if i == 0 {
					expected, has = scope, ok
				} else if has != ok || (ok && scope != expected) {
					return fmt.Errorf("non-equivalent historical API bindings at %s for role %d", route, rid)
				}
			}
		}
	}
	return nil
}
