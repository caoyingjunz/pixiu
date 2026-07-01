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

package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface {
	AI() Interface
}

type Interface interface {
	Respond(ctx context.Context, req *types.AIRespondRequest) (*types.AIRespondResponse, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
	client  *http.Client
}

type toolExecutionContextKey string

const toolExecutionMetaKey toolExecutionContextKey = "ai_tool_execution_meta"

type toolExecutionMeta struct {
	RequestId      string
	UserId         int64
	UserName       string
	AIAccountId    int64
	ConversationId int64
	Provider       string
	ModelName      string
}

type responseUsage struct {
	InputTokens     int64
	OutputTokens    int64
	TotalTokens     int64
	CachedTokens    int64
	ReasoningTokens int64
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:      cfg,
		factory: f,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *controller) Respond(ctx context.Context, req *types.AIRespondRequest) (*types.AIRespondResponse, error) {
	startTime := time.Now()
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil {
		return nil, apierrors.ErrUnauthorized
	}
	userId := user.Id

	account, err := c.getEnabledAccount(ctx, userId, req.Provider)
	if err != nil {
		return nil, err
	}

	conversation, err := c.getConversation(ctx, userId, req.ConversationId)
	if err != nil {
		return nil, err
	}

	modelName := req.Model
	if modelName == "" {
		modelName = account.ModelName
	}
	if modelName == "" {
		return nil, apierrors.NewError(fmt.Errorf("model is required"), http.StatusBadRequest)
	}
	if strings.TrimSpace(account.BaseURL) == "" {
		return nil, apierrors.NewError(fmt.Errorf("base_url is empty"), http.StatusBadRequest)
	}
	if strings.TrimSpace(account.APIKey) == "" {
		return nil, apierrors.NewError(fmt.Errorf("api_key is empty"), http.StatusBadRequest)
	}

	inputItems, err := buildConversationInput(conversation, req.Input)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	ctx = withToolExecutionMeta(ctx, &toolExecutionMeta{
		RequestId:      getRequestIDFromContext(ctx),
		UserId:         user.Id,
		UserName:       user.Name,
		AIAccountId:    account.Id,
		ConversationId: req.ConversationId,
		Provider:       account.Provider,
		ModelName:      modelName,
	})

	endpoint := strings.TrimRight(account.BaseURL, "/") + "/v1/responses"
	raw, text, responseID, err := c.runResponsesLoop(ctx, endpoint, account.APIKey, modelName, inputItems)
	if err != nil {
		c.recordResponseExecution(ctx, account, req.ConversationId, modelName, req.Input, "", "", nil, err, time.Since(startTime))
		return nil, err
	}

	conversationID, err := c.persistConversation(ctx, userId, account, conversation, modelName, req.Input, text, responseID)
	if err != nil {
		c.recordResponseExecution(ctx, account, req.ConversationId, modelName, req.Input, text, responseID, raw, err, time.Since(startTime))
		return nil, err
	}

	c.recordResponseExecution(ctx, account, conversationID, modelName, req.Input, text, responseID, raw, nil, time.Since(startTime))

	return &types.AIRespondResponse{
		ConversationId: conversationID,
		ResponseId:     responseID,
		Text:           text,
		Model:          modelName,
		Raw:            raw,
	}, nil
}

func withToolExecutionMeta(ctx context.Context, meta *toolExecutionMeta) context.Context {
	return context.WithValue(ctx, toolExecutionMetaKey, meta)
}

func getToolExecutionMeta(ctx context.Context) *toolExecutionMeta {
	meta, _ := ctx.Value(toolExecutionMetaKey).(*toolExecutionMeta)
	return meta
}

func getRequestIDFromContext(ctx context.Context) string {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return ""
	}
	return requestid.Get(ginCtx)
}

