package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlSessionBackend struct{ db *sql.DB }

// NewMySQLSessionStore connects to the platform database. Schema creation is
// owned by saasctl migrate (010_agent_sessions.sql), never by HTTP handlers.
func NewMySQLSessionStore(dsn string) (*SessionStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid agent history database configuration")
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect agent history database: %w", err)
	}
	var count int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_sessions WHERE 1=0").Scan(&count); err != nil {
		db.Close()
		return nil, fmt.Errorf("agent history schema unavailable; run saasctl migrate: %w", err)
	}
	return &SessionStore{backend: &mysqlSessionBackend{db: db}, ttl: defaultSessionTTL}, nil
}

func (s *mysqlSessionBackend) create(v Session, ttl time.Duration) (Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := json.Marshal(v)
	if err != nil {
		return Session{}, err
	}
	// Indexed expiration cleanup bounds retained conversation data to 30 days.
	if _, err = s.db.ExecContext(ctx, "DELETE FROM agent_sessions WHERE expires_at <= ? AND lease_expires_at <= ? LIMIT 256", time.Now().UnixMilli(), time.Now().UnixMilli()); err != nil {
		return Session{}, err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO agent_sessions (id,user_id,tenant_id,app_id,state_json,expires_at,lease_token,lease_expires_at) VALUES (?,?,?,?,?,?,?,?)", v.ID, v.Trusted.UserID, v.Trusted.TenantID, v.Trusted.AppID, b, v.UpdatedAt.Add(ttl).UnixMilli(), "", 0)
	if err != nil {
		return Session{}, err
	}
	return v, nil
}

func (s *mysqlSessionBackend) list(owner TrustedContext, ttl time.Duration) ([]SessionSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, "SELECT state_json FROM agent_sessions WHERE user_id=? AND tenant_id=? AND app_id=? AND expires_at>? ORDER BY expires_at DESC LIMIT 100", owner.UserID, owner.TenantID, owner.AppID, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SessionSummary, 0)
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var v Session
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, summarizeSession(v))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *mysqlSessionBackend) get(id string) (Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT state_json FROM agent_sessions WHERE id=? AND expires_at>?", id, time.Now().UnixMilli()).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	var v Session
	err = json.Unmarshal(b, &v)
	return v, err
}

func (s *mysqlSessionBackend) mutate(id string, ttl time.Duration, fn func(*Session, *turnLease) error) (Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	var b []byte
	var token string
	var leaseExpiry int64
	err = tx.QueryRowContext(ctx, "SELECT state_json,lease_token,lease_expires_at FROM agent_sessions WHERE id=? AND expires_at>? FOR UPDATE", id, time.Now().UnixMilli()).Scan(&b, &token, &leaseExpiry)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	var v Session
	if err = json.Unmarshal(b, &v); err != nil {
		return Session{}, err
	}
	lease := turnLease{Token: token, ExpiresAt: time.UnixMilli(leaseExpiry)}
	if err = fn(&v, &lease); err != nil {
		return Session{}, err
	}
	v.UpdatedAt = time.Now().UTC()
	b, err = json.Marshal(v)
	if err != nil {
		return Session{}, err
	}
	leaseExpiry = 0
	if !lease.ExpiresAt.IsZero() {
		leaseExpiry = lease.ExpiresAt.UnixMilli()
	}
	_, err = tx.ExecContext(ctx, "UPDATE agent_sessions SET state_json=?,expires_at=?,lease_token=?,lease_expires_at=? WHERE id=?", b, v.UpdatedAt.Add(ttl).UnixMilli(), lease.Token, leaseExpiry, id)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(); err != nil {
		return Session{}, err
	}
	return v, nil
}
