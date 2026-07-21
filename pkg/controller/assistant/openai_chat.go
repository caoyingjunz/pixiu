/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type chatStreamResult struct {
	Raw              map[string]interface{}
	Text             string
	ResponseId       string
	ToolCalls        []responseToolCall
	AssistantMessage map[string]interface{}
}

type chatToolCallAccumulator struct {
	Index     int
	Id        string
	Name      string
	Arguments strings.Builder
}

func (c *controller) runChatCompletionsLoopStream(
	ctx context.Context,
	endpoint, apiKey, modelName string,
	maxTokens int,
	inputItems []map[string]interface{},
	emit func(*types.AIStreamEvent) error,
) (map[string]interface{}, string, string, error) {
	messages := responseInputToChatMessages(inputItems)
	messages = append([]map[string]interface{}{{"role": "system", "content": c.defaultAIInstructions()}}, messages...)
	tools := toChatCompletionsTools(c.buildTools())

	for i := 0; i < 8; i++ {
		_ = emit(&types.AIStreamEvent{Type: "status", Stage: "model", Message: "AI is generating a response", Model: modelName})
		result, err := c.callChatCompletionsStream(ctx, endpoint, apiKey, modelName, maxTokens, messages, tools, emit)
		if err != nil {
			return nil, "", "", err
		}
		if len(result.ToolCalls) == 0 {
			return result.Raw, result.Text, result.ResponseId, nil
		}

		messages = append(messages, result.AssistantMessage)
		for _, call := range result.ToolCalls {
			emitToolEvent(emit, "tool_start", "tool", describeToolCall(call), call.CallID, call.Name, call.Arguments, "")
			output, toolErr := c.executeTool(ctx, call.CallID, call.Name, call.Arguments)
			if toolErr != nil {
				output = fmt.Sprintf(`{"error":%q}`, toolErr.Error())
				emitToolEvent(emit, "tool_result", "tool", fmt.Sprintf("tool %s failed", call.Name), call.CallID, call.Name, call.Arguments, toolErr.Error())
			} else {
				emitToolEvent(emit, "tool_result", "tool", fmt.Sprintf("tool %s completed", call.Name), call.CallID, call.Name, call.Arguments, output)
			}
			messages = append(messages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": call.CallID,
				"content":      output,
			})
		}
	}

	return nil, "", "", apierrors.NewError(fmt.Errorf("tool loop exceeded max iterations"), http.StatusBadGateway)
}

func (c *controller) callChatCompletionsStream(
	ctx context.Context,
	endpoint, apiKey, modelName string,
	maxTokens int,
	messages, tools []map[string]interface{},
	emit func(*types.AIStreamEvent) error,
) (*chatStreamResult, error) {
	payload := map[string]interface{}{
		"model":      modelName,
		"messages":   messages,
		"stream":     true,
		"max_tokens": maxTokens,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		klog.Errorf("failed to request ai endpoint %s: %v", endpoint, err)
		return nil, apierrors.NewError(fmt.Errorf("request ai endpoint failed: %v", err), http.StatusBadGateway)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		return nil, apierrors.NewError(fmt.Errorf("ai endpoint returned status %d: %s", resp.StatusCode, string(responseBody)), http.StatusBadGateway)
	}

	result, err := parseChatCompletionsSSE(resp.Body, emit)
	if err != nil {
		klog.Errorf("failed to parse chat completions stream: %v", err)
		return nil, apierrors.NewError(fmt.Errorf("invalid ai response"), http.StatusBadGateway)
	}
	return result, nil
}

func parseChatCompletionsSSE(reader io.Reader, emit func(*types.AIStreamEvent) error) (*chatStreamResult, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var responseId string
	var textBuilder strings.Builder
	var usage map[string]interface{}
	toolCalls := map[int]*chatToolCallAccumulator{}
	seenEvent := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		seenEvent = true
		if id, _ := event["id"].(string); id != "" {
			responseId = id
		}
		if u, ok := event["usage"].(map[string]interface{}); ok {
			usage = u
		}
		if errObject, ok := event["error"].(map[string]interface{}); ok {
			message, _ := errObject["message"].(string)
			return nil, errors.New(message)
		}

		choices, _ := event["choices"].([]interface{})
		for _, choiceValue := range choices {
			choice, _ := choiceValue.(map[string]interface{})
			delta, _ := choice["delta"].(map[string]interface{})
			if content, _ := delta["content"].(string); content != "" {
				textBuilder.WriteString(content)
				_ = emit(&types.AIStreamEvent{Type: "delta", Stage: "message", Delta: content})
			}
			deltas, _ := delta["tool_calls"].([]interface{})
			for _, deltaValue := range deltas {
				item, _ := deltaValue.(map[string]interface{})
				index := int(toInt64(item["index"]))
				accumulator := toolCalls[index]
				if accumulator == nil {
					accumulator = &chatToolCallAccumulator{Index: index}
					toolCalls[index] = accumulator
				}
				if id, _ := item["id"].(string); id != "" {
					accumulator.Id = id
				}
				function, _ := item["function"].(map[string]interface{})
				if name, _ := function["name"].(string); name != "" {
					accumulator.Name = name
				}
				if arguments, _ := function["arguments"].(string); arguments != "" {
					accumulator.Arguments.WriteString(arguments)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !seenEvent {
		return nil, fmt.Errorf("empty sse response")
	}

	indices := make([]int, 0, len(toolCalls))
	for index := range toolCalls {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	calls := make([]responseToolCall, 0, len(indices))
	wireCalls := make([]map[string]interface{}, 0, len(indices))
	for _, index := range indices {
		call := toolCalls[index]
		calls = append(calls, responseToolCall{CallID: call.Id, Name: call.Name, Arguments: call.Arguments.String()})
		wireCalls = append(wireCalls, map[string]interface{}{
			"id":   call.Id,
			"type": "function",
			"function": map[string]interface{}{
				"name":      call.Name,
				"arguments": call.Arguments.String(),
			},
		})
	}
	assistantMessage := map[string]interface{}{"role": "assistant", "content": textBuilder.String()}
	if len(wireCalls) > 0 {
		assistantMessage["tool_calls"] = wireCalls
	}
	raw := map[string]interface{}{
		"id": responseId,
		"choices": []interface{}{map[string]interface{}{
			"message": assistantMessage,
		}},
	}
	if usage != nil {
		raw["usage"] = usage
	}
	return &chatStreamResult{
		Raw: raw, Text: textBuilder.String(), ResponseId: responseId,
		ToolCalls: calls, AssistantMessage: assistantMessage,
	}, nil
}

func responseInputToChatMessages(inputItems []map[string]interface{}) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(inputItems))
	for _, item := range inputItems {
		role, _ := item["role"].(string)
		if role == "" {
			continue
		}
		var parts []string
		content, _ := item["content"].([]interface{})
		for _, value := range content {
			part, _ := value.(map[string]interface{})
			if text, _ := part["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		if typedContent, ok := item["content"].([]map[string]interface{}); ok {
			for _, part := range typedContent {
				if text, _ := part["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		messages = append(messages, map[string]interface{}{"role": role, "content": strings.Join(parts, "\n")})
	}
	return messages
}

func toChatCompletionsTools(tools []toolDefinition) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		items = append(items, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": tool.Name, "description": tool.Description, "parameters": tool.Parameters,
			},
		})
	}
	return items
}
