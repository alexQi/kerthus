package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrSessionNotFound = errors.New("agent session not found")

type Session struct {
	ID                string          `json:"session_id"`
	Trusted           TrustedContext  `json:"trusted"`
	Context           ContextEnvelope `json:"context"`
	PermissionVersion string          `json:"permission_version,omitempty"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionStore() *SessionStore { return &SessionStore{sessions: make(map[string]Session)} }

func (s *SessionStore) Create(ctx ContextEnvelope) Session {
	now := time.Now().UTC()
	session := Session{ID: newID(), Trusted: ctx.Trusted, Context: ctx, PermissionVersion: ctx.Trusted.PermissionVersion, Status: "active", CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return session
}

func (s *SessionStore) Get(id string) (Session, error) {
	s.mu.RLock()
	v, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return v, nil
}

func (s *SessionStore) UpdateContext(id string, ctx ContextEnvelope) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if v.Trusted != ctx.Trusted {
		return Session{}, errors.New("agent session identity cannot change")
	}
	v.Context, v.UpdatedAt = ctx, time.Now().UTC()
	v.PermissionVersion = ctx.Trusted.PermissionVersion
	s.sessions[id] = v
	return v, nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}
