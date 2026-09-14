package agent

import (
	"context"
	"errors"
	"os"
	"testing"
)

// KERTHUS_AGENT_TEST_DSN opts into a real MySQL persistence test. Only random
// test session IDs are inserted, then removed; application records are untouched.
func TestMySQLSessionSurvivesReopenAndSharesLease(t *testing.T) {
	dsn := os.Getenv("KERTHUS_AGENT_TEST_DSN")
	if dsn == "" {
		t.Skip("set KERTHUS_AGENT_TEST_DSN to run durable MySQL history test")
	}
	store, err := NewMySQLSessionStore(dsn)
	if err != nil {
		t.Fatalf("apply migration 010 first: %v", err)
	}
	defer store.Close()
	v := createSession(t, store, TrustedContext{UserID: 9911, TenantID: 9922, AppID: 9933})
	defer store.backend.db.ExecContext(context.Background(), "DELETE FROM agent_sessions WHERE id=?", v.ID)
	_, lease, err := store.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	history := []Message{{Role: "user", Content: "Remember sapphire"}, {Role: "assistant", ToolCalls: []FunctionCall{{ID: "call-1", Type: "function", Function: FunctionSpec{Name: "lookup", Arguments: `{"name":"sapphire"}`}}}}, {Role: "tool", ToolCallID: "call-1", Content: `{"found":true}`}, {Role: "assistant", Content: "I remember sapphire"}}
	if err = store.CompleteTurn(v.ID, lease, history); err != nil {
		t.Fatal(err)
	}
	other, err := NewMySQLSessionStore(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	restored, lease, err := other.BeginTurn(v.ID, v.Trusted)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.History) != 4 || restored.History[0].Content != "Remember sapphire" || restored.History[1].ToolCalls[0].ID != restored.History[2].ToolCallID {
		t.Fatalf("lost protocol after reopen: %+v", restored.History)
	}
	if _, _, err = store.BeginTurn(v.ID, v.Trusted); !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("separate instance bypassed lease: %v", err)
	}
	if _, _, err = store.BeginTurn(v.ID, TrustedContext{UserID: 9911, TenantID: 7777, AppID: 9933}); !errors.Is(err, ErrSessionIdentity) {
		t.Fatalf("tenant isolation failed: %v", err)
	}
	if err = other.AbortTurn(v.ID, lease); err != nil {
		t.Fatal(err)
	}
}
