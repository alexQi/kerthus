package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"kerthus/internal/saas/domain/audit"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

type Service struct {
	DB                 ports.Database
	Sessions           ports.Sessions
	Objects            ports.Objects
	Now                func() time.Time
	AttachmentMaxBytes int64
}

func New(db ports.Database, sessions ports.Sessions, objects ports.Objects) *Service {
	return &Service{DB: db, Sessions: sessions, Objects: objects, Now: time.Now, AttachmentMaxBytes: DefaultAttachmentMaxBytes}
}
func (s *Service) now() int64         { return s.Now().Unix() }
func eq(k string, v any) ports.Filter { return ports.Filter{Equal: map[string]any{k: v}} }
func filter(kv ...any) ports.Filter {
	f := ports.Filter{Equal: map[string]any{}}
	for i := 0; i < len(kv); i += 2 {
		f.Equal[kv[i].(string)] = kv[i+1]
	}
	return f
}
func first[T any](u ports.Unit, t ports.Table, f ports.Filter) (T, error) {
	var zero T
	var out []T
	f.PageSize = 1
	if e := u.Find(t, f, &out); e != nil {
		return zero, e
	}
	if len(out) == 0 {
		return zero, fault.NotFound
	}
	return out[0], nil
}
func all[T any](u ports.Unit, t ports.Table, f ports.Filter) ([]T, error) {
	var out []T
	e := u.Find(t, f, &out)
	return out, e
}
func status(v *int32, old int32) int32 {
	if v == nil {
		return old
	}
	return *v
}
func ptr(v int32) *int32 { return &v }
func validStatus(v int32) error {
	if v != 0 && v != 1 {
		return fault.Invalid("状态只能为 0 或 1")
	}
	return nil
}
func required(v, label string) error {
	if strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > 191 {
		return fault.Invalid(label + "不能为空且不能超过191字")
	}
	return nil
}
func randomToken() (string, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
func passwordHash(password string) (string, error) {
	if len(password) < 10 || len(password) > 72 {
		return "", fault.Invalid("密码长度应为10至72字节")
	}
	b, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), e
}
func userDTO(v identity.User) *c.User {
	return &c.User{ID: v.ID, Phone: v.Phone, Email: v.Email, Name: v.Name, Avatar: v.Avatar, Sex: v.Sex, Status: ptr(v.Status), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func (s *Service) record(u ports.Unit, a *actor, action string, target int64, req *c.Context) error {
	return u.Save(ports.Audits, &audit.Event{ActorID: a.User.ID, TenantID: a.Tenant.ID, AppID: a.App.ID, Action: action, TargetID: target, RequestID: req.RequestID, CreatedAt: s.now()})
}

// Entity-specific action codes keep deleted objects traceable without relying
// on a mutable row lookup or guessing which table a numeric ID belongs to.
func auditEntity(kind c.Kind) string {
	switch kind {
	case c.KindUsers:
		return "user"
	case c.KindTenants:
		return "tenant"
	case c.KindMembers:
		return "member"
	case c.KindOrgs:
		return "organization"
	case c.KindPositions:
		return "position"
	case c.KindApps:
		return "application"
	case c.KindResources:
		return "resource"
	case c.KindRoles:
		return "role"
	default:
		return "entity"
	}
}
func (s *Service) Health(ctx context.Context, _ *c.Empty) (*c.Result, error) {
	if e := s.DB.Ping(ctx); e != nil {
		return nil, e
	}
	if e := s.Sessions.Ping(ctx); e != nil {
		return nil, e
	}
	return &c.Result{Success: true}, nil
}
func copyJSON(from, to any) error {
	b, e := json.Marshal(from)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, to)
}
func domainError(v string) error { return fmt.Errorf("invalid persisted %s", v) }
