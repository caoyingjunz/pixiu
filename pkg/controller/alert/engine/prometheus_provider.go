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

package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	dsutil "github.com/caoyingjunz/pixiu/pkg/util/datasource"
)

const (
	prometheusInstantAPI   = "/api/v1/query"
	lokiQueryRangeAPI      = "/loki/api/v1/query_range"
	prometheusQueryTimeout = 30 * time.Second
	lokiQueryLimit         = "500"
)

// DatasourceMetricProvider queries Prometheus / Loki via external HTTP or in-cluster Service proxy.
type DatasourceMetricProvider struct {
	factory    db.ShareDaoFactory
	httpClient *http.Client

	mu       sync.Mutex
	clusters map[string]*client.ClusterSet
}

func NewDatasourceMetricProvider(factory db.ShareDaoFactory) *DatasourceMetricProvider {
	return &DatasourceMetricProvider{
		factory: factory,
		httpClient: &http.Client{
			Timeout: prometheusQueryTimeout,
		},
		clusters: make(map[string]*client.ClusterSet),
	}
}

func (p *DatasourceMetricProvider) Query(ctx context.Context, rule *model.AlertRule) ([]MetricSample, error) {
	if rule == nil {
		return nil, nil
	}
	switch rule.RuleType {
	case model.AlertRuleTypeMetric:
		return p.queryPrometheus(ctx, rule)
	case model.AlertRuleTypeLog:
		return p.queryLoki(ctx, rule)
	default:
		return nil, nil
	}
}

func (p *DatasourceMetricProvider) queryPrometheus(ctx context.Context, rule *model.AlertRule) ([]MetricSample, error) {
	ds, cfg, baseURL, query, err := p.loadQueryContext(ctx, rule, model.DatasourceSubTypePrometheus)
	if err != nil || query == "" {
		return nil, err
	}
	raw, err := p.fetch(ctx, ds, &cfg, baseURL, prometheusInstantAPI, map[string]string{
		"query": query,
	})
	if err != nil {
		return nil, err
	}
	return parsePrometheusInstantSamples(raw)
}

func (p *DatasourceMetricProvider) queryLoki(ctx context.Context, rule *model.AlertRule) ([]MetricSample, error) {
	ds, cfg, baseURL, query, err := p.loadQueryContext(ctx, rule, model.DatasourceSubTypeLoki)
	if err != nil || query == "" {
		return nil, err
	}

	lookback := time.Duration(NormalizeEvalInterval(rule.EvalInterval)) * time.Second
	end := time.Now()
	start := end.Add(-lookback)
	raw, err := p.fetch(ctx, ds, &cfg, baseURL, lokiQueryRangeAPI, map[string]string{
		"query":     query,
		"limit":     lokiQueryLimit,
		"start":     strconv.FormatInt(start.UnixNano(), 10),
		"end":       strconv.FormatInt(end.UnixNano(), 10),
		"direction": "BACKWARD",
	})
	if err != nil {
		return nil, err
	}
	return parseLokiRangeSamples(raw)
}

func (p *DatasourceMetricProvider) loadQueryContext(
	ctx context.Context,
	rule *model.AlertRule,
	wantSubType model.DatasourceSubType,
) (*model.Datasource, types.DatasourceConfig, string, string, error) {
	if rule.DatasourceId <= 0 {
		return nil, types.DatasourceConfig{}, "", "", fmt.Errorf("alert rule %d missing datasource_id", rule.Id)
	}
	query := GetRulePromQl(rule)
	if query == "" {
		return nil, types.DatasourceConfig{}, "", "", nil
	}

	ds, err := p.factory.Datasource().Get(ctx, rule.DatasourceId)
	if err != nil {
		return nil, types.DatasourceConfig{}, "", "", fmt.Errorf("load datasource %d: %w", rule.DatasourceId, err)
	}
	if ds == nil {
		return nil, types.DatasourceConfig{}, "", "", fmt.Errorf("datasource %d not found", rule.DatasourceId)
	}
	if ds.SubType != wantSubType {
		return nil, types.DatasourceConfig{}, "", "", fmt.Errorf("datasource %d subtype %s is not %s", ds.Id, ds.SubType, wantSubType)
	}

	var cfg types.DatasourceConfig
	if err = cfg.Unmarshal(ds.Config); err != nil {
		return nil, types.DatasourceConfig{}, "", "", fmt.Errorf("parse datasource %d config: %w", ds.Id, err)
	}
	baseURL := resolveDatasourceURL(ds.Type, &cfg)
	if baseURL == "" {
		return nil, types.DatasourceConfig{}, "", "", fmt.Errorf("datasource %d has empty url", ds.Id)
	}
	return ds, cfg, baseURL, query, nil
}

