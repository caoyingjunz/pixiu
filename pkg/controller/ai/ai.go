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
	"errors"
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
	RespondStream(ctx context.Context, req *types.AIRespondRequest, emit func(*types.AIStreamEvent) error) (*types.AIRespondResponse, error)
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
	Authorization  string
	Cookies        []*http.Cookie
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

func (c *controller) RespondStream(ctx context.Context, req *types.AIRespondRequest, emit func(*types.AIStreamEvent) error) (*types.AIRespondResponse, error) {
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

	var (
		authorization string
		cookies       []*http.Cookie
	)
	if ginCtx, ok := ctx.(*gin.Context); ok {
		authorization = ginCtx.GetHeader("Authorization")
		cookies = ginCtx.Request.Cookies()
	}

	ctx = withToolExecutionMeta(ctx, &toolExecutionMeta{
		RequestId:      getRequestIDFromContext(ctx),
		UserId:         user.Id,
		UserName:       user.Name,
		AIAccountId:    account.Id,
		ConversationId: req.ConversationId,
		Provider:       account.Provider,
		ModelName:      modelName,
		Authorization:  authorization,
		Cookies:        cookies,
	})

	_ = emit(&types.AIStreamEvent{
		Type:    "status",
		Stage:   "started",
		Message: "宸插彂璧?AI 鍒嗘瀽璇锋眰",
		Model:   modelName,
	})

	endpoint := strings.TrimRight(account.BaseURL, "/") + "/v1/responses"
	raw, text, responseID, err := c.runResponsesLoopStream(ctx, endpoint, account.APIKey, modelName, inputItems, emit)
	if err != nil {
		c.recordResponseExecution(ctx, account, req.ConversationId, modelName, req.Input, "", "", nil, err, time.Since(startTime))
		return nil, err
	}

	conversationID := req.ConversationId
	if conversationID == 0 && conversation != nil {
		conversationID = conversation.Id
	}
	if persistedConversationID, persistErr := c.persistConversation(ctx, userId, account, conversation, modelName, req.Input, text, responseID); persistErr != nil {
		klog.Errorf("failed to persist ai conversation after streaming response: %v", persistErr)
		_ = emit(&types.AIStreamEvent{
			Type:    "status",
			Stage:   "warning",
			Message: "回复已生成，但本轮会话未成功保存",
		})
	} else {
		conversationID = persistedConversationID
	}

	c.recordResponseExecution(ctx, account, conversationID, modelName, req.Input, text, responseID, raw, nil, time.Since(startTime))

	resp := &types.AIRespondResponse{
		ConversationId: conversationID,
		ResponseId:     responseID,
		Text:           text,
		Model:          modelName,
		Raw:            raw,
	}
	_ = emit(&types.AIStreamEvent{
		Type:           "complete",
		Stage:          "completed",
		Message:        "AI 鍒嗘瀽瀹屾垚",
		Text:           text,
		Model:          modelName,
		ConversationId: conversationID,
		ResponseId:     responseID,
	})
	return resp, nil
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

func (c *controller) runResponsesLoopStream(
	ctx context.Context,
	endpoint, apiKey, modelName string,
	inputItems []map[string]interface{},
	emit func(*types.AIStreamEvent) error,
) (map[string]interface{}, string, string, error) {
	tools := toResponsesTools(c.buildTools())
	maxIterations := 8

	for i := 0; i < maxIterations; i++ {
		_ = emit(&types.AIStreamEvent{
			Type:    "status",
			Stage:   "model",
			Message: "AI 姝ｅ湪鐢熸垚鍒嗘瀽鍐呭",
			Model:   modelName,
		})

		raw, text, err := c.callResponsesAPIStream(ctx, endpoint, apiKey, modelName, inputItems, tools, emit)
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
			_ = emit(&types.AIStreamEvent{
				Type:       "tool_start",
				Stage:      "tool",
				Message:    describeToolCall(call),
				ToolCallId: call.CallID,
				ToolName:   call.Name,
				ToolArgs:   truncateToolOutput(call.Arguments),
			})

			toolOutput, toolErr := c.executeTool(ctx, call.CallID, call.Name, call.Arguments)
			if toolErr != nil {
				_ = emit(&types.AIStreamEvent{
					Type:       "tool_result",
					Stage:      "tool",
					Message:    fmt.Sprintf("宸ュ叿 %s 鎵ц澶辫触", call.Name),
					ToolCallId: call.CallID,
					ToolName:   call.Name,
					ToolArgs:   truncateToolOutput(call.Arguments),
					ToolOutput: truncateToolOutput(toolErr.Error()),
				})
				toolOutput = fmt.Sprintf(`{"error":%q}`, toolErr.Error())
			} else {
				_ = emit(&types.AIStreamEvent{
					Type:       "tool_result",
					Stage:      "tool",
					Message:    fmt.Sprintf("宸ュ叿 %s 鎵ц瀹屾垚", call.Name),
					ToolCallId: call.CallID,
					ToolName:   call.Name,
					ToolArgs:   truncateToolOutput(call.Arguments),
					ToolOutput: truncateToolOutput(toolOutput),
				})
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

func (c *controller) callResponsesAPIStream(
	ctx context.Context,
	endpoint, apiKey, modelName string,
	inputItems []map[string]interface{},
	tools []map[string]interface{},
	emit func(*types.AIStreamEvent) error,
) (map[string]interface{}, string, error) {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		klog.Errorf("ai endpoint returned status=%d body=%s", resp.StatusCode, string(respBody))
		return nil, "", apierrors.NewError(fmt.Errorf("ai endpoint returned status %d: %s", resp.StatusCode, string(respBody)), http.StatusBadGateway)
	}

	raw, text, err := parseSSEResponseStream(resp.Body, emit)
	if err != nil {
		klog.Errorf("failed to parse ai response stream: %v", err)
		return nil, "", apierrors.NewError(fmt.Errorf("invalid ai response"), http.StatusBadGateway)
	}
	return raw, text, nil
}

func (c *controller) defaultAIInstructions() string {
	instructions := []string{
		"你是 Pixiu 平台里的中文 Kubernetes 运维助手，负责直接排查、定位和给出修复建议。",
		"优先使用 k8s 工具获取真实结果，不要凭空猜测；只要工具可用，就先查再答。",
		"默认只做查询和分析，不要主动执行删除、重启、扩缩容、修改配置等变更操作，除非用户明确要求。",
		"回答必须简洁，优先输出结论，不要写成长篇报告，不要大段复述原始日志、事件或 YAML。",
		"故障分析默认按这个结构输出：1) 结论 2) 直接原因 3) 关键证据 4) 修复建议。",
		"每个部分尽量控制在 1 到 3 条短句或短 bullet，优先保留最关键的信息。",
		"如果根因已经明确，直接给出根因和修复动作；如果还不能完全确认，只给最可能原因、当前证据和下一步排查建议。",
		"如果结果很多，只摘录最关键的对象名、状态、报错原因和必要字段，不要把全部原始输出搬回来。",
		"修复建议要可执行，优先给最短路径，例如检查镜像地址、镜像仓库权限、启动命令、探针、环境变量、PVC、DNS、Service 选择器、Ingress、节点状态等。",
		"如果用户是在连续追问同一个问题，要结合上下文直接回答，不要每次都重复完整背景。",
		"回答必须使用中文，语气专业、直接、短句化；除非用户明确要求详细解释，否则默认输出精简版。",
		"如果工具无法完成请求，直接说明缺少什么信息或卡在哪一步。",
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

func describeToolCall(call responseToolCall) string {
	if call.Name == "" {
		return "AI 姝ｅ湪鎵ц鎺掓煡宸ュ叿"
	}
	if call.Name != "k8s" {
		return fmt.Sprintf("AI 姝ｅ湪鎵ц宸ュ叿 %s", call.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return "AI 姝ｅ湪鎵ц Kubernetes 鎺掓煡"
	}

	cluster, _ := args["cluster"].(string)
	rawArgs, err := parseStringArgs(args["args"])
	if err != nil || len(rawArgs) == 0 {
		if cluster != "" {
			return fmt.Sprintf("AI 姝ｅ湪闆嗙兢 %s 涓墽琛?Kubernetes 鎺掓煡", cluster)
		}
		return "AI 姝ｅ湪鎵ц Kubernetes 鎺掓煡"
	}

	summary := strings.Join(rawArgs, " ")
	if cluster != "" {
		return fmt.Sprintf("AI 姝ｅ湪闆嗙兢 %s 涓墽琛?kubectl %s", cluster, summary)
	}
	return fmt.Sprintf("AI 姝ｅ湪鎵ц kubectl %s", summary)
}

func parseSSEResponseStream(reader io.Reader, emit func(*types.AIStreamEvent) error) (map[string]interface{}, string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
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
		case "response.created":
			_ = emit(&types.AIStreamEvent{
				Type:    "status",
				Stage:   "accepted",
				Message: "AI 已开始处理请求",
			})
		case "response.output_text.delta":
			if delta, _ := event["delta"].(string); delta != "" {
				texts = append(texts, delta)
				_ = emit(&types.AIStreamEvent{
					Type:  "delta",
					Stage: "message",
					Delta: delta,
				})
			}
		case "response.output_item.done", "response.output_item.added":
			if item, ok := event["item"].(map[string]interface{}); ok {
				outputItems = upsertOutputItem(outputItems, item)
			}
		case "response.completed":
			if response, ok := event["response"].(map[string]interface{}); ok {
				baseResponse = response
			}
		case "response.failed", "error":
			message := "AI 璇锋眰澶辫触"
			if msg, _ := event["message"].(string); msg != "" {
				message = msg
			}
			return nil, "", errors.New(message)
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
