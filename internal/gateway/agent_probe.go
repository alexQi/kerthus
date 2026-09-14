package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	agentProbeTimeout = 45 * time.Second
	agentProbeMaxBody = 1 << 20
)

type agentModelProbeResult struct {
	OK               bool   `json:"ok"`
	Model            string `json:"model"`
	LatencyMS        int64  `json:"latency_ms"`
	OutputCharacters int    `json:"output_characters"`
}

// A successful HTTP handshake alone does not prove that a model can answer.
// Keep the request alive until a complete, nonempty assistant reply arrives.
// Errors intentionally omit the upstream body, URL and transport error because
// a provider can echo submitted credentials in any of those values.
func probeAgentModel(ctx context.Context, client *http.Client, endpoint *url.URL, model, apiKey string) (agentModelProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, agentProbeTimeout)
	defer cancel()
	started := time.Now()
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with exactly OK."}},
		"max_tokens": 512,
		"stream":     true,
	})
	if err != nil {
		return agentModelProbeResult{}, errors.New("无法创建模型测试请求")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return agentModelProbeResult{}, errors.New("无法创建模型测试请求")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	// Provider credentials must never be replayed to a redirect destination.
	probeClient := *client
	probeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := probeClient.Do(req)
	if err != nil {
		return agentModelProbeResult{}, agentProbeReadError(ctx, "无法连接模型服务")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return agentModelProbeResult{}, errors.New("模型接口返回错误，连通性测试失败")
	}
	limited := &io.LimitedReader{R: resp.Body, N: agentProbeMaxBody + 1}
	reader := bufio.NewReader(limited)
	// Some compatible endpoints omit the content type or return JSON despite
	// stream=true. Sniff a JSON object while still enforcing the body limit.
	for {
		prefix, peekErr := reader.Peek(1)
		if peekErr != nil {
			return agentModelProbeResult{}, agentProbeReadError(ctx, "模型没有返回完整回复")
		}
		if !strings.ContainsRune(" \t\r\n", rune(prefix[0])) {
			break
		}
		_, _ = reader.ReadByte()
	}
	prefix, _ := reader.Peek(1)
	var text string
	if prefix[0] == '{' {
		text, err = readAgentProbeJSON(reader)
	} else {
		text, err = readAgentProbeSSE(reader)
	}
	if ctx.Err() != nil {
		return agentModelProbeResult{}, agentProbeReadError(ctx, "模型没有返回完整回复")
	}
	if limited.N <= 0 {
		return agentModelProbeResult{}, errors.New("模型测试响应超过大小限制")
	}
	if err != nil {
		return agentModelProbeResult{}, err
	}
	return agentModelProbeResult{OK: true, Model: model, LatencyMS: time.Since(started).Milliseconds(), OutputCharacters: utf8.RuneCountInString(text)}, nil
}

func agentProbeReadError(ctx context.Context, fallback string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("模型测试超时，未收到完整回复")
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return errors.New("模型测试已取消")
	}
	return errors.New(fallback)
}

type agentProbeCompletion struct {
	Error   json.RawMessage `json:"error"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func decodeAgentProbeCompletion(data []byte) (agentProbeCompletion, error) {
	var completion agentProbeCompletion
	if err := json.Unmarshal(data, &completion); err != nil {
		return completion, errors.New("模型响应格式错误")
	}
	if len(completion.Error) != 0 && string(completion.Error) != "null" {
		return completion, errors.New("模型返回错误，连通性测试失败")
	}
	return completion, nil
}

func readAgentProbeJSON(reader io.Reader) (string, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", errors.New("模型响应中断，未收到完整回复")
	}
	completion, err := decodeAgentProbeCompletion(body)
	if err != nil {
		return "", err
	}
	for _, choice := range completion.Choices {
		if choice.Index == 0 && choice.Message.Role == "assistant" && choice.FinishReason == "stop" && strings.TrimSpace(choice.Message.Content) != "" {
			return choice.Message.Content, nil
		}
	}
	return "", errors.New("模型没有返回完整的文本回复")
}

func readAgentProbeSSE(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	var output strings.Builder
	var data []string
	var event string
	finished := false
	dispatch := func() (bool, error) {
		if event == "error" {
			return false, errors.New("模型流返回错误，连通性测试失败")
		}
		if len(data) == 0 {
			return false, nil
		}
		value := strings.Join(data, "\n")
		if strings.TrimSpace(value) == "[DONE]" {
			if !finished || strings.TrimSpace(output.String()) == "" {
				return false, errors.New("模型没有返回完整的文本回复")
			}
			return true, nil
		}
		completion, err := decodeAgentProbeCompletion([]byte(value))
		if err != nil {
			return false, err
		}
		for _, choice := range completion.Choices {
			if choice.Index != 0 {
				continue
			}
			if finished && (choice.Delta.Content != "" || choice.FinishReason != "") {
				return false, errors.New("模型流在回复完成后返回了额外内容")
			}
			output.WriteString(choice.Delta.Content)
			if choice.FinishReason != "" {
				if choice.FinishReason != "stop" {
					return false, errors.New("模型回复被截断或未正常完成")
				}
				finished = true
			}
		}
		return false, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := dispatch()
			if err != nil {
				return "", err
			}
			if done {
				return output.String(), nil
			}
			data, event = nil, ""
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			data = append(data, value)
		case "event":
			event = value
		}
	}
	if scanner.Err() != nil {
		return "", errors.New("模型响应中断或超过大小限制")
	}
	// No EOF success: a proxy can close a 200 response halfway through a token.
	// Both a normal finish reason and the protocol terminator must be present.
	return "", errors.New("模型流提前结束，未收到完整回复")
}
