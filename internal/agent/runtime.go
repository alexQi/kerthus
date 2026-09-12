package agent

import (
	"context"
	"errors"
	"os"
	"strings"

	micro "go-micro.dev/v6"
	"go-micro.dev/v6/agent"
)

var ErrProviderNotConfigured = errors.New("agent provider is not configured")

type RuntimeConfig struct {
	Name     string
	Provider string
	Model    string
	APIKey   string
	Prompt   string
	Services []string
	MaxSteps int
}

type Runtime struct {
	agent micro.Agent
}

func NewRuntime(c RuntimeConfig, tools ...micro.AgentOption) (*Runtime, error) {
	provider := strings.TrimSpace(c.Provider)
	if provider == "" {
		provider = strings.TrimSpace(os.Getenv("KERTHUS_AGENT_PROVIDER"))
	}
	if provider == "" {
		return nil, ErrProviderNotConfigured
	}
	name := c.Name
	if name == "" {
		name = "kerthus-agent"
	}
	maxSteps := c.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 8
	}
	opts := []micro.AgentOption{
		micro.AgentProvider(provider),
		micro.AgentMaxSteps(maxSteps),
		micro.AgentLoopLimit(3),
	}
	if c.Model != "" {
		opts = append(opts, micro.AgentModel(c.Model))
	}
	key := c.APIKey
	if key == "" {
		key = os.Getenv("KERTHUS_AGENT_API_KEY")
	}
	if key != "" {
		opts = append(opts, micro.AgentAPIKey(key))
	}
	if c.Prompt != "" {
		opts = append(opts, micro.AgentPrompt(c.Prompt))
	}
	if len(c.Services) > 0 {
		opts = append(opts, micro.AgentServices(c.Services...))
	}
	opts = append(opts, tools...)
	return &Runtime{agent: micro.NewAgent(name, opts...)}, nil
}

func (r *Runtime) Start() error { return r.agent.Run() }
func (r *Runtime) Stop() error  { return r.agent.Stop() }

func (r *Runtime) Ask(ctx context.Context, message string) (*agent.Response, error) {
	return r.agent.Ask(ctx, message)
}

func (r *Runtime) Stream(ctx context.Context, message string) (agent.AgentStream, error) {
	return agent.StreamAsk(ctx, r.agent, message)
}
