package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrSessionNotFound = errors.New("agent session not found")
	ErrSessionBusy     = errors.New("agent session is already generating a reply")
	ErrTurnLeaseLost   = errors.New("agent turn lease expired or was replaced")
	ErrSessionIdentity = errors.New("agent session identity cannot change")
)

type SessionSummary struct {
	ID           string    `json:"session_id"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	ID                string          `json:"session_id"`
	Trusted           TrustedContext  `json:"trusted"`
	Context           ContextEnvelope `json:"context"`
	History           []Message       `json:"history"`
	PermissionVersion string          `json:"permission_version,omitempty"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type turnLease struct {
	Token     string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu          sync.Mutex
	sessions    map[string]Session
	leases      map[string]turnLease
	maxSessions int
	ttl         time.Duration
	backend     *mysqlSessionBackend
}

const (
	defaultSessionLimit = 4096
	defaultSessionTTL   = 30 * 24 * time.Hour
	turnLeaseTTL        = 2 * time.Minute
	maxHistoryTurns     = 20
	maxHistoryMessages  = 128
	maxHistoryBytes     = 512 << 10
)

// NewSessionStore creates an in-memory store for isolated tests. The gateway
// executable injects NewMySQLSessionStore so completed turns survive restarts.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]Session), leases: make(map[string]turnLease), maxSessions: defaultSessionLimit, ttl: defaultSessionTTL}
}

func (s *SessionStore) Close() error {
	if s.backend != nil {
		return s.backend.db.Close()
	}
	return nil
}

