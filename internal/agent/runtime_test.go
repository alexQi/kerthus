package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	microagent "go-micro.dev/v6/agent"
	"go-micro.dev/v6/ai"
)

func sendChunk(w http.ResponseWriter, content string) {
	data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": content}}}})
	fmt.Fprintf(w, "data: %s\n\n", data)
	w.(http.Flusher).Flush()
}
func finishRound(w http.ResponseWriter, reason string) {
	fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":%q}]}\n\ndata: [DONE]\n\n", reason)
	w.(http.Flusher).Flush()
}
func testRuntime(t *testing.T, url string, tools ...Tool) *Runtime {
	t.Helper()
	r, err := NewRuntime(RuntimeConfig{Provider: "openai", Model: "test", BaseURL: url, APIKey: "test-key", Prompt: "test system"}, tools...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func nextEvent(t *testing.T, s *Stream) *microagent.StreamEvent {
	t.Helper()
	type result struct {
		event *microagent.StreamEvent
		err   error
	}
	ch := make(chan result, 1)
	go func() { e, err := s.Recv(); ch <- result{e, err} }()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.event
	case <-time.After(2 * time.Second):
		t.Fatal("event buffered until provider completes")
		return nil
	}
}

func TestRuntimeStreamsToolsAndPersistsProtocolAcrossFreshRuntime(t *testing.T) {
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests, executions atomic.Int32
	var captured []Message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream   bool      `json:"stream"`
			Messages []Message `json:"messages"`
			Tools    []any     `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if !body.Stream || len(body.Tools) != 1 {
			t.Error("every round must stream and include permission-filtered tools")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		switch requests.Add(1) {
		case 1:
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_record\",\"arguments\":\"{\\\"id\\\":\"}}]}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"7}\"}}]}}]}\n\n")
			finishRound(w, "tool_calls")
		case 2:
			if len(body.Messages) != 4 || body.Messages[2].ToolCalls[0].ID != "call_1" || body.Messages[3].ToolCallID != "call_1" {
				t.Errorf("missing call/result correlation: %#v", body.Messages)
			}
			sendChunk(w, "# 结果\n\n")
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			sendChunk(w, "- **七**\n")
			finishRound(w, "stop")
		case 3:
			captured = body.Messages
			sendChunk(w, "记得：七")
			finishRound(w, "stop")
		default:
			t.Error("unexpected model retry")
		}
	}))
	defer server.Close()
	tool := Tool{Definition: ai.Tool{Name: "read_record", Properties: map[string]any{}}, Handler: func(ctx context.Context, input map[string]any) (string, error) {
		executions.Add(1)
		if input["id"] != float64(7) {
			t.Errorf("fragmented arguments not assembled: %#v", input)
		}
		return `{"name":"七"}`, nil
	}}
	s, err := testRuntime(t, server.URL, tool).Stream(ctx, "查 7")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if e := nextEvent(t, s); e.Type != microagent.StreamEventToolStart {
		t.Fatal(e)
	}
	if e := nextEvent(t, s); e.Type != microagent.StreamEventToolEnd {
		t.Fatal(e)
	}
	// The upstream handler has not sent finish_reason or [DONE] yet.
	if e := nextEvent(t, s); e.Type != microagent.StreamEventToken || e.Token != "# 结果\n\n" {
		t.Fatal(e)
	}
	close(release)
	if e := nextEvent(t, s); e.Token != "- **七**\n" {
		t.Fatal(e)
	}
	if e := nextEvent(t, s); e.Type != microagent.StreamEventDone || e.Response.Reply != "# 结果\n\n- **七**\n" {
		t.Fatal(e)
	}
	history := s.History()
	if len(history) != 4 {
		t.Fatalf("history: %#v", history)
	}
	// A new runtime must receive prior messages from the durable session.
	next, err := testRuntime(t, server.URL, tool).Stream(ctx, "刚才叫什么", history)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	for {
		e := nextEvent(t, next)
		if e.Type == microagent.StreamEventDone {
			break
		}
	}
	if len(captured) != 6 || captured[1].Content != "查 7" || captured[4].Content != "# 结果\n\n- **七**\n" || captured[5].Content != "刚才叫什么" {
		t.Fatalf("lost multi-turn context: %#v", captured)
	}
	if executions.Load() != 1 {
		t.Fatal("historical tool call was replayed")
	}
}

func TestRuntimeRejectsEmptyErrorAndTruncatedStreams(t *testing.T) {
	tests := map[string]string{
		"empty":               "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n",
		"error":               "data: {\"error\":{\"message\":\"upstream-secret\"}}\n\n",
		"event error":         "event: error\ndata: {\"message\":\"upstream-secret\"}\n\n",
		"truncated":           "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n",
		"length":              "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n",
		"malformed arguments": "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"1\",\"type\":\"function\",\"function\":{\"name\":\"write\",\"arguments\":\"{\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n",
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, data)
			}))
			defer srv.Close()
			s, err := testRuntime(t, srv.URL).Stream(context.Background(), "hi")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			for {
				e, err := s.Recv()
				if err != nil {
					if err == io.EOF || strings.Contains(err.Error(), "upstream-secret") {
						t.Fatalf("bad error: %v", err)
					}
					break
				}
				if e.Type == microagent.StreamEventDone {
					t.Fatal("failed stream reported done")
				}
			}
			if len(s.History()) != 0 {
				t.Fatal("failed answer committed to history")
			}
		})
	}
}

func TestRuntimeCancelClosesUpstream(t *testing.T) {
	closed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sendChunk(w, "first")
		<-r.Context().Done()
		close(closed)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := testRuntime(t, srv.URL).Stream(ctx, "hi")
	if err != nil {
		t.Fatal(err)
	}
	nextEvent(t, s)
	cancel()
	s.Close()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("upstream not canceled")
	}
	if len(s.History()) > 0 {
		t.Fatal("canceled turn was committed")
	}
}
