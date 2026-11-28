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

// 本文件是审计的规则中心：操作/资源类型的判定表、噪音过滤、资源类型推断。

import (
	"regexp"
	"strings"

	auditlog "github.com/caoyingjunz/pixiu/pkg/audit"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// auditRule 描述一条审计记录的操作类型与资源类型。
type auditRule struct {
	Operation string
	Kind      string
}

// auditRules 覆盖语义不等价于 HTTP 方法的接口，key 为 "METHOD 路由模板"。
var auditRules = map[string]auditRule{
	"POST /pixiu/auth/login":  {Operation: auditlog.OperationLogin},
	"POST /pixiu/auth/logout": {Operation: auditlog.OperationLogout},

	"POST /pixiu/extension/mysql/:datasourceId/query":       {Operation: auditlog.OperationExecute},
	"POST /pixiu/extension/mysql/:datasourceId/query/batch": {Operation: auditlog.OperationExecute},
	"POST /pixiu/extension/mysql/:datasourceId/import":      {Operation: auditlog.OperationImport},
	"POST /pixiu/extension/mysql/:datasourceId/backup":      {Operation: auditlog.OperationBackup},
	"POST /pixiu/extension/mysql/:datasourceId/backup/file": {Operation: auditlog.OperationBackup},
	"POST /pixiu/extension/mysql/:datasourceId/grant":       {Operation: auditlog.OperationGrant},
	"DELETE /pixiu/extension/mysql/:datasourceId/sessions":  {Operation: auditlog.OperationKill},

	"POST /pixiu/extension/postgres/:datasourceId/query":       {Operation: auditlog.OperationExecute},
	"POST /pixiu/extension/postgres/:datasourceId/batch":       {Operation: auditlog.OperationExecute},
	"POST /pixiu/extension/postgres/:datasourceId/import":      {Operation: auditlog.OperationImport},
	"POST /pixiu/extension/postgres/:datasourceId/backup/file": {Operation: auditlog.OperationBackup},
	"POST /pixiu/extension/postgres/:datasourceId/grant":       {Operation: auditlog.OperationGrant},
	"DELETE /pixiu/extension/postgres/:datasourceId/sessions":  {Operation: auditlog.OperationKill},
	"POST /pixiu/extension/postgres/:datasourceId/alter-table": {Operation: auditlog.OperationUpdate},

	"POST /pixiu/extension/redis/:datasourceId/key/ttl":        {Operation: auditlog.OperationUpdate},
	"POST /pixiu/extension/redis/:datasourceId/acl/user/grant": {Operation: auditlog.OperationGrant},
	"DELETE /pixiu/extension/redis/:datasourceId/clients":      {Operation: auditlog.OperationKill},

	"POST /pixiu/extension/storage/:datasourceId/buckets":                    {Kind: "Bucket"},
	"DELETE /pixiu/extension/storage/:datasourceId/buckets/:bucket":          {Kind: "Bucket"},
	"PUT /pixiu/extension/storage/:datasourceId/objects":                     {Operation: auditlog.OperationUpload, Kind: "Object"},
	"POST /pixiu/extension/storage/:datasourceId/objects/delete":             {Operation: auditlog.OperationDelete, Kind: "Object"},
	"POST /pixiu/extension/storage/:datasourceId/objects/rename":             {Operation: auditlog.OperationUpdate, Kind: "Object"},
	"POST /pixiu/extension/storage/:datasourceId/objects/transfer":           {Operation: auditlog.OperationUpdate, Kind: "Object"},
	"POST /pixiu/extension/storage/:datasourceId/multipart-uploads":          {Operation: auditlog.OperationUpload, Kind: "Object"},
	"POST /pixiu/extension/storage/:datasourceId/multipart-uploads/complete": {Operation: auditlog.OperationUpload, Kind: "Object"},
	"POST /pixiu/extension/storage/:datasourceId/object-versions/restore":    {Operation: auditlog.OperationRestore, Kind: "Object"},

	"POST /pixiu/backup/plans":                   {Kind: "BackupPlan"},
	"PUT /pixiu/backup/plans/:planId":            {Kind: "BackupPlan"},
	"DELETE /pixiu/backup/plans/:planId":         {Kind: "BackupPlan"},
	"POST /pixiu/backup/plans/:planId/run":       {Operation: auditlog.OperationBackup},
	"POST /pixiu/backup/restores":                {Kind: "Restore"},
	"PUT /pixiu/backup/restores/:restoreId":      {Kind: "Restore"},
	"POST /pixiu/backup/restores/:restoreId/run": {Operation: auditlog.OperationRestore, Kind: "Restore"},
	"DELETE /pixiu/backup/restores/:restoreId":   {Kind: "Restore"},

	"POST /pixiu/apps/charts":                                                   {Operation: auditlog.OperationUpload, Kind: "Chart"},
	"DELETE /pixiu/apps/charts/:id":                                             {Kind: "Chart"},
	"POST /pixiu/apps/clusters/:cluster/namespaces/:namespace/releases":         {Kind: "Release"},
	"PUT /pixiu/apps/clusters/:cluster/namespaces/:namespace/releases/:name":    {Kind: "Release"},
	"DELETE /pixiu/apps/clusters/:cluster/namespaces/:namespace/releases/:name": {Kind: "Release"},
}

// k8sAuditReviewPath 匹配 K8s 鉴权自检接口（SelfSubjectAccessReview 等），此类请求不记审计。
var k8sAuditReviewPath = regexp.MustCompile(model.K8sAuditReviewPathPattern)

// isAuditNoise 过滤探活、心跳、内部轮询等无审计价值的请求。
// 数据源相关的判定只在代理链路（external/proxy）上执行，避免普通请求承担多余开销。
func isAuditNoise(path, fullPath string) bool {
	switch fullPath {
	case "/pixiu/clusters/ping",
		"/pixiu/alerts/channels/ping",
		"/pixiu/extension/mysql/ping",
		"/pixiu/extension/postgres/ping",
		"/pixiu/extension/redis/ping",
		"/pixiu/extension/storage/ping",
		"/pixiu/agents/heartbeat",
		"/pixiu/agents/jobs/:jobId/logs",
		"/pixiu/agents/jobs/:jobId/result",
		"/pixiu/auth/refresh",
		"/pixiu/auth/verification-codes",
		"/pixiu/auth/forgot-password/verification-codes":
		return true
	}
	if !strings.HasPrefix(path, "/pixiu/external/") && !strings.HasPrefix(path, "/pixiu/proxy/") {
		return false
	}
	if strings.HasPrefix(path, "/pixiu/proxy/") && k8sAuditReviewPath.MatchString(path) {
		return true
	}
	return isNacosAuditNoise(path) || isElasticsearchAuditNoise(path)
}

// detectObjectTypeFromPath 按路径前缀推断资源类型，未命中返回空串交由上层兜底。
func detectObjectTypeFromPath(path string) model.ObjectType {
	if strings.HasPrefix(path, "/pixiu/backup") {
		return model.ObjectBackup
	}
	if strings.HasPrefix(path, "/pixiu/extension/") {
		rest := strings.TrimPrefix(path, "/pixiu/extension/")
		switch {
		case strings.HasPrefix(rest, "mysql"):
			return model.ObjectMySQL
		case strings.HasPrefix(rest, "postgres"):
			return model.ObjectPostgres
		case strings.HasPrefix(rest, "redis"):
			return model.ObjectRedis
		case strings.HasPrefix(rest, "storage"):
			return model.ObjectStorage
		}
		return ""
	}
	// 只有代理链路才可能命中 Nacos/ES/RabbitMQ 等外部数据源
	if !strings.HasPrefix(path, "/pixiu/external/") && !strings.HasPrefix(path, "/pixiu/proxy/") {
		return ""
	}
	if isNacosAuditPath(path) {
		return model.ObjectNacos
	}
	if isRabbitMQAuditPath(path) {
		return model.ObjectRabbitMQ
	}
	if isElasticsearchAuditPath(path) {
		return model.ObjectES
	}
	if strings.HasPrefix(path, "/pixiu/proxy/") {
		return model.ObjectContainerProxy
	}
	return model.ObjectExternalProxy
}

// classifyAudit 先取方法兜底规则，再用 auditRules 的显式配置覆盖。
func classifyAudit(method, fullPath string, objectType model.ObjectType) auditRule {
	rule := defaultAuditRule(method, objectType)
	if override, ok := auditRules[method+" "+fullPath]; ok {
		rule.Operation = firstNonEmpty(override.Operation, rule.Operation)
		rule.Kind = firstNonEmpty(override.Kind, rule.Kind)
	}
	return rule
}

// defaultAuditRule 按 HTTP 方法与资源类型给出兜底规则。
func defaultAuditRule(method string, objectType model.ObjectType) auditRule {
	return auditRule{Operation: auditlog.OperationForMethod(method), Kind: auditlog.ResourceKind(objectType)}
}