func (s *SessionStore) Create(ctx ContextEnvelope) (Session, error) {
	now := time.Now().UTC()
	session := Session{ID: newID(), Trusted: ctx.Trusted, Context: ctx, History: []Message{}, PermissionVersion: ctx.Trusted.PermissionVersion, Status: "active", CreatedAt: now, UpdatedAt: now}
	if s.backend != nil {
		return s.backend.create(session, s.ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var oldestID string
	var oldest time.Time
	for id, existing := range s.sessions {
		if now.Sub(existing.UpdatedAt) > s.ttl && !s.leases[id].ExpiresAt.After(now) {
			delete(s.sessions, id)
			delete(s.leases, id)
			continue
		}
		if !s.leases[id].ExpiresAt.After(now) && (oldestID == "" || existing.UpdatedAt.Before(oldest)) {
			oldestID, oldest = id, existing.UpdatedAt
		}
	}
	if len(s.sessions) >= s.maxSessions {
		if oldestID == "" {
			return Session{}, ErrSessionBusy
		}
		delete(s.sessions, oldestID)
		delete(s.leases, oldestID)
	}
	copy, err := cloneSession(session)
	if err != nil {
		return Session{}, err
	}
	s.sessions[session.ID] = copy
	return session, nil
}

func (s *SessionStore) List(owner TrustedContext) ([]SessionSummary, error) {
	if s.backend != nil {
		return s.backend.list(owner, s.ttl)
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SessionSummary, 0)
	for id, existing := range s.sessions {
		if now.Sub(existing.UpdatedAt) > s.ttl {
			delete(s.sessions, id)
			delete(s.leases, id)
			continue
		}
		if !sameSessionOwner(existing.Trusted, owner) {
			continue
		}
		out = append(out, summarizeSession(existing))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func summarizeSession(v Session) SessionSummary {
	title := strings.TrimSpace(v.Context.Page.Title)
	for _, message := range v.History {
		if message.Role == "user" && strings.TrimSpace(message.Content) != "" {
			title = strings.TrimSpace(message.Content)
			break
		}
	}
	if title == "" {
		title = "新对话"
	}
	if len([]rune(title)) > 48 {
		title = string([]rune(title)[:48]) + "…"
	}
	return SessionSummary{ID: v.ID, Title: title, MessageCount: len(v.History), UpdatedAt: v.UpdatedAt}
}

func (s *SessionStore) Get(id string) (Session, error) {
	if s.backend != nil {
		return s.backend.get(id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	if !ok || time.Since(v.UpdatedAt) > s.ttl {
		delete(s.sessions, id)
		delete(s.leases, id)
		return Session{}, ErrSessionNotFound
	}
	return cloneSession(v)
}

// mutate serializes state changes across requests; MySQL uses a row lock so
// separate gateway processes also share the same generation lease.
func (s *SessionStore) mutate(id string, fn func(*Session, *turnLease) error) (Session, error) {
	if s.backend != nil {
		return s.backend.mutate(id, s.ttl, fn)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	if !ok || time.Since(v.UpdatedAt) > s.ttl {
		delete(s.sessions, id)
		delete(s.leases, id)
		return Session{}, ErrSessionNotFound
	}
	copy, err := cloneSession(v)
	if err != nil {
		return Session{}, err
	}
	lease := s.leases[id]
	if err = fn(&copy, &lease); err != nil {
		return Session{}, err
	}
	copy.UpdatedAt = time.Now().UTC()
	s.sessions[id], s.leases[id] = copy, lease
	return cloneSession(copy)
}

func sameSessionOwner(a, b TrustedContext) bool {
	return a.UserID == b.UserID && a.TenantID == b.TenantID && a.AppID == b.AppID
}

func (s *SessionStore) UpdateContext(id string, ctx ContextEnvelope) (Session, error) {
	return s.mutate(id, func(v *Session, lease *turnLease) error {
		if !sameSessionOwner(v.Trusted, ctx.Trusted) {
			return ErrSessionIdentity
		}
		// A new permission version must not reuse cached tool results from
		// permissions that may have been revoked since the previous request.
		if v.PermissionVersion != ctx.Trusted.PermissionVersion {
			if lease.ExpiresAt.After(time.Now()) {
				return ErrSessionBusy
			}
			v.History = []Message{}
		}
		v.Trusted, v.Context, v.PermissionVersion = ctx.Trusted, ctx, ctx.Trusted.PermissionVersion
		return nil
	})
}

func (s *SessionStore) BeginTurn(id string, expected TrustedContext) (Session, string, error) {
	token := newID()
	v, err := s.mutate(id, func(v *Session, lease *turnLease) error {
		if !sameSessionOwner(v.Trusted, expected) {
			return ErrSessionIdentity
		}
		if lease.ExpiresAt.After(time.Now()) {
			return ErrSessionBusy
		}
		if v.PermissionVersion != expected.PermissionVersion {
			v.History = []Message{}
			v.PermissionVersion = expected.PermissionVersion
			v.Trusted = expected
			v.Context.Trusted = expected
		}
		*lease = turnLease{Token: token, ExpiresAt: time.Now().UTC().Add(turnLeaseTTL)}
		return nil
	})
	if err != nil {
		return Session{}, "", err
	}
	return v, token, nil
}

// CompleteTurn commits a complete protocol transcript in one transaction. It
// trims only whole user turns, retaining tool call/result pairs together.
func (s *SessionStore) CompleteTurn(id, token string, history []Message) error {
	bounded, err := boundedHistory(history)
	if err != nil {
		return err
	}
	_, err = s.mutate(id, func(v *Session, lease *turnLease) error {
		if token == "" || lease.Token != token || !lease.ExpiresAt.After(time.Now()) {
			return ErrTurnLeaseLost
		}
		v.History = bounded
		*lease = turnLease{}
		return nil
	})
	return err
}

func (s *SessionStore) AbortTurn(id, token string) error {
	_, err := s.mutate(id, func(_ *Session, lease *turnLease) error {
		if token == "" || lease.Token != token {
			return ErrTurnLeaseLost
		}
		*lease = turnLease{}
		return nil
	})
	return err
}

func boundedHistory(history []Message) ([]Message, error) {
	if len(history) == 0 {
		return []Message{}, nil
	}
	if history[0].Role != "user" || history[len(history)-1].Role != "assistant" || len(history[len(history)-1].ToolCalls) > 0 {
		return nil, errors.New("agent history must contain completed user turns")
	}
	starts := []int{}
	for i, msg := range history {
		if msg.Role == "user" {
			starts = append(starts, i)
		}
	}
	if len(starts) > maxHistoryTurns {
		starts = starts[len(starts)-maxHistoryTurns:]
	}
	for len(starts) > 0 {
		candidate := history[starts[0]:]
		b, err := json.Marshal(candidate)
		if err != nil {
			return nil, err
		}
		if len(candidate) <= maxHistoryMessages && len(b) <= maxHistoryBytes {
			var copy []Message
			if err = json.Unmarshal(b, &copy); err != nil {
				return nil, err
			}
			return copy, nil
		}
		starts = starts[1:]
	}
	return nil, errors.New("agent turn exceeds history storage limit")
}

func cloneSession(v Session) (Session, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Session{}, err
	}
	var copy Session
	err = json.Unmarshal(b, &copy)
	return copy, err
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("agent session random source unavailable")
	}
	return hex.EncodeToString(b)
}