func (p *DatasourceMetricProvider) fetch(
	ctx context.Context,
	ds *model.Datasource,
	cfg *types.DatasourceConfig,
	baseURL, apiPath string,
	params map[string]string,
) ([]byte, error) {
	if ds.External {
		return p.fetchExternal(ctx, baseURL, apiPath, params, cfg)
	}
	return p.fetchInternal(ctx, ds.ClusterName, baseURL, apiPath, params)
}

func (p *DatasourceMetricProvider) fetchExternal(
	ctx context.Context,
	baseURL, apiPath string,
	params map[string]string,
	cfg *types.DatasourceConfig,
) ([]byte, error) {
	endpoint, err := buildDatasourceAPIURL(baseURL, apiPath, params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	// TODO 复用
	applyDatasourceAuth(req, cfg)
	applyDatasourceHeaders(req, cfg)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("datasource external query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("datasource external query status %d: %s", resp.StatusCode, truncate(string(body), 256))
	}
	return body, nil
}

func (p *DatasourceMetricProvider) fetchInternal(
	ctx context.Context,
	clusterName, baseURL, apiPath string,
	params map[string]string,
) ([]byte, error) {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return nil, fmt.Errorf("internal datasource missing cluster_name")
	}

	ep, err := dsutil.ParseInClusterEndpoint(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse internal datasource url: %w", err)
	}

	// TODO 复用
	cs, err := p.getClusterSet(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	raw, err := cs.Client.CoreV1().Services(ep.Namespace).ProxyGet(
		ep.Protocol,
		ep.ServiceName,
		strconv.Itoa(ep.Port),
		ep.JoinProxyPath(apiPath),
		params,
	).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("datasource internal query via %s/%s: %w", ep.Namespace, ep.ServiceName, err)
	}
	return raw, nil
}

func (p *DatasourceMetricProvider) getClusterSet(ctx context.Context, clusterName string) (*client.ClusterSet, error) {
	p.mu.Lock()
	if cs, ok := p.clusters[clusterName]; ok {
		p.mu.Unlock()
		return cs, nil
	}
	p.mu.Unlock()

	object, err := p.factory.Cluster().GetClusterByName(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("load cluster %s: %w", clusterName, err)
	}
	if object == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}
	cs, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("build cluster client %s: %w", clusterName, err)
	}

	p.mu.Lock()
	p.clusters[clusterName] = cs
	p.mu.Unlock()
	return cs, nil
}

func resolveDatasourceURL(dsType model.DatasourceType, cfg *types.DatasourceConfig) string {
	if cfg == nil {
		return ""
	}
	switch dsType {
	case model.DatasourceTypeAlert:
		if cfg.Alert != nil {
			return strings.TrimRight(strings.TrimSpace(cfg.Alert.URL), "/")
		}
	case model.DatasourceTypeLog:
		if cfg.Log != nil {
			return strings.TrimRight(strings.TrimSpace(cfg.Log.URL), "/")
		}
	}
	return ""
}

func buildDatasourceAPIURL(baseURL, apiPath string, params map[string]string) (string, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", fmt.Errorf("invalid datasource url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("datasource url must include scheme and host")
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	u.Path = strings.TrimRight(u.Path, "/") + apiPath
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func applyDatasourceAuth(req *http.Request, cfg *types.DatasourceConfig) {
	if cfg == nil {
		return
	}
	var username, password string
	if cfg.Alert != nil {
		username = cfg.Alert.UserName
		password = cfg.Alert.Password
	}
	if username == "" && password == "" && cfg.Log != nil {
		username = cfg.Log.UserName
		password = cfg.Log.Password
	}
	if username == "" && password == "" {
		return
	}
	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+token)
}

func applyDatasourceHeaders(req *http.Request, cfg *types.DatasourceConfig) {
	if cfg == nil {
		return
	}
	for _, h := range cfg.Headers {
		key := strings.TrimSpace(h.Key)
		if key == "" {
			continue
		}
		req.Header.Set(key, h.Value)
	}
}

type prometheusInstantResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
}

type prometheusVectorItem struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}

type prometheusMatrixItem struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
}