func (c *controller) recordResponseExecution(
	ctx context.Context,
	account *model.AIAccount,
	conversationID int64,
	modelName string,
	inputText string,
	outputText string,
	responseID string,
	raw map[string]interface{},
	runErr error,
	duration time.Duration,
) {
	meta := getToolExecutionMeta(ctx)
	if meta == nil || account == nil {
		return
	}

	usage := extractResponseUsage(raw)

	recordCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	record := &model.AIResponseExecution{
		RequestId:       meta.RequestId,
		UserId:          meta.UserId,
		UserName:        meta.UserName,
		AIAccountId:     account.Id,
		ConversationId:  conversationID,
		Provider:        account.Provider,
		ModelName:       modelName,
		ResponseId:      responseID,
		InputText:       truncateAuditText(inputText),
		OutputText:      truncateAuditText(outputText),
		Success:         runErr == nil,
		Duration:        duration.Milliseconds(),
		InputTokens:     usage.InputTokens,
		OutputTokens:    usage.OutputTokens,
		TotalTokens:     usage.TotalTokens,
		CachedTokens:    usage.CachedTokens,
		ReasoningTokens: usage.ReasoningTokens,
	}
	if runErr != nil {
		record.ErrorMessage = truncateAuditText(runErr.Error())
	}

	if _, err := c.factory.AIResponseExecution().Create(recordCtx, record); err != nil {
		klog.Errorf("failed to create ai response execution record for response(%s): %v", responseID, err)
	}
}

func (c *controller) runResponsesLoop(ctx context.Context, endpoint, apiKey, modelName string, inputItems []map[string]interface{}) (map[string]interface{}, string, string, error) {
	tools := toResponsesTools(c.buildTools())
	maxIterations := 8

	for i := 0; i < maxIterations; i++ {
		raw, text, err := c.callResponsesAPI(ctx, endpoint, apiKey, modelName, inputItems, tools)
		if err != nil {
			return nil, "", "", err
		}

		toolCalls := extractToolCalls(raw)
		if len(toolCalls) == 0 {
			return raw, text, extractResponseID(raw), nil
		}

		outputItems := extractResponseOutputItems(raw)
		if len(outputItems) > 0 {
			inputItems = append(inputItems, outputItems...)
		}

		for _, call := range toolCalls {
			toolOutput, toolErr := c.executeTool(ctx, call.CallID, call.Name, call.Arguments)
			if toolErr != nil {
				toolOutput = fmt.Sprintf(`{"error":%q}`, toolErr.Error())
			}
			inputItems = append(inputItems, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": call.CallID,
				"output":  toolOutput,
			})
		}
	}

	return nil, "", "", apierrors.NewError(fmt.Errorf("tool loop exceeded max iterations"), http.StatusBadGateway)
}

func (c *controller) callResponsesAPI(ctx context.Context, endpoint, apiKey, modelName string, inputItems []map[string]interface{}, tools []map[string]interface{}) (map[string]interface{}, string, error) {
	payload := map[string]interface{}{
		"model":        modelName,
		"input":        inputItems,
		"stream":       true,
		"instructions": c.defaultAIInstructions(),
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", apierrors.ErrServerInternal
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", apierrors.ErrServerInternal
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		klog.Errorf("failed to request ai endpoint %s: %v", endpoint, err)
		return nil, "", apierrors.NewError(fmt.Errorf("request ai endpoint failed: %v", err), http.StatusBadGateway)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", apierrors.ErrServerInternal
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		klog.Errorf("ai endpoint returned status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, "", apierrors.NewError(fmt.Errorf("ai endpoint returned status %d: %s", resp.StatusCode, string(respBody)), http.StatusBadGateway)
	}

	raw, text, err := parseResponseBody(respBody)
	if err != nil {
		klog.Errorf("failed to parse ai response: %v", err)
		return nil, "", apierrors.NewError(fmt.Errorf("invalid ai response"), http.StatusBadGateway)
	}
	return raw, text, nil
}

func (c *controller) defaultAIInstructions() string {
	instructions := []string{
		"You are an execution-oriented AI assistant running inside Pixiu.",
		"When the user asks to inspect the system, network, files, directories, processes, configuration, or to execute commands, prefer using available tools instead of only explaining what the user could do manually.",
		"If a matching tool exists, call the tool first and then answer with the actual result.",
		"Do not claim a tool is unavailable if it is present in the provided tool list.",
		"If a dangerous shell tool is present, it is intentionally enabled by the server for privileged use. You may use it when necessary to fulfill the user's request.",
		"If a request truly cannot be completed with the available tools, clearly say what is missing.",
	}
	return strings.Join(instructions, " ")
}

func (c *controller) getEnabledAccount(ctx context.Context, userId int64, provider string) (*model.AIAccount, error) {
	opts := []db.Options{
		db.WithUser(userId),
		db.WithEnabled(true),
		db.WithModifyOrderByDesc(),
		db.WithLimit(1),
	}
	if provider != "" {
		opts = append(opts, db.WithProvider(provider))
	}

	accounts, err := c.factory.AIAccount().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list ai accounts for user(%d): %v", userId, err)
		return nil, apierrors.ErrServerInternal
	}
	if len(accounts) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no enabled ai account found"), http.StatusNotFound)
	}
	return &accounts[0], nil
}

