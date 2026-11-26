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

package middleware

// 本文件处理经代理访问的 HTTP 数据源（Nacos/Elasticsearch/RabbitMQ/Redis/对象存储）：
// 判断是否属于该数据源、归类操作类型、并从路径/查询参数/请求体中提取资源名。
// 每个数据源按「路径判定 -> 噪音过滤 -> 操作归类 -> 资源提取 -> Detail 拼装」顺序排列。

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	auditlog "github.com/caoyingjunz/pixiu/pkg/audit"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const pixiuDatasourceIDHeader = "X-Pixiu-Datasource-Id"

// ---------- 共用工具 ----------

// extractProxyUpstreamPath 去掉 pixiu 的代理前缀，还原上游服务的原始路径。
func extractProxyUpstreamPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/pixiu/external/"):
		return "/" + strings.TrimPrefix(path, "/pixiu/external/")
	case strings.HasPrefix(path, "/pixiu/proxy/"):
		rest := strings.TrimPrefix(path, "/pixiu/proxy/")
		if idx := strings.Index(rest, "/proxy/"); idx >= 0 {
			return "/" + rest[idx+len("/proxy/"):]
		}
	}
	return path
}

func decodeAuditBody(raw []byte) map[string]interface{} {
	var obj map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil
		}
	}
	return obj
}

