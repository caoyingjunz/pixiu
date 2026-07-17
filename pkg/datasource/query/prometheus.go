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

package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// MetricSample is a normalized query sample for alert evaluation / UI reuse.
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

// InstantQuery 按数据源 sub_type 执行即时查询（与实时查询选择逻辑对齐：metric 目前仅 prometheus）。
func (c *Client) InstantQuery(ctx context.Context, ds *model.Datasource, expression string) ([]MetricSample, error) {
	if ds == nil {
		return nil, fmt.Errorf("datasource is nil")
	}
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("empty query expression")
	}

	switch ds.SubType {
	case model.DatasourceSubTypePrometheus:
		return c.prometheusInstantQuery(ctx, ds, expression)
	case model.DatasourceSubTypeLoki, model.DatasourceSubTypeES:
		return nil, fmt.Errorf("datasource sub_type %q is not supported for metric instant query", ds.SubType)
	default:
		return nil, fmt.Errorf("unsupported datasource sub_type %q", ds.SubType)
	}
}

func (c *Client) prometheusInstantQuery(ctx context.Context, ds *model.Datasource, promQL string) ([]MetricSample, error) {
	query := url.Values{}
	query.Set("query", promQL)
	body, status, err := c.Do(ctx, ds, Request{
		Method:  http.MethodGet,
		APIPath: "/api/v1/query",
		Query:   query,
	})
	if err != nil {
		return nil, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("prometheus query status %d: %s", status, truncateForError(string(body)))
	}
	return ParsePrometheusQueryResponse(body)
}

type prometheusAPIResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
}

type prometheusVectorItem struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// ParsePrometheusQueryResponse parses Prometheus /api/v1/query JSON.
func ParsePrometheusQueryResponse(body []byte) ([]MetricSample, error) {
	var resp prometheusAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("invalid prometheus response: %w", err)
	}
	if !strings.EqualFold(resp.Status, "success") {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = "unknown prometheus error"
		}
		return nil, fmt.Errorf("prometheus status=%s error=%s", resp.Status, msg)
	}

	switch resp.Data.ResultType {
	case "vector":
		var items []prometheusVectorItem
		if err := json.Unmarshal(resp.Data.Result, &items); err != nil {
			return nil, fmt.Errorf("invalid prometheus vector: %w", err)
		}
		samples := make([]MetricSample, 0, len(items))
		for _, item := range items {
			value, ok := prometheusSampleValue(item.Value)
			if !ok {
				continue
			}
			labels := cloneStringMap(item.Metric)
			samples = append(samples, MetricSample{
				Value:        value,
				ResourceType: "metric",
				ResourceName: FingerprintMetricLabels(labels),
				Namespace:    labels["namespace"],
				Labels:       labels,
			})
		}
		return samples, nil
	case "scalar":
		var raw []interface{}
		if err := json.Unmarshal(resp.Data.Result, &raw); err != nil {
			return nil, fmt.Errorf("invalid prometheus scalar: %w", err)
		}
		value, ok := prometheusSampleValue(raw)
		if !ok {
			return nil, nil
		}
		return []MetricSample{{
			Value:        value,
			ResourceType: "metric",
			ResourceName: "scalar",
			Labels:       map[string]string{},
		}}, nil
	default:
		return nil, nil
	}
}

func prometheusSampleValue(pair []interface{}) (string, bool) {
	if len(pair) < 2 {
		return "", false
	}
	switch v := pair[1].(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" || strings.EqualFold(v, "NaN") || strings.EqualFold(v, "+Inf") || strings.EqualFold(v, "-Inf") {
			return "", false
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", false
		}
		return v, true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case json.Number:
		return v.String(), true
	default:
		return "", false
	}
}

// FingerprintMetricLabels builds a stable series identity for alerts.
func FingerprintMetricLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "series"
	}
	for _, key := range []string{"instance", "pod", "node", "container", "job"} {
		if v := strings.TrimSpace(labels[key]); v != "" {
			parts := []string{key + "=" + v}
			if ns := strings.TrimSpace(labels["namespace"]); ns != "" && key != "instance" {
				parts = append([]string{"namespace=" + ns}, parts...)
			}
			return strings.Join(parts, ",")
		}
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		if k == "__name__" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		if name := strings.TrimSpace(labels["__name__"]); name != "" {
			return name
		}
		return "series"
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	out := strings.Join(parts, ",")
	if len(out) > 240 {
		return out[:240]
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func truncateForError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 256 {
		return s
	}
	return s[:256]
}