type lokiStreamItem struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func parsePrometheusInstantSamples(raw []byte) ([]MetricSample, error) {
	var resp prometheusInstantResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	if resp.Status != "" && resp.Status != "success" {
		if resp.Error != "" {
			return nil, fmt.Errorf("prometheus query failed: %s", resp.Error)
		}
		return nil, fmt.Errorf("prometheus query status=%s", resp.Status)
	}

	switch resp.Data.ResultType {
	case "", "vector":
		var items []prometheusVectorItem
		if len(resp.Data.Result) == 0 || string(resp.Data.Result) == "null" {
			return nil, nil
		}
		if err := json.Unmarshal(resp.Data.Result, &items); err != nil {
			return nil, fmt.Errorf("decode prometheus vector: %w", err)
		}
		out := make([]MetricSample, 0, len(items))
		for _, item := range items {
			value, ok := extractPrometheusValue(item.Value)
			if !ok {
				continue
			}
			out = append(out, metricSampleFromLabels(item.Metric, value))
		}
		return out, nil
	case "scalar":
		var pair []any
		if err := json.Unmarshal(resp.Data.Result, &pair); err != nil {
			return nil, fmt.Errorf("decode prometheus scalar: %w", err)
		}
		value, ok := extractPrometheusValue(pair)
		if !ok {
			return nil, nil
		}
		return []MetricSample{metricSampleFromLabels(nil, value)}, nil
	default:
		return nil, fmt.Errorf("unsupported prometheus resultType %q", resp.Data.ResultType)
	}
}

func parseLokiRangeSamples(raw []byte) ([]MetricSample, error) {
	var resp prometheusInstantResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode loki response: %w", err)
	}
	if resp.Status != "" && resp.Status != "success" {
		if resp.Error != "" {
			return nil, fmt.Errorf("loki query failed: %s", resp.Error)
		}
		return nil, fmt.Errorf("loki query status=%s", resp.Status)
	}
	if len(resp.Data.Result) == 0 || string(resp.Data.Result) == "null" {
		return nil, nil
	}

	switch resp.Data.ResultType {
	case "streams":
		var items []lokiStreamItem
		if err := json.Unmarshal(resp.Data.Result, &items); err != nil {
			return nil, fmt.Errorf("decode loki streams: %w", err)
		}
		out := make([]MetricSample, 0, len(items))
		for _, item := range items {
			labels := item.Stream
			if labels == nil {
				labels = map[string]string{}
			}
			// Stream LogQL: sample value is matched line count in the eval window.
			sample := metricSampleFromLabels(labels, strconv.Itoa(len(item.Values)))
			sample.ResourceType = "log"
			if sample.ResourceName == "" || sample.ResourceName == "scalar" {
				sample.ResourceName = labelsFingerprint(labels)
			}
			out = append(out, sample)
		}
		return out, nil
	case "matrix":
		var items []prometheusMatrixItem
		if err := json.Unmarshal(resp.Data.Result, &items); err != nil {
			return nil, fmt.Errorf("decode loki matrix: %w", err)
		}
		out := make([]MetricSample, 0, len(items))
		for _, item := range items {
			if len(item.Values) == 0 {
				continue
			}
			value, ok := extractPrometheusValue(item.Values[len(item.Values)-1])
			if !ok {
				continue
			}
			out = append(out, metricSampleFromLabels(item.Metric, value))
		}
		return out, nil
	case "vector":
		return parsePrometheusInstantSamples(raw)
	default:
		return nil, fmt.Errorf("unsupported loki resultType %q", resp.Data.ResultType)
	}
}

func extractPrometheusValue(pair []any) (string, bool) {
	if len(pair) < 2 {
		return "", false
	}
	switch v := pair[1].(type) {
	case string:
		return v, true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case json.Number:
		return v.String(), true
	default:
		return fmt.Sprint(v), true
	}
}

func metricSampleFromLabels(labels map[string]string, value string) MetricSample {
	if labels == nil {
		labels = map[string]string{}
	}
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}

	resourceType, resourceName := inferResource(copied)
	return MetricSample{
		Value:        value,
		ResourceType: resourceType,
		ResourceName: resourceName,
		Namespace:    firstNonEmpty(copied["namespace"], copied["Namespace"]),
		Labels:       copied,
	}
}

func inferResource(labels map[string]string) (resourceType, resourceName string) {
	switch {
	case labels["pod"] != "":
		return "pod", labels["pod"]
	case labels["node"] != "":
		return "node", labels["node"]
	case labels["deployment"] != "":
		return "deployment", labels["deployment"]
	case labels["container"] != "":
		return "container", labels["container"]
	case labels["instance"] != "":
		return "instance", labels["instance"]
	case labels["job"] != "":
		return "job", labels["job"]
	default:
		return "series", labelsFingerprint(labels)
	}
}

func labelsFingerprint(labels map[string]string) string {
	if len(labels) == 0 {
		return "scalar"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		if k == "__name__" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	if len(parts) == 0 {
		if name := labels["__name__"]; name != "" {
			return name
		}
		return "series"
	}
	return strings.Join(parts, ",")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
