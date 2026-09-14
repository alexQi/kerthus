package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"time"

	microagent "go-micro.dev/v6/agent"
	"go-micro.dev/v6/ai"
)

const maxRoundBytes = 4 << 20

const maxUpstreamRetries = 5

var upstreamRetryDelays = [...]time.Duration{
	5 * time.Second,
	10 * time.Second,
	10 * time.Second,
	10 * time.Second,
	10 * time.Second,
}

func (r *Runtime) run(ctx context.Context, prompt string, history []Message, emit func(*microagent.StreamEvent) error) ([]Message, error) {
	messages := append(append([]Message(nil), history...), Message{Role: "user", Content: prompt})
	repeats := map[string]int{}
	steps := 0
	for round := 0; round <= r.config.MaxSteps; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		response, err := r.round(ctx, messages, emit)
		if err != nil {
			return nil, err
		}
		messages = append(messages, response)
		if len(response.ToolCalls) == 0 {
			return messages, nil
		}
		for _, call := range response.ToolCalls {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if steps >= r.config.MaxSteps {
				return nil, errors.New("Agent 工具调用次数达到上限")
			}
			var input map[string]any
			if json.Unmarshal([]byte(call.Function.Arguments), &input) != nil || input == nil {
				return nil, errors.New("模型返回的工具参数不是有效 JSON 对象")
			}
			canonical, _ := json.Marshal(input)
			signature := call.Function.Name + string(canonical)
			repeats[signature]++
			if repeats[signature] > 3 {
				return nil, errors.New("Agent 重复调用相同工具，已停止")
			}
			steps++
			tc := ai.ToolCall{ID: call.ID, Name: call.Function.Name, Input: input}
			if err := emit(&microagent.StreamEvent{Type: microagent.StreamEventToolStart, ToolCall: tc}); err != nil {
				return nil, err
			}
			result := ai.ToolResult{ID: call.ID}
			if tool, ok := r.tools[call.Function.Name]; ok {
				toolCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				value, toolErr := tool.Handler(toolCtx, input)
				cancel()
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if toolErr != nil {
					result.Content = "工具执行失败，请检查权限或请求参数后重试"
				} else {
					result.Content = value
				}
			} else {
				result.Content = "当前用户无权调用该工具"
				result.Refused = ai.RefusedApproval
			}
			if len(result.Content) > 64<<10 {
				result.Content = "工具结果过大，请增加查询筛选条件或分页"
			}
			messages = append(messages, Message{Role: "tool", Content: result.Content, ToolCallID: call.ID})
			if err := emit(&microagent.StreamEvent{Type: microagent.StreamEventToolEnd, ToolCall: tc, Result: result}); err != nil {
				return nil, err
			}
		}
	}
	return nil, errors.New("Agent 工具调用轮数达到上限")
}

func (r *Runtime) round(ctx context.Context, history []Message, emit func(*microagent.StreamEvent) error) (Message, error) {
	messages := append([]Message{{Role: "system", Content: r.config.Prompt}}, history...)
	request := map[string]any{"model": r.config.Model, "messages": messages, "stream": true}
	if len(r.definitions) > 0 {
		request["tools"] = r.definitions
	}
	data, err := json.Marshal(request)
	if err != nil {
		return Message{}, errors.New("无法编码模型请求")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(data))
	if err != nil {
		return Message{}, errors.New("无法创建模型请求")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	var lastErr error
	for attempt := 0; attempt <= maxUpstreamRetries; attempt++ {
		if attempt > 0 && req.GetBody != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return Message{}, errors.New("无法重试模型请求")
			}
			req.Body = body
		}
		resp, requestErr := r.client.Do(req)
		if requestErr != nil {
			if ctx.Err() != nil {
				return Message{}, ctx.Err()
			}
			lastErr = errors.New("无法连接模型服务")
			if attempt == maxUpstreamRetries {
				return Message{}, lastErr
			}
			if err := waitUpstreamRetry(ctx, attempt); err != nil {
				return Message{}, err
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("模型服务返回 HTTP %d: %s", resp.StatusCode, readModelError(resp.Body))
			_ = resp.Body.Close()
			if !retryableUpstreamStatus(resp.StatusCode) || attempt == maxUpstreamRetries {
				return Message{}, lastErr
			}
			if err := waitUpstreamRetry(ctx, attempt); err != nil {
				return Message{}, err
			}
			continue
		}

		media, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if media != "text/event-stream" {
			lastErr = fmt.Errorf("模型服务未返回 SSE 流: %s", readModelError(resp.Body))
			_ = resp.Body.Close()
			return Message{}, lastErr
		}
		emitted := false
		emitAttempt := func(event *microagent.StreamEvent) error {
			if event != nil && event.Type == microagent.StreamEventToken {
				emitted = true
			}
			return emit(event)
		}
		result, streamErr := readCompletion(resp.Body, emitAttempt)
		_ = resp.Body.Close()
		if streamErr != nil && !emitted && retryableStreamError(streamErr) && attempt < maxUpstreamRetries {
			lastErr = streamErr
			if err := waitUpstreamRetry(ctx, attempt); err != nil {
				return Message{}, err
			}
			continue
		}
		return result, streamErr
	}
	return Message{}, lastErr
}

func retryableUpstreamStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func retryableStreamError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.HasPrefix(message, "模型服务返回流式错误:") ||
		strings.HasPrefix(message, "模型服务返回错误:") ||
		message == "模型流式连接中断"
}

