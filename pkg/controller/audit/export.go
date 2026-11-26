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

package audit

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	auditlog "github.com/caoyingjunz/pixiu/pkg/audit"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const auditExportBatch = 2000

var csvBOM = []byte{0xEF, 0xBB, 0xBF}

var auditCSVHeader = []string{
	"时间",
	"用户",
	"操作类型",
	"结果",
	"资源",
	"对象",
	"来源 IP",
	"耗时(ms)",
	"响应码",
	"请求 ID",
	"摘要",
}

var auditOperationLabel = map[string]string{
	auditlog.OperationCreate:     "创建",
	auditlog.OperationUpdate:     "更新",
	auditlog.OperationDelete:     "删除",
	auditlog.OperationRestart:    "重启",
	auditlog.OperationScale:      "扩缩容",
	auditlog.OperationExecute:    "执行 SQL",
	auditlog.OperationBackup:     "备份",
	auditlog.OperationRestore:    "恢复",
	auditlog.OperationImport:     "导入",
	auditlog.OperationGrant:      "授权",
	auditlog.OperationLogin:      "登录",
	auditlog.OperationLogout:     "登出",
	auditlog.OperationKill:       "终止会话",
	auditlog.OperationUpload:     "上传",
	auditlog.OperationDisconnect: "失联",
	auditlog.OperationReconnect:  "恢复连接",
}

var auditObjectTypeLabel = map[string]string{
	"clusters":        "集群",
	"container_proxy": "集群资源",
	"mysql":           "MySQL",
	"postgres":        "PostgreSQL",
	"redis":           "Redis",
	"nacos":           "Nacos",
	"es":              "Elasticsearch",
	"rabbitmq":        "RabbitMQ",
	"storage":         "对象存储",
	"datasources":     "数据源",
	"users":           "用户",
	"tenants":         "租户",
	"auth":            "认证",
	"apps":            "应用商店",
	"plans":           "版本计划",
	"backup":          "备份",
	"BackupPlan":      "备份计划",
	"Backup":          "备份",
	"Restore":         "恢复",
	"Object":          "对象",
	"Bucket":          "存储桶",
	"*":               "-",
}

// AuditExport 审计 CSV 导出结果。
type AuditExport struct {
	Filename string
	Data     []byte
	Total    int64
	Count    int
}

