package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const agentProbeValidSSE = "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":null}]}\n\n" +
	"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
	"data: [DONE]\n\n"

func TestAgentModelProbeValidatesCompletedReply(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		valid  bool
	}{
		{"stream", 200, agentProbeValidSSE, true},
		{"json fallback", 200, `{"choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`, true},
		{"empty 200", 200, "", false},
		{"http failure", 500, `{"error":{"message":"probe-secret"}}`, false},
		{"200 error json", 200, `{"error":{"message":"probe-secret"}}`, false},
		{"sse error object", 200, "data: {\"error\":{\"message\":\"probe-secret\"}}\n\n", false},
		{"sse error event", 200, "event: error\ndata: probe-secret\n\n", false},
		{"partial json", 200, `{"choices":[{"message":{"role":"assistant","content":"OK"`, false},
		{"invalid trailing json", 200, `{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]} garbage`, false},
		{"json empty content", 200, `{"choices":[{"message":{"role":"assistant","content":" "},"finish_reason":"stop"}]}`, false},
		{"json missing finish", 200, `{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`, false},
		{"json truncated output", 200, `{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"length"}]}`, false},
		{"only done", 200, "data: [DONE]\n\n", false},
		{"missing done", 200, strings.TrimSuffix(agentProbeValidSSE, "data: [DONE]\n\n"), false},
		{"missing finish", 200, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n", false},
		{"truncated stream", 200, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"", false},
		{"empty stream reply", 200, strings.ReplaceAll(agentProbeValidSSE, `"content":"OK"`, `"content":" "`), false},
		{"truncated model output", 200, strings.ReplaceAll(agentProbeValidSSE, `"stop"`, `"length"`), false},
		{"reasoning without answer", 200, strings.ReplaceAll(agentProbeValidSSE, `"content":"OK"`, `"reasoning_content":"OK"`), false},
		{"oversized json", 200, `{"choices":[{"message":{"role":"assistant","content":"` + strings.Repeat("x", agentProbeMaxBody) + `"},"finish_reason":"stop"}]}`, false},
		{"oversized sse frame", 200, "data: " + strings.Repeat("x", 70<<10) + "\n\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer probe-secret" {
					t.Error("probe did not send authenticated POST")
				}
				var body struct {
					Model     string `json:"model"`
					Stream    bool   `json:"stream"`
					MaxTokens int    `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "test-model" || !body.Stream || body.MaxTokens < 64 {
					t.Errorf("unexpected probe request: %+v, %v", body, err)
				}
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			endpoint, _ := url.Parse(server.URL)
			result, err := probeAgentModel(context.Background(), server.Client(), endpoint, "test-model", "probe-secret")
			if tc.valid {
				if err != nil || !result.OK || result.Model != "test-model" || result.OutputCharacters != 2 {
					t.Fatalf("complete reply rejected: %+v, %v", result, err)
				}
			} else if err == nil || result.OK {
				t.Fatalf("invalid reply accepted: %+v, %v", result, err)
			}
			if err != nil && strings.Contains(err.Error(), "probe-secret") {
				t.Fatal("upstream error leaked sensitive response contents")
			}
		})
	}
}

func TestAgentModelProbeWaitsForReplyAfterHeaders(t *testing.T) {
	headersSent := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(headersSent)
		select {
		case <-release:
			_, _ = fmt.Fprint(w, agentProbeValidSSE)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := probeAgentModel(ctx, server.Client(), endpoint, "test-model", "probe-secret")
		done <- err
	}()
	<-headersSent
	select {
	case err := <-done:
		t.Fatalf("probe returned on HTTP headers without model output: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("probe failed after complete reply: %v", err)
	}
}

func TestAgentModelProbeDeadlineDuringStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result, err := probeAgentModel(ctx, server.Client(), endpoint, "test-model", "probe-secret")
	if err == nil || result.OK || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("stalled model should time out explicitly: %+v, %v", result, err)
	}
}

func TestAgentModelProbeDoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		_, _ = fmt.Fprint(w, agentProbeValidSSE)
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	if result, err := probeAgentModel(context.Background(), server.Client(), endpoint, "test-model", "probe-secret"); err == nil || result.OK {
		t.Fatalf("redirect accepted: %+v, %v", result, err)
	}
	if redirected.Load() != 0 {
		t.Fatal("probe followed redirect and could replay provider credentials")
	}
}
