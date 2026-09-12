package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/ports"
	"kerthus/migrations"
)

type Store struct{ db *gorm.DB }

func Open(dsn string) (*Store, error) {
	config, e := driver.ParseDSN(dsn)
	if e != nil {
		return nil, fmt.Errorf("invalid mysql configuration")
	}
	// An unchanged update is still a successful save, regardless of caller DSN.
	config.ClientFoundRows = true
	db, e := gorm.Open(gmysql.Open(config.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), SkipDefaultTransaction: true})
	if e != nil {
		return nil, fmt.Errorf("connect mysql: %w", e)
	}
	raw, e := db.DB()
	if e != nil {
		return nil, e
	}
	raw.SetMaxOpenConns(16)
	raw.SetMaxIdleConns(4)
	raw.SetConnMaxLifetime(5 * time.Minute)
	return &Store{db}, nil
}
func (s *Store) Close() error {
	db, e := s.db.DB()
	if e != nil {
		return e
	}
	return db.Close()
}
func (s *Store) Ping(ctx context.Context) error {
	db, e := s.db.DB()
	if e != nil {
		return e
	}
	return db.PingContext(ctx)
}
func (s *Store) Read(ctx context.Context, fn func(ports.Unit) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&unit{tx}) }, &sql.TxOptions{ReadOnly: true})
}
func (s *Store) Write(ctx context.Context, fn func(ports.Unit) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u := &unit{tx}
		if e := u.LockCatalog(); e != nil {
			return e
		}
		return fn(u)
	})
}

type unit struct{ db *gorm.DB }

var columns = map[ports.Table]string{
	ports.Users: "id phone email name status platform_admin created_at updated_at", ports.Tenants: "id name status verify_status bootstrap_version created_at updated_at expires_at", ports.Members: "id tenant_id user_id status is_default default_app_id default_org_id default_unit_id created_at updated_at",
	ports.Orgs: "id tenant_id parent_id unit_id type status name sort created_at updated_at", ports.Positions: "id tenant_id org_id status name created_at updated_at", ports.MemberOrgs: "id tenant_id user_id org_id", ports.MemberPositions: "id tenant_id user_id position_id",
	ports.Apps: "id code name status managed service_key route_prefix created_at updated_at", ports.Resources: "id app_id parent_id code name status type is_public sort created_at updated_at", ports.Operations: "id app_id resource_id operation_id method path group_name action",
	ports.Entitlements: "id tenant_id app_id status expires_at created_at updated_at", ports.TenantResources: "id tenant_id app_id resource_id", ports.Roles: "id tenant_id code name status administrator created_at updated_at", ports.RoleMembers: "id tenant_id role_id user_id", ports.Grants: "id tenant_id role_id app_id resource_id", ports.Audits: "id tenant_id actor_id app_id created_at", ports.Files: "id tenant_id app_id user_id object_key status", ports.Districts: "id parent_id name",
}

func allowed(t ports.Table, col string) bool {
	c, ok := columns[t]
	return ok && strings.Contains(" "+c+" ", " "+col+" ")
}
func (u *unit) base(t ports.Table, f ports.Filter) (*gorm.DB, error) {
	if _, ok := columns[t]; !ok {
		return nil, fmt.Errorf("unknown table")
	}
	q := u.db.Table(string(t))
	for k, v := range f.Equal {
		if !allowed(t, k) {
			return nil, fmt.Errorf("invalid column %s", k)
		}
		q = q.Where("`"+k+"` = ?", v)
	}
	for k, v := range f.In {
		if !allowed(t, k) {
			return nil, fmt.Errorf("invalid column %s", k)
		}
		if len(v) == 0 {
			q = q.Where("1=0")
		} else {
			q = q.Where("`"+k+"` IN ?", v)
		}
	}
	for column, value := range f.Contains {
		if !allowed(t, column) {
			return nil, fmt.Errorf("invalid column %s", column)
		}
		q = q.Where("`"+column+"` LIKE ?", containsPattern(value))
	}
	for column, value := range f.GreaterEqual {
		if !allowed(t, column) {
			return nil, fmt.Errorf("invalid column %s", column)
		}
		q = q.Where("`"+column+"` >= ?", value)
	}
	for column, value := range f.LessThan {
		if !allowed(t, column) {
			return nil, fmt.Errorf("invalid column %s", column)
		}
		q = q.Where("`"+column+"` < ?", value)
	}
	if f.Search != "" {
		pattern := containsPattern(f.Search)
		if t == ports.Operations {
			q = q.Where("(operation_id LIKE ? OR path LIKE ? OR group_name LIKE ?)", pattern, pattern, pattern)
		} else if t == ports.Users {
			q = q.Where("(name LIKE ? OR phone LIKE ? OR email LIKE ?)", pattern, pattern, pattern)
		} else {
			if !allowed(t, "name") {
				return nil, fault.Invalid("该列表不支持搜索")
			}
			q = q.Where("name LIKE ?", pattern)
		}
	}
	return q, nil
}