func lookupJSONValue(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if s, ok := obj[key].(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	return ""
}

func lookupFormValue(values []url.Values, keys ...string) string {
	for _, item := range values {
		for _, key := range keys {
			if s := strings.TrimSpace(item.Get(key)); s != "" {
				return s
			}
		}
	}
	return ""
}

// parseDatasourceIDHeader 读取前端透传的数据源 ID，缺失或非法时返回 0。
func parseDatasourceIDHeader(c *gin.Context) int64 {
	raw := strings.TrimSpace(c.GetHeader(pixiuDatasourceIDHeader))
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// ---------- Nacos ----------

func isNacosAuditPath(path string) bool {
	u := strings.ToLower(extractProxyUpstreamPath(path))
	if strings.Contains(u, "/nacos/") {
		return true
	}
	return strings.Contains(u, "/v3/console/") || strings.Contains(u, "/v3/auth/")
}

func isNacosAuditNoise(path string) bool {
	if !isNacosAuditPath(path) {
		return false
	}
	u := strings.ToLower(extractProxyUpstreamPath(path))
	switch {
	case strings.Contains(u, "/auth/login"),
		strings.Contains(u, "/auth/user/login"),
		strings.Contains(u, "/server/state"),
		strings.Contains(u, "/listener"),
		strings.Contains(u, "/operator/metrics"),
		strings.Contains(u, "/cs/history"),
		strings.Contains(u, "/config/list"):
		return true
	default:
		return false
	}
}

func isNacosConfigPath(upstream string) bool {
	u := strings.ToLower(upstream)
	if strings.Contains(u, "listener") || strings.Contains(u, "/history") || strings.Contains(u, "/config/list") {
		return false
	}
	return strings.Contains(u, "/cs/configs") || strings.Contains(u, "/cs/config")
}

func classifyNacos(method, upstream string) auditRule {
	rule := defaultAuditRule(method, model.ObjectNacos)
	u := strings.ToLower(upstream)
	// 配置发布和实例注册均为 upsert。
	if method != http.MethodDelete && (isNacosConfigPath(u) || strings.Contains(u, "/ns/instance")) {
		rule.Operation = auditlog.OperationUpdate
	}
	return rule
}

type nacosResource struct {
	Name      string
	Group     string
	Namespace string
}

func extractNacosResource(c *gin.Context, body []byte) nacosResource {
	query := url.Values{}
	if c.Request != nil && c.Request.URL != nil {
		query = c.Request.URL.Query()
	}
	form := url.Values{}
	if len(body) > 0 {
		if parsed, err := url.ParseQuery(string(body)); err == nil {
			form = parsed
		}
	}
	obj := decodeAuditBody(body)
	res := nacosResource{
		Name:      lookupFormValue([]url.Values{query, form}, "dataId", "data_id", "serviceName", "username", "namespaceName", "namespaceShowName"),
		Group:     lookupFormValue([]url.Values{query, form}, "group", "groupName"),
		Namespace: lookupFormValue([]url.Values{query, form}, "tenant", "namespaceId", "customNamespaceId", "namespace"),
	}
	if res.Name == "" {
		res.Name = lookupJSONValue(obj, "dataId", "data_id", "serviceName", "username", "namespaceName")
	}
	if res.Group == "" {
		res.Group = lookupJSONValue(obj, "group", "groupName")
	}
	if res.Namespace == "" {
		res.Namespace = lookupJSONValue(obj, "tenant", "namespaceId", "namespace")
	}
	return res
}

func nacosAuditDetail(res nacosResource) string {
	parts := make([]string, 0, 3)
	if res.Name != "" {
		if res.Group != "" {
			parts = append(parts, "dataId="+res.Name)
		} else {
			parts = append(parts, "name="+res.Name)
		}
	}
	if res.Group != "" {
		parts = append(parts, "group="+res.Group)
	}
	if res.Namespace != "" {
		parts = append(parts, "namespace="+res.Namespace)
	}
	return strings.Join(parts, " ")
}

// ---------- Elasticsearch ----------

func isElasticsearchAuditPath(path string) bool {
	if isNacosAuditPath(path) || isRabbitMQAuditPath(path) {
		return false
	}
	u := strings.Trim(strings.ToLower(extractProxyUpstreamPath(path)), "/")
	if u == "" {
		return false
	}
	if !(strings.HasPrefix(path, "/pixiu/external/") || strings.Contains(path, "/proxy/")) {
		return false
	}
	segs := strings.Split(u, "/")
	first := segs[0]
	if first == "nacos" || first == "api" {
		return false
	}
	if strings.HasPrefix(first, "_") {
		return true
	}
	if len(segs) == 1 {
		return true
	}
	if strings.HasPrefix(segs[1], "_") {
		return true
	}
	return strings.Contains(u, "/_index_template") || strings.Contains(u, "/_template/")
}

func isElasticsearchAuditNoise(path string) bool {
	if !isElasticsearchAuditPath(path) {
		return false
	}
	u := strings.ToLower(extractProxyUpstreamPath(path))
	noise := []string{
		"/_search", "/_msearch", "/_count", "/_sql", "/_explain",
		"/_field_caps", "/_mget", "/_analyze", "/_bulk", "/_doc",
		"/_update", "/_create", "/_cat/", "/_nodes", "/_cluster/health",
		"/_cluster/state", "/_xpack",
	}
	for _, item := range noise {
		if strings.Contains(u, item) {
			return true
		}
	}
	return false
}

func classifyElasticsearch(method, upstream string) auditRule {
	rule := defaultAuditRule(method, model.ObjectES)
	u := strings.ToLower(upstream)
	switch {
	case strings.Contains(u, "/_settings"), strings.Contains(u, "/_aliases"), strings.Contains(u, "/_mapping"),
		strings.HasSuffix(u, "/_close"), strings.HasSuffix(u, "/_open"):
		rule.Operation = auditlog.OperationUpdate
	case method == http.MethodPut && (isESIndexPath(u) || strings.Contains(u, "/_index_template") || strings.Contains(u, "/_template/")):
		rule.Operation = auditlog.OperationCreate
	}
	return rule
}

func isESIndexPath(upstream string) bool {
	u := strings.Trim(strings.ToLower(upstream), "/")
	segs := strings.Split(u, "/")
	return len(segs) == 1 && segs[0] != "" && !strings.HasPrefix(segs[0], "_")
}

type esResource struct {
	Name string
}

func extractElasticsearchResource(upstream string) esResource {
	u := strings.Trim(upstream, "/")
	if u == "" {
		return esResource{}
	}
	segs := strings.Split(u, "/")
	for i, seg := range segs {
		if seg == "_index_template" || seg == "_template" {
			if i+1 < len(segs) {
				return esResource{Name: segs[i+1]}
			}
		}
	}
	if len(segs) > 0 && !strings.HasPrefix(segs[0], "_") {
		return esResource{Name: segs[0]}
	}
	return esResource{}
}

func esAuditDetail(res esResource) string {
	if res.Name == "" {
		return ""
	}
	return "index=" + res.Name
}

// ---------- RabbitMQ ----------

func isRabbitMQAuditPath(path string) bool {
	u := strings.ToLower(extractProxyUpstreamPath(path))
	prefixes := []string{"/api/queues", "/api/exchanges", "/api/bindings", "/api/vhosts", "/api/users", "/api/permissions"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(u, prefix) {
			return true
		}
	}
	return false
}

func classifyRabbitMQ(method, upstream string) auditRule {
	rule := defaultAuditRule(method, model.ObjectRabbitMQ)
	if method == http.MethodPut || method == http.MethodPatch {
		rule.Operation = auditlog.OperationCreate
	}
	u := strings.ToLower(upstream)
	if strings.Contains(u, "/contents") {
		rule.Operation = auditlog.OperationDelete
	}
	return rule
}

type rabbitResource struct {
	Name  string
	Vhost string
}

func extractRabbitMQResource(upstream string) rabbitResource {
	u := strings.Trim(upstream, "/")
	segs := strings.Split(u, "/")
	unescape := func(raw string) string {
		if decoded, err := url.PathUnescape(raw); err == nil && decoded != "" {
			return decoded
		}
		return raw
	}
	res := rabbitResource{}
	if len(segs) >= 4 && (segs[1] == "queues" || segs[1] == "exchanges") {
		res.Vhost = unescape(segs[2])
		res.Name = unescape(segs[3])
		return res
	}
	if len(segs) >= 5 && segs[1] == "bindings" {
		res.Vhost = unescape(segs[2])
		res.Name = unescape(segs[4]) + "->" + unescape(lastNonEmpty(segs))
		return res
	}
	if len(segs) >= 3 && segs[1] == "vhosts" {
		res.Name = unescape(segs[2])
		res.Vhost = res.Name
	}
	return res
}

func lastNonEmpty(segs []string) string {
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] != "" {
			return segs[i]
		}
	}
	return ""
}

