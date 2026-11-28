/*
Copyright 2024 The Pixiu Authors.

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

// 审计中间件按职责拆分：本文件是入口与记录组装，规则见 audit_rule.go、
// K8s 代理解析见 audit_k8s_proxy.go、代理数据源见 audit_proxy_datasource.go、备份见 audit_backup.go。

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	auditlog "github.com/caoyingjunz/pixiu/pkg/audit"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const maxProxyBodySample = 4096

func Audit(o *options.Options) gin.HandlerFunc {
	recorder := o.AuditRecorder
	if recorder == nil {
		recorder = auditlog.NewRecorder(o.Factory)
	}
	return func(c *gin.Context) {
		if !shouldAudit(c) {
			c.Next()
			return
		}

		var proxySample []byte
		if shouldSampleAuditBody(c) {
			proxySample = sampleProxyBody(c)
		}
		datasourceID := parseDatasourceIDHeader(c)
		backupHint := captureBackupAuditHint(c, o.Factory, proxySample)

		startTime := time.Now()
		c.Next()
		respCode := auditResponseCode(c)
		if respCode == http.StatusTooManyRequests || respCode == http.StatusNotFound || respCode == http.StatusMethodNotAllowed {
			return
		}
		if httputils.ShouldSkipAudit(c) {
			return
		}
		recorder.Enqueue(buildAuditRecord(c, startTime, proxySample, datasourceID, backupHint))
	}
}

type auditBodyReader struct {
	io.Reader
	io.Closer
}

func sampleProxyBody(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}
	sampled, err := io.ReadAll(io.LimitReader(c.Request.Body, maxProxyBodySample))
	c.Request.Body = auditBodyReader{
		Reader: io.MultiReader(bytes.NewReader(sampled), c.Request.Body),
		Closer: c.Request.Body,
	}
	if err != nil {
		return nil
	}
	return sampled
}

// shouldSampleAuditBody 只对可能携带资源标识的写请求采样请求体。
func shouldSampleAuditBody(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/pixiu/extension/storage/") {
		return strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "json")
	}
	return strings.HasPrefix(path, "/pixiu/proxy/") ||
		strings.HasPrefix(path, "/pixiu/external/") ||
		strings.HasPrefix(path, "/pixiu/extension/redis/") ||
		strings.HasPrefix(path, "/pixiu/backup")
}

func buildAuditRecord(c *gin.Context, startTime time.Time, proxySample []byte, datasourceID int64, backupHint backupAuditHint) *model.Audit {
	responseCode := auditResponseCode(c)
	userName := "unknown"
	var operatorID int64
	if user, err := httputils.GetUserFromContext(c); err == nil && user != nil {
		userName = user.Name
		operatorID = user.Id
	} else if operator, err := httputils.GetAuditOperator(c); err == nil && operator != "" {
		userName = operator
	}

	objectType := detectObjectType(c)
	rule := classifyAudit(c.Request.Method, c.FullPath(), objectType)
	proxy := parseK8sProxyPathInfo(c.Request.URL.Path)
	if objectType != model.ObjectNacos && objectType != model.ObjectES && objectType != model.ObjectRabbitMQ {
		if proxy.Kind != "" {
			rule.Kind = proxy.Kind
		}
		applyProxyBodyHint(c.Request.Method, &rule, &proxy, inspectProxyBody(proxySample))
	}

	record := &model.Audit{
		RequestId:         requestid.Get(c),
		Action:            c.Request.Method,
		Ip:                c.ClientIP(),
		Operator:          userName,
		OperatorId:        operatorID,
		Path:              c.Request.RequestURI,
		ObjectType:        objectType,
		Status:            getAuditStatus(responseCode),
		ResponseCode:      responseCode,
		Cluster:           proxy.Cluster,
		ResourceName:      proxy.Name,
		ResourceNamespace: proxy.Namespace,
		ResourceId:        parseIntParam(c, "datasourceId", "clusterId", "planId", "backupId", "restoreId"),
	}
	upstream := extractProxyUpstreamPath(c.Request.URL.Path)
	switch objectType {
	case model.ObjectNacos:
		rule = classifyNacos(c.Request.Method, upstream)
		res := extractNacosResource(c, proxySample)
		record.ResourceName = res.Name
		record.ResourceNamespace = firstNonEmpty(res.Namespace, res.Group)
		record.Detail = nacosAuditDetail(res)
	case model.ObjectES:
		rule = classifyElasticsearch(c.Request.Method, upstream)
		res := extractElasticsearchResource(upstream)
		record.ResourceName = res.Name
		record.Detail = esAuditDetail(res)
	case model.ObjectRabbitMQ:
		rule = classifyRabbitMQ(c.Request.Method, upstream)
		res := extractRabbitMQResource(upstream)
		record.ResourceName = res.Name
		record.ResourceNamespace = res.Vhost
		record.Detail = rabbitAuditDetail(res)
	case model.ObjectRedis:
		record.ResourceName = firstNonEmpty(extractRedisKey(c, proxySample), record.ResourceName)
	case model.ObjectStorage:
		res := extractStorageResource(c, proxySample)
		record.ResourceName = firstNonEmpty(res.Name, record.ResourceName)
		record.ResourceNamespace = firstNonEmpty(res.Bucket, record.ResourceNamespace)
	}
	record.Operation = rule.Operation
	record.ResourceKind = rule.Kind
	record.ResourceName = firstNonEmpty(httputils.GetAuditSQLObject(c), record.ResourceName, c.Param("name"), c.Param("bucket"))
	record.ResourceNamespace = firstNonEmpty(record.ResourceNamespace, c.Param("namespace"))
	record.Cluster = firstNonEmpty(record.Cluster, c.Param("cluster"))
	record.Detail = firstNonEmpty(httputils.GetAuditDetail(c), record.Detail)

	if record.ResourceId == 0 {
		record.ResourceId = datasourceID
	}
	if backupHint.ResourceID > 0 {
		record.ResourceId = backupHint.ResourceID
	}
	record.ResourceName = firstNonEmpty(backupHint.ResourceName, record.ResourceName)
	record.Cluster = firstNonEmpty(backupHint.Cluster, record.Cluster)
	record.Duration = time.Since(startTime).Milliseconds()
	return record
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseIntParam 按顺序取第一个合法的路径参数，全部缺失时返回 0。
func parseIntParam(c *gin.Context, keys ...string) int64 {
	for _, key := range keys {
		raw := c.Param(key)
		if raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func shouldAudit(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions || c.Request.Method == http.MethodHead {
		return false
	}
	path := c.Request.URL.Path
	if !strings.HasPrefix(path, "/pixiu") {
		return false
	}
	return !isAuditNoise(path, c.FullPath())
}

func detectObjectType(c *gin.Context) model.ObjectType {
	if ot := detectObjectTypeFromPath(c.Request.URL.Path); ot != "" {
		return ot
	}
	obj, _, ok := httputils.GetObjectFromRequest(c)
	if !ok {
		return model.ObjectAll
	}
	ot := model.ObjectType(obj)
	if _, exists := model.ObjectTypeMap[ot]; exists {
		return ot
	}
	return model.ObjectAll
}

func auditResponseCode(c *gin.Context) int {
	if code := httputils.GetResponseCode(c); code != 0 {
		return code
	}
	return c.Writer.Status()
}

func getAuditStatus(code int) model.AuditOperationStatus {
	if code == 0 {
		return model.AuditOpUnknown
	}
	if code >= http.StatusOK && code < http.StatusMultipleChoices {
		return model.AuditOpSuccess
	}
	return model.AuditOpFail
}
