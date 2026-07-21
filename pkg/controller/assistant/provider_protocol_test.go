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
	"strings"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestResolveAIEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		provider model.AIProvider
		want     string
	}{
		{
			name:     "openai responses root",
			provider: model.AIProvider{BaseURL: "https://api.openai.com", Protocol: ProtocolOpenAIResponses},
			want:     "https://api.openai.com/v1/responses",
		},
		{
			name:     "siliconflow versioned root",
			provider: model.AIProvider{BaseURL: "https://api.siliconflow.cn/v1/", Protocol: ProtocolOpenAIChat},
			want:     "https://api.siliconflow.cn/v1/chat/completions",
		},
		{
			name:     "zhipu versioned root",
			provider: model.AIProvider{BaseURL: "https://open.bigmodel.cn/api/paas/v4", Protocol: ProtocolOpenAIChat},
			want:     "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAIEndpoint(&tt.provider)
			if err != nil {
				t.Fatalf("resolveAIEndpoint() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveAIEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseChatCompletionsSSE(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"id":"chat-1","choices":[{"delta":{"content":"hello "}}]}`,
		`data: {"id":"chat-1","choices":[{"delta":{"content":"world"}}]}`,
		`data: [DONE]`,
	}, "\n\n")
	var deltas []string
	result, err := parseChatCompletionsSSE(strings.NewReader(stream), func(event *types.AIStreamEvent) error {
		if event.Delta != "" {
			deltas = append(deltas, event.Delta)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parseChatCompletionsSSE() error = %v", err)
	}
	if result.ResponseId != "chat-1" || result.Text != "hello world" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Join(deltas, "") != "hello world" {
		t.Fatalf("unexpected deltas: %#v", deltas)
	}
}