func waitUpstreamRetry(ctx context.Context, attempt int) error {
	delay := upstreamRetryDelays[len(upstreamRetryDelays)-1]
	if attempt >= 0 && attempt < len(upstreamRetryDelays) {
		delay = upstreamRetryDelays[attempt]
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type completionChunk struct {
	Error   json.RawMessage `json:"error"`
	Choices []struct {
		Index        int     `json:"index"`
		FinishReason *string `json:"finish_reason"`
		Delta        struct {
			Content   string `json:"content"`
			Refusal   string `json:"refusal"`
			ToolCalls []struct {
				Index    int          `json:"index"`
				ID       string       `json:"id"`
				Type     string       `json:"type"`
				Function FunctionSpec `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// readModelError keeps only bounded, non-sensitive classification data from an
// upstream error. Provider messages may contain credentials, prompts, or
// account identifiers, so they are deliberately never returned to the UI.
func readModelError(body io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(body, 4096))
	if len(raw) == 0 {
		return "上游未提供错误详情"
	}
	var envelope struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) == nil {
		if envelope.Error.Type != "" {
			return "上游错误类型: " + truncateModelError(envelope.Error.Type)
		}
		if envelope.Error.Code != nil {
			return "上游错误代码: " + truncateModelError(fmt.Sprint(envelope.Error.Code))
		}
		if envelope.Error.Message != "" || envelope.Message != "" {
			return "上游返回了错误详情"
		}
	}
	return "上游返回了不可识别的错误详情"
}

func truncateModelError(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 512 {
		return value[:512] + "…"
	}
	return value
}

// readCompletion assembles only tool arguments; answer tokens are forwarded
// immediately. Both a finish reason and the terminal frame are required so a
// truncated stream can never be persisted as a successful assistant turn.
func readCompletion(body io.Reader, emit func(*microagent.StreamEvent) error) (Message, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, maxRoundBytes+1))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	response := Message{Role: "assistant"}
	calls := map[int]*FunctionCall{}
	var text strings.Builder
	var data []string
	eventType, finish := "", ""
	total := 0
	process := func() (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		raw := strings.Join(data, "\n")
		data = nil
		if eventType == "error" {
			return false, fmt.Errorf("模型服务返回流式错误: %s", readModelError(strings.NewReader(raw)))
		}
		if strings.TrimSpace(raw) == "[DONE]" {
			return true, nil
		}
		var chunk completionChunk
		if json.Unmarshal([]byte(raw), &chunk) != nil {
			return false, errors.New("模型流式响应格式错误")
		}
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return false, fmt.Errorf("模型服务返回错误: %s", readModelError(bytes.NewReader(chunk.Error)))
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.Delta.Refusal != "" {
				return false, errors.New("模型拒绝了本次请求")
			}
			if finish != "" && (choice.Delta.Content != "" || len(choice.Delta.ToolCalls) > 0) {
				return false, errors.New("模型在结束后继续返回内容")
			}
			if choice.Delta.Content != "" {
				text.WriteString(choice.Delta.Content)
				if err := emit(&microagent.StreamEvent{Type: microagent.StreamEventToken, Token: choice.Delta.Content}); err != nil {
					return false, err
				}
			}
			for _, part := range choice.Delta.ToolCalls {
				if part.Index < 0 || part.Index >= 32 {
					return false, errors.New("模型工具调用数量异常")
				}
				call := calls[part.Index]
				if call == nil {
					call = &FunctionCall{}
					calls[part.Index] = call
				}
				if part.ID != "" {
					if call.ID != "" && call.ID != part.ID {
						return false, errors.New("工具调用标识不一致")
					}
					call.ID = part.ID
				}
				if part.Type != "" {
					call.Type = part.Type
				}
				call.Function.Name += part.Function.Name
				call.Function.Arguments += part.Function.Arguments
				if len(call.Function.Arguments) > 64<<10 || len(call.Function.Name) > 64 {
					return false, errors.New("模型工具参数过大")
				}
			}
			if choice.FinishReason != nil {
				finish = *choice.FinishReason
			}
		}
		return false, nil
	}
	done := false
	for scanner.Scan() {
		line := scanner.Text()
		total += len(line) + 1
		if total > maxRoundBytes {
			return Message{}, errors.New("模型流式响应过大")
		}
		if line == "" {
			var err error
			done, err = process()
			if err != nil {
				return Message{}, err
			}
			eventType = ""
			if done {
				break
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		key, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		if key == "event" {
			eventType = value
		}
		if key == "data" {
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return Message{}, errors.New("模型流式连接中断")
	}
	if !done {
		var err error
		done, err = process()
		if err != nil {
			return Message{}, err
		}
	}
	if !done || finish == "" {
		return Message{}, errors.New("模型流式响应未完整结束")
	}
	response.Content = text.String()
	if len(calls) == 0 {
		if finish != "stop" {
			return Message{}, fmt.Errorf("模型未正常完成回答（%s）", safeFinish(finish))
		}
		if strings.TrimSpace(response.Content) == "" {
			return Message{}, errors.New("模型未返回内容")
		}
		return response, nil
	}
	if finish != "tool_calls" {
		return Message{}, errors.New("模型工具调用未完整结束")
	}
	indexes := make([]int, 0, len(calls))
	for index := range calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	ids := map[string]bool{}
	for _, index := range indexes {
		call := calls[index]
		var args map[string]any
		if call.ID == "" || ids[call.ID] || call.Type != "function" || call.Function.Name == "" || json.Unmarshal([]byte(call.Function.Arguments), &args) != nil || args == nil {
			return Message{}, errors.New("模型工具调用不完整")
		}
		ids[call.ID] = true
		response.ToolCalls = append(response.ToolCalls, *call)
	}
	return response, nil
}
func safeFinish(reason string) string {
	switch reason {
	case "length":
		return "达到输出上限"
	case "content_filter":
		return "内容过滤"
	default:
		return "异常结束"
	}
}
