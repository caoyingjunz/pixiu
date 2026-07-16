/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"context"
	"encoding/json"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type MetricSample struct {
	Value        string
	ResourceType string
	ResourceName string
	Namespace    string
	ClusterId    int64
	TenantId     int64
	Labels       map[string]string
	Annotations  map[string]string
}

type MetricProvider interface {
	Query(ctx context.Context, rule *model.AlertRule) ([]MetricSample, error)
}

// StaticMetricProvider is a no-op provider kept for tests / local dry-runs.
type StaticMetricProvider struct{}

func (p *StaticMetricProvider) Query(_ context.Context, _ *model.AlertRule) ([]MetricSample, error) {
	return nil, nil
}

func encodeJSONMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}