func buildConversationInput(conversation *model.AIConversation, input string) ([]map[string]interface{}, error) {
	items := make([]map[string]interface{}, 0)
	if conversation != nil && strings.TrimSpace(conversation.History) != "" {
		if err := json.Unmarshal([]byte(conversation.History), &items); err != nil {
			return nil, err
		}
	}
	items = append(items, map[string]interface{}{
		"role": "user",
		"content": []map[string]interface{}{
			{
				"type": "input_text",
				"text": input,
			},
		},
	})
	return items, nil
}

func (c *controller) getConversation(ctx context.Context, userId, conversationId int64) (*model.AIConversation, error) {
	if conversationId == 0 {
		return nil, nil
	}

	object, err := c.factory.AIConversation().Get(ctx, conversationId)
	if err != nil {
		klog.Errorf("failed to get ai conversation(%d): %v", conversationId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil || object.UserId != userId {
		return nil, apierrors.NewError(fmt.Errorf("ai conversation not found"), http.StatusNotFound)
	}
	return object, nil
}

func (c *controller) persistConversation(
	ctx context.Context,
	userId int64,
	account *model.AIAccount,
	conversation *model.AIConversation,
	modelName string,
	input string,
	outputText string,
	responseID string,
) (int64, error) {
	history, err := appendConversationHistory(conversation, input, outputText)
	if err != nil {
		return 0, apierrors.ErrServerInternal
	}

	if conversation == nil {
		title := strings.TrimSpace(input)
		if len(title) > 120 {
			title = title[:120]
		}
		object, err := c.factory.AIConversation().Create(ctx, &model.AIConversation{
			UserId:             userId,
			AIAccountId:        account.Id,
			Provider:           account.Provider,
			ModelName:          modelName,
			Title:              title,
			PreviousResponseId: responseID,
			History:            history,
		})
		if err != nil {
			klog.Errorf("failed to create ai conversation for user(%d): %v", userId, err)
			return 0, apierrors.ErrServerInternal
		}
		return object.Id, nil
	}

	updates := map[string]interface{}{
		"ai_account_id":        account.Id,
		"provider":             account.Provider,
		"model":                modelName,
		"previous_response_id": responseID,
		"history":              history,
	}
	if err := c.factory.AIConversation().Update(ctx, conversation.Id, conversation.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update ai conversation(%d): %v", conversation.Id, err)
		return 0, apierrors.ErrServerInternal
	}
	return conversation.Id, nil
}

func appendConversationHistory(conversation *model.AIConversation, input, outputText string) (string, error) {
	items := make([]map[string]interface{}, 0)
	if conversation != nil && strings.TrimSpace(conversation.History) != "" {
		if err := json.Unmarshal([]byte(conversation.History), &items); err != nil {
			return "", err
		}
	}

	items = append(items, map[string]interface{}{
		"role": "user",
		"content": []map[string]interface{}{
			{
				"type": "input_text",
				"text": input,
			},
		},
	})
	if strings.TrimSpace(outputText) != "" {
		items = append(items, map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{
					"type": "output_text",
					"text": outputText,
				},
			},
		})
	}

	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseResponseBody(respBody []byte) (map[string]interface{}, string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err == nil {
		return raw, extractResponseText(raw), nil
	}

	raw, text, err := parseSSEResponse(respBody)
	if err != nil {
		return nil, "", err
	}
	return raw, text, nil
}

type responseToolCall struct {
	CallID    string
	Name      string
	Arguments string
}

func extractResponseID(raw map[string]interface{}) string {
	if raw == nil {
		return ""
	}
	if id, _ := raw["id"].(string); id != "" {
		return id
	}
	if response, ok := raw["response"].(map[string]interface{}); ok {
		if id, _ := response["id"].(string); id != "" {
			return id
		}
	}
	return ""
}

func extractResponseUsage(raw map[string]interface{}) responseUsage {
	if raw == nil {
		return responseUsage{}
	}

	usageObj, ok := raw["usage"].(map[string]interface{})
	if !ok {
		return responseUsage{}
	}

	usage := responseUsage{
		InputTokens:  toInt64(usageObj["input_tokens"]),
		OutputTokens: toInt64(usageObj["output_tokens"]),
		TotalTokens:  toInt64(usageObj["total_tokens"]),
	}

	if details, ok := usageObj["input_tokens_details"].(map[string]interface{}); ok {
		usage.CachedTokens = toInt64(details["cached_tokens"])
	}
	if details, ok := usageObj["output_tokens_details"].(map[string]interface{}); ok {
		usage.ReasoningTokens = toInt64(details["reasoning_tokens"])
	}

	return usage
}

func extractResponseText(raw map[string]interface{}) string {
	output, ok := raw["output"].([]interface{})
	if !ok {
		return ""
	}

	var result strings.Builder
	for _, item := range output {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if itemType, _ := obj["type"].(string); itemType == "message" {
			if role, _ := obj["role"].(string); role != "" && role != "assistant" {
				continue
			}
		}

		content, ok := obj["content"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range content {
			contentObj, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if contentType, _ := contentObj["type"].(string); contentType != "" && contentType != "output_text" && contentType != "text" {
				continue
			}
			text, _ := contentObj["text"].(string)
			if text == "" {
				continue
			}
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(text)
		}
	}
	return result.String()
}

func extractResponseOutputItems(raw map[string]interface{}) []map[string]interface{} {
	output, ok := raw["output"].([]interface{})
	if !ok {
		return nil
	}

	items := make([]map[string]interface{}, 0, len(output))
	for _, item := range output {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		items = append(items, obj)
	}
	return items
}

func extractToolCalls(raw map[string]interface{}) []responseToolCall {
	output, ok := raw["output"].([]interface{})
	if !ok {
		return nil
	}

	calls := make([]responseToolCall, 0)
	for _, item := range output {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		itemType, _ := obj["type"].(string)
		if itemType != "function_call" {
			continue
		}

		callID, _ := obj["call_id"].(string)
		name, _ := obj["name"].(string)
		arguments, _ := obj["arguments"].(string)
		if callID == "" || name == "" {
			continue
		}
		calls = append(calls, responseToolCall{
			CallID:    callID,
			Name:      name,
			Arguments: arguments,
		})
	}
	return calls
}

func parseSSEResponse(respBody []byte) (map[string]interface{}, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(respBody))
	var (
		lastEvent    map[string]interface{}
		baseResponse map[string]interface{}
		texts        []string
		outputItems  []interface{}
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
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
		lastEvent = event

		if response, ok := event["response"].(map[string]interface{}); ok {
			baseResponse = response
		}

		switch itemType, _ := event["type"].(string); itemType {
		case "response.output_text.delta":
			if delta, _ := event["delta"].(string); delta != "" {
				texts = append(texts, delta)
			}
		case "response.output_item.done", "response.output_item.added":
			if item, ok := event["item"].(map[string]interface{}); ok {
				outputItems = upsertOutputItem(outputItems, item)
			}
		case "response.completed":
			if response, ok := event["response"].(map[string]interface{}); ok {
				baseResponse = response
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	if lastEvent == nil {
		return nil, "", fmt.Errorf("empty sse response")
	}

	if baseResponse == nil {
		if response, ok := lastEvent["response"].(map[string]interface{}); ok {
			baseResponse = response
		}
	}
	if baseResponse == nil {
		baseResponse = lastEvent
	}

	if len(outputItems) > 0 {
		baseResponse["output"] = outputItems
	}

	text := strings.Join(texts, "")
	if text == "" {
		text = extractResponseText(baseResponse)
	}
	return baseResponse, text, nil
}

func upsertOutputItem(items []interface{}, item map[string]interface{}) []interface{} {
	newKey := responseOutputItemKey(item)
	if newKey == "" {
		return append(items, item)
	}

	for i, existing := range items {
		existingObj, ok := existing.(map[string]interface{})
		if !ok {
			continue
		}
		if responseOutputItemKey(existingObj) == newKey {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func responseOutputItemKey(item map[string]interface{}) string {
	if id, _ := item["id"].(string); id != "" {
		return "id:" + id
	}
	if callID, _ := item["call_id"].(string); callID != "" {
		return "call_id:" + callID
	}
	if itemType, _ := item["type"].(string); itemType != "" {
		if name, _ := item["name"].(string); name != "" {
			return itemType + ":" + name
		}
		return itemType
	}
	return ""
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	default:
		return 0
	}
}

func truncateAuditText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 8000 {
		return text
	}
	return text[:8000]
}
