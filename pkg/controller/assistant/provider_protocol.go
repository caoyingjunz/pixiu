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
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	ProtocolOpenAIChat      = "openai_chat"
	ProtocolOpenAIResponses = "openai_responses"
)

var versionPathPattern = regexp.MustCompile(`/v[0-9]+$`)

func normalizeProtocol(protocol string) string {
	return strings.ToLower(strings.TrimSpace(protocol))
}

func resolveAIEndpoint(provider *model.AIProvider) (string, error) {
	if provider == nil || strings.TrimSpace(provider.BaseURL) == "" {
		return "", apierrors.NewError(fmt.Errorf("ai provider base_url is empty"), http.StatusBadRequest)
	}

	var resource string
	switch normalizeProtocol(provider.Protocol) {
	case ProtocolOpenAIResponses:
		resource = "responses"
	case ProtocolOpenAIChat:
		resource = "chat/completions"
	default:
		return "", apierrors.NewError(fmt.Errorf("unsupported ai protocol %q", provider.Protocol), http.StatusBadRequest)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if versionPathPattern.MatchString(baseURL) {
		return baseURL + "/" + resource, nil
	}
	return baseURL + "/v1/" + resource, nil
}

func (c *controller) runProviderStream(
	ctx context.Context,
	protocol, endpoint, apiKey, modelName string,
	maxTokens int,
	inputItems []map[string]interface{},
	emit func(*types.AIStreamEvent) error,
) (map[string]interface{}, string, string, error) {
	switch normalizeProtocol(protocol) {
	case ProtocolOpenAIResponses:
		return c.runResponsesLoopStream(ctx, endpoint, apiKey, modelName, maxTokens, inputItems, emit)
	case ProtocolOpenAIChat:
		return c.runChatCompletionsLoopStream(ctx, endpoint, apiKey, modelName, maxTokens, inputItems, emit)
	default:
		return nil, "", "", apierrors.NewError(fmt.Errorf("unsupported ai protocol %q", protocol), http.StatusBadRequest)
	}
}