func (a *audit) Export(ctx context.Context, listOption types.ListOptions) (*AuditExport, error) {
	filterOpts, err := a.buildFilterOpts(ctx, listOption, false)
	if err != nil {
		return nil, err
	}

	total, err := a.factory.Audit().Count(ctx, filterOpts...)
	if err != nil {
		klog.Errorf("failed to count audits for export: %v", err)
		return nil, errors.ErrServerInternal
	}

	var buf bytes.Buffer
	if _, err := buf.Write(csvBOM); err != nil {
		return nil, err
	}
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(auditCSVHeader); err != nil {
		return nil, err
	}

	count := 0
	var beforeID int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		opts := append([]db.Options{}, filterOpts...)
		opts = append(opts, db.WithLimit(auditExportBatch), db.WithOrderByDesc())
		if beforeID > 0 {
			opts = append(opts, db.WithIDBefore(beforeID))
		}
		objects, err := a.factory.Audit().List(ctx, opts...)
		if err != nil {
			klog.Errorf("failed to list audits for export: %v", err)
			return nil, errors.ErrServerInternal
		}
		if len(objects) == 0 {
			break
		}
		items, err := a.hydrate(ctx, objects)
		if err != nil {
			return nil, err
		}
		for i := range items {
			if err := w.Write(auditCSVRow(&items[i])); err != nil {
				return nil, err
			}
			count++
		}
		beforeID = objects[len(objects)-1].Id
		if len(objects) < auditExportBatch {
			break
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return &AuditExport{
		Filename: auditExportFilename(time.Now()),
		Data:     buf.Bytes(),
		Total:    total,
		Count:    count,
	}, nil
}

func auditCSVRow(item *types.Audit) []string {
	target := formatAuditTargetView(item)
	secondary := target.secondary
	if secondary == "" {
		secondary = "-"
	}
	code := "-"
	if item.ResponseCode > 0 {
		code = strconv.Itoa(item.ResponseCode)
	}
	return []string{
		csvCell(formatAuditTime(item.GmtCreate)),
		csvCell(auditOperator(item)),
		csvCell(auditOperationLabelText(item.Operation)),
		csvCell(auditStatusLabel(item.Status)),
		csvCell(target.primary),
		csvCell(secondary),
		csvCell(emptyDash(item.Ip)),
		csvCell(strconv.FormatInt(item.Duration, 10)),
		csvCell(code),
		csvCell(emptyDash(item.RequestId)),
		csvCell(item.Detail),
	}
}

func auditExportFilename(now time.Time) string {
	return now.Local().Format("pixiu-audit-20060102-150405.csv")
}

func formatAuditTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func auditOperator(item *types.Audit) string {
	if item.OperatorName != "" {
		return item.OperatorName
	}
	if item.Operator != "" {
		return item.Operator
	}
	return "-"
}

func auditOperationLabelText(op string) string {
	if op == "" {
		return "操作"
	}
	if label, ok := auditOperationLabel[op]; ok {
		return label
	}
	return op
}

func auditStatusLabel(status model.AuditOperationStatus) string {
	switch status {
	case model.AuditOpFail:
		return "失败"
	case model.AuditOpSuccess:
		return "成功"
	default:
		return "未知"
	}
}

type auditTargetView struct {
	primary   string
	secondary string
}

func formatAuditTargetView(item *types.Audit) auditTargetView {
	kind := resourceKindLabel(item)
	instance := item.ResourceAlias
	name := item.ResourceName
	ns := item.ResourceNamespace
	cluster := item.ClusterAlias
	if cluster == "" {
		cluster = item.Cluster
	}
	objectLevel := item.ObjectType.IsDatasource() && item.ObjectType != model.ObjectDatasource
	isK8s := item.ObjectType == model.ObjectContainerProxy ||
		(ns != "" && cluster != "" && !objectLevel && item.ObjectType != model.ObjectBackup)

	if item.ObjectType == model.ObjectStorage {
		primary := "对象存储"
		if instance != "" {
			primary = "对象存储 / " + instance
		}
		secondary := ""
		if name != "" && name != instance {
			secondary = name
		}
		if ns != "" && secondary != "" && secondary != ns && !strings.HasPrefix(secondary, ns+"/") {
			secondary = ns + "/" + secondary
		} else if secondary == "" && ns != "" {
			secondary = ns
		}
		return auditTargetView{primary: primary, secondary: secondary}
	}

	if isK8s && (kind != "" || instance != "" || name != "" || cluster != "") {
		display := instance
		if display == "" {
			display = name
		}
		res := display
		if kind != "" && display != "" {
			res = kind + "/" + display
		} else if kind != "" {
			res = kind
		}
		loc := joinNonEmpty(" / ", cluster, ns)
		if loc != "" && res != "" {
			return auditTargetView{primary: loc, secondary: res}
		}
		if loc != "" {
			return auditTargetView{primary: loc}
		}
		if res != "" {
			return auditTargetView{primary: res}
		}
		return auditTargetView{primary: "-"}
	}

	if objectLevel {
		parts := make([]string, 0, 2)
		if kind != "" && kind != "-" {
			parts = append(parts, kind)
		}
		if instance != "" && instance != kind {
			parts = append(parts, instance)
		}
		primary := strings.Join(parts, " / ")
		if primary == "" {
			primary = emptyDash(kind)
		}
		secondary := ""
		if name != "" && name != instance {
			secondary = name
		}
		return auditTargetView{primary: primary, secondary: secondary}
	}

	if item.ObjectType == model.ObjectBackup {
		kindLabel := kind
		if kindLabel == "" || kindLabel == "-" {
			kindLabel = "备份"
		}
		primary := kindLabel
		if cluster != "" {
			primary = kindLabel + " / " + cluster
		}
		objectName := name
		if objectName == "" {
			objectName = instance
		}
		secondary := ""
		if objectName != "" && objectName != cluster {
			secondary = objectName
		}
		return auditTargetView{primary: primary, secondary: secondary}
	}

	display := instance
	if display == "" {
		display = name
	}
	if kind != "" && kind != "-" && display != "" {
		return auditTargetView{primary: kind + " / " + display}
	}
	if display != "" {
		return auditTargetView{primary: display}
	}
	if kind != "" && kind != "-" {
		return auditTargetView{primary: kind}
	}
	if label := auditObjectTypeLabel[string(item.ObjectType)]; label != "" && label != "-" {
		return auditTargetView{primary: label}
	}
	return auditTargetView{primary: "-"}
}

func resourceKindLabel(item *types.Audit) string {
	raw := item.ResourceKind
	if raw != "" {
		if label, ok := auditObjectTypeLabel[raw]; ok {
			return label
		}
		return raw
	}
	if label, ok := auditObjectTypeLabel[string(item.ObjectType)]; ok {
		return label
	}
	return ""
}

func csvCell(v string) string {
	if v == "" {
		return ""
	}
	switch v[0] {
	case '=', '+', '@', '\t', '\r':
		return "'" + v
	case '-':
		if v != "-" {
			return "'" + v
		}
	}
	return v
}

func emptyDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func joinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, sep)
}