func containsPattern(value string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value) + "%"
}
func (u *unit) Get(t ports.Table, id int64, out any) error {
	q, e := u.base(t, ports.Filter{})
	if e != nil {
		return e
	}
	return dbError(q.Where("id = ?", id).First(out).Error)
}
func (u *unit) Find(t ports.Table, f ports.Filter, out any) error {
	q, e := u.base(t, f)
	if e != nil {
		return e
	}
	o := f.Order
	if o == "" {
		o = "id"
	}
	if !allowed(t, o) {
		return fault.Invalid("不支持的排序字段")
	}
	order := "`" + o + "` ASC"
	if f.Desc {
		order = "`" + o + "` DESC"
	}
	q = q.Order(order)
	if o != "id" {
		// Keep pages deterministic when display names or explicit sort values tie.
		q = q.Order("`id` ASC")
	}
	if f.PageSize > 0 {
		p := f.Page
		if p < 1 {
			p = 1
		}
		q = q.Limit(f.PageSize).Offset((p - 1) * f.PageSize)
	}
	return dbError(q.Find(out).Error)
}
func (u *unit) Count(t ports.Table, f ports.Filter) (int64, error) {
	q, e := u.base(t, f)
	if e != nil {
		return 0, e
	}
	var n int64
	e = q.Count(&n).Error
	return n, dbError(e)
}
func (u *unit) Insert(t ports.Table, v any) error {
	if _, ok := columns[t]; !ok {
		return fmt.Errorf("unknown table")
	}
	return dbError(u.db.Table(string(t)).Create(v).Error)
}
func (u *unit) Save(t ports.Table, value any) error {
	if t == ports.Operations {
		if op, ok := value.(*catalog.Operation); ok {
			return u.saveOperation(op)
		}
	}
	if _, ok := columns[t]; !ok {
		return fmt.Errorf("unknown table")
	}
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("model must be pointer")
	}
	id := v.Elem().FieldByName("ID")
	if !id.IsValid() {
		return fmt.Errorf("missing ID")
	}
	q := u.db.Table(string(t))
	if id.Int() == 0 {
		return dbError(q.Create(value).Error)
	}
	res := q.Where("id = ?", id.Int()).Select("*").Updates(value)
	if res.Error != nil {
		return dbError(res.Error)
	}
	if res.RowsAffected == 0 {
		return fault.NotFound
	}
	return nil
}
func (u *unit) Delete(t ports.Table, f ports.Filter) error {
	if len(f.Equal)+len(f.In) == 0 {
		return fmt.Errorf("unscoped delete")
	}
	q, e := u.base(t, f)
	if e != nil {
		return e
	}
	return dbError(q.Delete(&struct{}{}).Error)
}
func (u *unit) LockTenant(id int64) error {
	var n int64
	return dbError(u.db.Raw("SELECT id FROM tenants WHERE id=? FOR UPDATE", id).Scan(&n).Error)
}
func (u *unit) LockCatalog() error {
	var n int
	res := u.db.Raw("SELECT id FROM platform_lock WHERE id=1 FOR UPDATE").Scan(&n)
	if res.Error != nil {
		return res.Error
	}
	if n != 1 {
		return fmt.Errorf("schema not initialized; run saasctl migrate")
	}
	return nil
}
func dbError(e error) error {
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return fault.NotFound
	}
	var me *driver.MySQLError
	if errors.As(e, &me) {
		switch me.Number {
		case 1062:
			return fault.Conflict("记录已存在")
		case 1451, 1452:
			return fault.Conflict("记录仍被引用或关联不存在")
		}
	}
	return e
}

func (s *Store) Migrate(ctx context.Context) error {
	raw, e := s.db.DB()
	if e != nil {
		return e
	}
	conn, e := raw.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK('kerthus_schema_migrations', 30)").Scan(&locked); e != nil || locked != 1 {
		return fmt.Errorf("acquire migration lock: %v", e)
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('kerthus_schema_migrations')")
	if _, e = conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(191) PRIMARY KEY, checksum VARCHAR(64) NOT NULL, applied_at BIGINT NOT NULL)"); e != nil {
		return e
	}
	entries, e := migrations.Files.ReadDir("mysql")
	if e != nil {
		return e
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		b, e := migrations.Files.ReadFile("mysql/" + entry.Name())
		if e != nil {
			return e
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(b))
		var old string
		e = conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=?", entry.Name()).Scan(&old)
		if e == nil {
			if old != sum {
				return fmt.Errorf("migration checksum changed: %s", entry.Name())
			}
			continue
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		for _, statement := range strings.Split(string(b), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, e = conn.ExecContext(ctx, statement); e != nil {
				return fmt.Errorf("migration %s: %w", entry.Name(), e)
			}
		}
		if _, e = conn.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(?,?,?)", entry.Name(), sum, time.Now().Unix()); e != nil {
			return e
		}
	}
	return nil
}

// SQL NULL means registered but unbound. The domain represents this as ID zero.
// Keeping the foreign key prevents a binding from referring to a missing resource.
type operationRow struct {
	ID          int64
	AppID       int64
	ResourceID  *int64
	OperationID string
	Method      string
	Path        string
	GroupName   string
	Action      string
}

func (u *unit) saveOperation(op *catalog.Operation) error {
	row := operationRow{ID: op.ID, AppID: op.AppID, OperationID: op.OperationID, Method: op.Method, Path: op.Path, GroupName: op.GroupName, Action: op.Action}
	if op.ResourceID > 0 {
		row.ResourceID = &op.ResourceID
	}
	q := u.db.Table(string(ports.Operations))
	if op.ID == 0 {
		if e := dbError(q.Create(&row).Error); e != nil {
			return e
		}
		op.ID = row.ID
		return nil
	}
	res := q.Where("id = ?", op.ID).Select("*").Updates(&row)
	if res.Error != nil {
		return dbError(res.Error)
	}
	if res.RowsAffected == 0 {
		return fault.NotFound
	}
	return nil
}
