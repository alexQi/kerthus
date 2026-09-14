package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	microagent "go-micro.dev/v6/agent"
	"go-micro.dev/v6/ai"
)

var ErrProviderNotConfigured = errors.New("agent provider is not configured")

// Message retains the complete chat protocol, including tool call correlation.
// It is stored per authenticated session, never in a shared agent-name memory.
type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []FunctionCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}
type FunctionCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}
type FunctionSpec struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Tool struct {
	Definition ai.Tool
	Handler    microagent.ToolFunc
}
type RuntimeConfig struct {
	Name, Provider, Model, BaseURL, APIKey, Prompt string
	MaxSteps                                       int
	HTTPClient                                     *http.Client
}
type Runtime struct {
	config      RuntimeConfig
	tools       map[string]Tool
	definitions []map[string]any
	endpoint    string
	client      *http.Client
}

func NewRuntime(c RuntimeConfig, tools ...Tool) (*Runtime, error) {
	if strings.TrimSpace(c.Provider) == "" {
		return nil, ErrProviderNotConfigured
	}
	if !strings.EqualFold(strings.TrimSpace(c.Provider), "openai") {
		return nil, errors.New("当前服务商不支持工具流式协议")
	}
	if strings.TrimSpace(c.Model) == "" || strings.TrimSpace(c.APIKey) == "" || c.APIKey == "********" {
		return nil, errors.New("模型或 API 密钥未配置")
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("模型接口地址不合法")
	}
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	if c.MaxSteps <= 0 {
		c.MaxSteps = 8
	}
	if c.Name == "" {
		c.Name = "kerthus-agent"
	}
	hc := &http.Client{Timeout: 90 * time.Second}
	if c.HTTPClient != nil {
		*hc = *c.HTTPClient
	}
	// Credentials must never follow a provider redirect to a different endpoint.
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r := &Runtime{config: c, tools: map[string]Tool{}, endpoint: base + "/chat/completions", client: hc}
	for _, tool := range tools {
		name := tool.Definition.Name
		if name == "" || tool.Handler == nil {
			return nil, errors.New("Agent 工具配置不完整")
		}
		if _, exists := r.tools[name]; exists {
			return nil, errors.New("Agent 工具名称重复")
		}
		r.tools[name] = tool
		r.definitions = append(r.definitions, map[string]any{"type": "function", "function": map[string]any{"name": name, "description": tool.Definition.Description, "parameters": map[string]any{"type": "object", "properties": tool.Definition.Properties}}})
	}
	return r, nil
}

// Stream uses upstream token deltas on every round, including rounds with
// tools. go-micro v6.13 StreamAsk buffers Generate then splits the final reply;
// this adapter retains its tool/event contracts without that buffering path.
func (r *Runtime) Stream(ctx context.Context, message string, histories ...[]Message) (*Stream, error) {
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("消息不能为空")
	}
	if len(message) > 32<<10 {
		return nil, errors.New("消息过长")
	}
	var history []Message
	if len(histories) > 0 {
		history = append(history, histories[0]...)
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Stream{events: make(chan *microagent.StreamEvent, 16), done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(s.done)
		defer close(s.events)
		h, err := r.run(ctx, message, history, s.emit(ctx))
		s.mu.Lock()
		s.history, s.err = h, err
		s.mu.Unlock()
		if err == nil {
			var reply strings.Builder
			for _, m := range h[len(history)+1:] {
				if m.Role == "assistant" {
					reply.WriteString(m.Content)
				}
			}
			s.emit(ctx)(&microagent.StreamEvent{Type: microagent.StreamEventDone, Response: &microagent.Response{Reply: reply.String(), Agent: r.config.Name}})
		}
	}()
	return s, nil
}

func (r *Runtime) Ask(ctx context.Context, message string) (*microagent.Response, error) {
	s, err := r.Stream(ctx, message)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	for {
		event, err := s.Recv()
		if err != nil {
			return nil, err
		}
		if event.Type == microagent.StreamEventDone {
			return event.Response, nil
		}
	}
}

type Stream struct {
	events  chan *microagent.StreamEvent
	done    chan struct{}
	cancel  context.CancelFunc
	mu      sync.RWMutex
	history []Message
	err     error
}

var _ microagent.AgentStream = (*Stream)(nil)

func (s *Stream) emit(ctx context.Context) func(*microagent.StreamEvent) error {
	return func(e *microagent.StreamEvent) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s.events <- e:
			return nil
		}
	}
}
func (s *Stream) Recv() (*microagent.StreamEvent, error) {
	if e, ok := <-s.events; ok {
		return e, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return nil, s.err
	}
	return nil, io.EOF
}
func (s *Stream) History() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Message(nil), s.history...)
}
func (s *Stream) Close() error { s.cancel(); <-s.done; return nil }
