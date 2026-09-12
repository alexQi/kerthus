package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	r "github.com/redis/go-redis/v9"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"time"
)

type Sessions struct {
	client *r.Client
	prefix string
}

func New(addr, password string) *Sessions {
	return &Sessions{r.NewClient(&r.Options{Addr: addr, Password: password, DB: 0, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second}), "kerthus:session:"}
}

// NewNamespaced separates sessions when switching datasets whose numeric user IDs overlap.
func NewNamespaced(addr, password, namespace string) *Sessions {
	s := New(addr, password)
	if namespace != "" {
		sum := sha256.Sum256([]byte(namespace))
		s.prefix = "kerthus:" + hex.EncodeToString(sum[:]) + ":session:"
	}
	return s
}
func key(token string) string { sum := sha256.Sum256([]byte(token)); return hex.EncodeToString(sum[:]) }
func (s *Sessions) Put(ctx context.Context, token string, v identity.Session, ttl time.Duration) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return s.client.Set(ctx, s.prefix+key(token), b, ttl).Err()
}
func (s *Sessions) Get(ctx context.Context, token string) (identity.Session, error) {
	var v identity.Session
	if token == "" {
		return v, fault.Unauthorized
	}
	b, e := s.client.Get(ctx, s.prefix+key(token)).Bytes()
	if e == r.Nil {
		return v, fault.Unauthorized
	}
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Sessions) Delete(ctx context.Context, token string) error {
	return s.client.Del(ctx, s.prefix+key(token)).Err()
}
func (s *Sessions) Ping(ctx context.Context) error { return s.client.Ping(ctx).Err() }
func (s *Sessions) Close() error                   { return s.client.Close() }