func rabbitAuditDetail(res rabbitResource) string {
	parts := make([]string, 0, 2)
	if res.Name != "" {
		parts = append(parts, "name="+res.Name)
	}
	if res.Vhost != "" {
		parts = append(parts, "vhost="+res.Vhost)
	}
	return strings.Join(parts, " ")
}

// ---------- Redis ----------

func extractRedisKey(c *gin.Context, body []byte) string {
	if c.Request != nil && c.Request.URL != nil {
		if key := strings.TrimSpace(c.Query("key")); key != "" {
			return key
		}
		if key := strings.TrimSpace(c.Query("username")); key != "" {
			return key
		}
	}
	return lookupJSONValue(decodeAuditBody(body), "key", "username")
}

// ---------- 对象存储 ----------

type storageResource struct {
	Bucket string
	Name   string
}

func extractStorageResource(c *gin.Context, body []byte) storageResource {
	if c == nil {
		return storageResource{}
	}
	query := url.Values{}
	if c.Request != nil && c.Request.URL != nil {
		query = c.Request.URL.Query()
	}
	obj := decodeAuditBody(body)
	bucket := firstNonEmpty(
		c.Param("bucket"),
		strings.TrimSpace(query.Get("bucket")),
		lookupJSONValue(obj, "bucket"),
	)
	name := firstNonEmpty(
		strings.TrimSpace(query.Get("key")),
		lookupJSONValue(obj, "key"),
		lookupJSONValue(obj, "src"),
		lookupJSONValue(obj, "dst"),
		firstJSONStringList(obj, "keys"),
		lookupJSONValue(obj, "name"),
	)
	if bucket == "" {
		bucket = lookupJSONValue(obj, "name")
	}
	if name == "" {
		name = bucket
	}
	return storageResource{Bucket: bucket, Name: name}
}

// firstJSONStringList 把 JSON 字符串数组折叠成逗号分隔文本，最多取 3 项。
func firstJSONStringList(obj map[string]interface{}, key string) string {
	arr, ok := obj[key].([]interface{})
	if !ok || len(arr) == 0 {
		return ""
	}
	out := make([]string, 0, 3)
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return ""
	}
	if len(arr) > len(out) {
		return strings.Join(out, ", ") + "..."
	}
	return strings.Join(out, ", ")
}
