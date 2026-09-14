package agent

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func createSession(t *testing.T, store *SessionStore, trusted TrustedContext) Session {
	t.Helper()
	v, err := store.Create(ContextEnvelope{Trusted: trusted})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSessionStoreBindsIdentity(t *testing.T) {
	store := NewSessionStore()
	first := createSession(t, store, TrustedContext{UserID: 1, TenantID: 2, AppID: 3})
	for _, trusted := range []TrustedContext{{UserID: 9, TenantID: 2, AppID: 3}, {UserID: 1, TenantID: 9, AppID: 3}, {UserID: 1, TenantID: 2, AppID: 9}} {
		if _, err := store.UpdateContext(first.ID, ContextEnvelope{Trusted: trusted}); !errors.Is(err, ErrSessionIdentity) {
			t.Fatalf("identity mutation allowed: %v", err)
		}
		if _, _, err := store.BeginTurn(first.ID, trusted); !errors.Is(err, ErrSessionIdentity) {
			t.Fatalf("cross-owner turn allowed: %v", err)
		}
	}
	updated, err := store.UpdateContext(first.ID, ContextEnvelope{Trusted: first.Trusted, Page: PageContext{Route: "/dashboard"}})
	if err != nil || updated.Context.Page.Route != "/dashboard" {
		t.Fatalf("context update failed: %+v %v", updated, err)
	}
}

func TestSessionStoreListsOwnerSummaries(t *testing.T) {
	store := NewSessionStore()
	owner := TrustedContext{UserID: 7, TenantID: 8, AppID: 9}
	first, err := store.Create(ContextEnvelope{Trusted: owner, Page: PageContext{Title: "用户中心"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ContextEnvelope{Trusted: TrustedContext{UserID: 99, TenantID: 8, AppID: 9}, Page: PageContext{Title: "其他"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = second
	items, err := store.List(owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != first.ID || items[0].Title != "用户中心" {
		t.Fatalf("unexpected summaries: %#v", items)
	}
}

func TestSessionStoreExpiresAndCapsEntries(t *testing.T) {
	store := NewSessionStore()
	store.ttl = time.Millisecond
	first := createSession(t, store, TrustedContext{UserID: 1})
	time.Sleep(3 * time.Millisecond)
	if _, err := store.Get(first.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("expired session remained available")
	}
	store.ttl = time.Hour
	store.maxSessions = 1
	old := createSession(t, store, TrustedContext{UserID: 2})
	time.Sleep(time.Millisecond)
	newer := createSession(t, store, TrustedContext{UserID: 3})
	if _, err := store.Get(old.ID); err == nil {
		t.Fatal("old session was not evicted at capacity")
	}
	if _, err := store.Get(newer.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSessionTurnLeaseAndAtomicHistory(t *testing.T) {
	store := NewSessionStore()
	v := createSession(t, store, TrustedContext{UserID: 1, TenantID: 2, AppID: 3})
	_, lease, err := store.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, e := store.BeginTurn(v.ID, v.Trusted); !errors.Is(e, ErrSessionBusy) {
				t.Errorf("concurrent turn accepted: %v", e)
			}
		}()
	}
	wg.Wait()
	if err = store.CompleteTurn(v.ID, "wrong", []Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hello"}}); !errors.Is(err, ErrTurnLeaseLost) {
		t.Fatalf("wrong lease accepted: %v", err)
	}
	if err = store.AbortTurn(v.ID, lease); err != nil {
		t.Fatal(err)
	}
	v, lease, err = store.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.History) != 0 {
		t.Fatal("aborted request persisted history")
	}
	history := []Message{{Role: "user", Content: "Remember orchard"}, {Role: "assistant", Content: "I will remember orchard"}}
	if err = store.CompleteTurn(v.ID, lease, history); err != nil {
		t.Fatal(err)
	}
	history[0].Content = "mutated"
	v, lease, err = store.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.History) != 2 || v.History[0].Content != "Remember orchard" {
		t.Fatalf("history missing or aliases caller: %+v", v.History)
	}
	v.History[0].Content = "caller mutation"
	persisted, err := store.Get(v.ID)
	if err != nil || persisted.History[0].Content != "Remember orchard" {
		t.Fatal("read mutates persisted transcript")
	}
	if err = store.AbortTurn(v.ID, lease); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryTrimsWholeTurnsAndKeepsToolPairs(t *testing.T) {
	history := []Message{}
	for i := 0; i < 25; i++ {
		history = append(history, Message{Role: "user", Content: fmt.Sprint(i)}, Message{Role: "assistant", ToolCalls: []FunctionCall{{ID: fmt.Sprint(i), Type: "function", Function: FunctionSpec{Name: "lookup", Arguments: "{}"}}}}, Message{Role: "tool", ToolCallID: fmt.Sprint(i), Content: "found"}, Message{Role: "assistant", Content: "done"})
	}
	bounded, err := boundedHistory(history)
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded) != 80 || bounded[0].Content != "5" || bounded[1].ToolCalls[0].ID != bounded[2].ToolCallID {
		t.Fatalf("turn was split: %+v", bounded)
	}
	if _, err = boundedHistory([]Message{{Role: "user", Content: "incomplete"}}); err == nil {
		t.Fatal("incomplete turn accepted")
	}
	if _, err = boundedHistory([]Message{{Role: "user", Content: "large"}, {Role: "assistant", Content: strings.Repeat("x", maxHistoryBytes)}}); err == nil {
		t.Fatal("oversize single turn accepted")
	}
}

func TestExpiredLeaseCannotOverwriteNewTurn(t *testing.T) {
	store := NewSessionStore()
	v := createSession(t, store, TrustedContext{UserID: 1})
	_, old, err := store.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.leases[v.ID] = turnLease{Token: old, ExpiresAt: time.Now().Add(-time.Second)}
	store.mu.Unlock()
	_, fresh, err := store.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AbortTurn(v.ID, old); !errors.Is(err, ErrTurnLeaseLost) {
		t.Fatalf("stale request canceled new turn: %v", err)
	}
	if err = store.CompleteTurn(v.ID, old, []Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "old"}}); !errors.Is(err, ErrTurnLeaseLost) {
		t.Fatalf("stale result committed: %v", err)
	}
	if err = store.CompleteTurn(v.ID, fresh, []Message{{Role: "user", Content: "new"}, {Role: "assistant", Content: "new"}}); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionVersionClearsOldToolContext(t *testing.T) {
	store := NewSessionStore()
	trusted := TrustedContext{UserID: 1, PermissionVersion: "v1"}
	v := createSession(t, store, trusted)
	_, lease, err := store.BeginTurn(v.ID, trusted)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CompleteTurn(v.ID, lease, []Message{{Role: "user", Content: "private"}, {Role: "assistant", Content: "private"}}); err != nil {
		t.Fatal(err)
	}
	trusted.PermissionVersion = "v2"
	v, _, err = store.BeginTurn(v.ID, trusted)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.History) != 0 {
		t.Fatal("revoked permission history retained")
	}
}

func TestPermissionChangeCannotOverwriteActiveTurn(t *testing.T) {
	store := NewSessionStore()
	trusted := TrustedContext{UserID: 1, TenantID: 2, AppID: 3, PermissionVersion: "v1"}
	v := createSession(t, store, trusted)
	_, lease, err := store.BeginTurn(v.ID, trusted)
	if err != nil {
		t.Fatal(err)
	}
	newTrusted := trusted
	newTrusted.PermissionVersion = "v2"
	if _, err = store.UpdateContext(v.ID, ContextEnvelope{Trusted: newTrusted}); !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("active permission change accepted: %v", err)
	}
	if err = store.CompleteTurn(v.ID, lease, []Message{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}}); err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateContext(v.ID, ContextEnvelope{Trusted: newTrusted})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.History) != 0 || updated.PermissionVersion != "v2" {
		t.Fatal("permission change retained stale history")
	}
}
