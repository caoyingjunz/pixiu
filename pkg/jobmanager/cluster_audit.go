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

package jobmanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"

	auditlog "github.com/caoyingjunz/pixiu/pkg/audit"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	// auditOperatorSystem 系统事件统一的操作人，避免混入真实用户。
	auditOperatorSystem = "系统"
	// auditTimeLayout 审计详情中的时间格式。
	auditTimeLayout = "2006-01-02 15:04:05"
)

// updateClusterProbe 提交探测结果，并在失联/恢复的翻转点各写一条审计。
// 探测结果、失联周期标记和审计在同一事务内提交，锁内丢弃过期或配置已变化的探测结果。
func updateClusterProbe(ctx context.Context, factory db.ShareDaoFactory, cluster model.Cluster, updates map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	probeStatus, ok := updates["probe_status"].(model.ClusterProbeStatus)
	if !ok || (probeStatus != model.ClusterProbeHealthy && probeStatus != model.ClusterProbeUnhealthy) {
		return fmt.Errorf("invalid cluster probe status")
	}
	probeTime, ok := updates["last_probe_time"].(time.Time)
	if !ok || probeTime.IsZero() {
		return fmt.Errorf("invalid cluster probe time")
	}
	probeTime = probeTime.Truncate(time.Microsecond)
	updates["last_probe_time"] = probeTime

	var written string
	err := factory.Transaction(ctx, func(tx db.ShareDaoFactory) error {
		current, err := tx.Cluster().Get(ctx, cluster.Id, db.WithForUpdate())
		if err != nil {
			return err
		}
		if current == nil || current.ClusterStatus != cluster.ClusterStatus ||
			current.ConnectMode != cluster.ConnectMode || current.PermissionId != 0 ||
			current.ResourceVersion != cluster.ResourceVersion || current.KubeConfig != cluster.KubeConfig ||
			current.AgentToken != cluster.AgentToken {
			return nil
		}
		if current.LastProbeTime != nil && !probeTime.After(*current.LastProbeTime) {
			return nil
		}

		status, ok := updates["status"].(model.ClusterStatus)
		if !ok {
			status = current.ClusterStatus
		}
		disconnected := status == model.ClusterStatusError && probeStatus == model.ClusterProbeUnhealthy
		// 同一个失联周期只记一条；状态从非失联转入失联时，即使标记残留也要重新记录。
		recordDisconnect := disconnected &&
			(!current.DisconnectAudited || current.ClusterStatus != model.ClusterStatusError)
		// 恢复与失联成对记录，否则最近动态只能看到"掉了"，看不到"回来了"。
		recordReconnect := probeStatus == model.ClusterProbeHealthy &&
			current.ClusterStatus == model.ClusterStatusError && status != model.ClusterStatusError

		switch {
		case probeStatus == model.ClusterProbeHealthy:
			updates["disconnect_audited"] = false
		case recordDisconnect:
			updates["disconnect_audited"] = true
		}
		if err := tx.Cluster().InternalUpdate(ctx, cluster.Id, updates); err != nil {
			return err
		}

		switch {
		case recordDisconnect:
			audit := buildDisconnectAudit(current, probeString(updates, "probe_reason"),
				probeString(updates, "probe_message"), probeTime)
			if err := auditlog.Write(ctx, tx, audit); err != nil {
				return err
			}
			written = fmt.Sprintf("%s(%d) disconnect recorded", current.Name, current.Id)
			return nil
		case recordReconnect:
			if err := auditlog.Write(ctx, tx, buildReconnectAudit(current, probeTime)); err != nil {
				return err
			}
			written = fmt.Sprintf("%s(%d) reconnect recorded", current.Name, current.Id)
			return nil
		}
		return nil
	})
	// 只有事务提交成功才打印事件日志，避免回滚时留下误导性的"已记录"。
	if err == nil && written != "" {
		klog.Infof("[cluster-audit] %s", written)
	}
	return err
}

// backfillDisconnectAudit 为"已是失联状态但从未写过审计"的集群补一条失联记录。
// 典型场景是升级前就失联、或隧道集群未被本实例接管（不会走正常探测），
// 文案与正常失联完全一致，靠 disconnect_audited 在事务内去重。
func backfillDisconnectAudit(ctx context.Context, factory db.ShareDaoFactory, cluster model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var written string
	err := factory.Transaction(ctx, func(tx db.ShareDaoFactory) error {
		current, err := tx.Cluster().Get(ctx, cluster.Id, db.WithForUpdate())
		if err != nil {
			return err
		}
		if current == nil || current.ClusterStatus != model.ClusterStatusError ||
			current.ConnectMode != cluster.ConnectMode || current.PermissionId != 0 ||
			current.DisconnectAudited {
			return nil
		}

		if err := tx.Cluster().InternalUpdate(ctx, current.Id,
			map[string]interface{}{"disconnect_audited": true}); err != nil {
			return err
		}
		audit := buildDisconnectAudit(current, probeReasonAgentDisconnected, "", time.Now())
		if err := auditlog.Write(ctx, tx, audit); err != nil {
			return err
		}
		written = fmt.Sprintf("%s(%d) disconnect recorded by backfill", current.Name, current.Id)
		return nil
	})
	if err == nil && written != "" {
		klog.Infof("[cluster-audit] %s", written)
	}
	return err
}

// buildDisconnectAudit 生成失联审计，详情带上探测失败原因与已失联时长。
func buildDisconnectAudit(current *model.Cluster, reason, message string, probeTime time.Time) *model.Audit {
	detail := "控制面连通性探测失败"
	if reason == probeReasonAgentDisconnected {
		detail = "Agent 隧道已断开，集群失联"
	}
	if msg := strings.TrimSpace(message); msg != "" {
		detail += "：" + msg
	}
	if since, ok := lastHealthyAt(current); ok {
		detail += fmt.Sprintf("；最后正常探测于 %s，已失联约 %s",
			since.Format(auditTimeLayout), formatProbeAge(probeTime.Sub(since)))
	} else {
		detail += "；实际失联开始时间未知"
	}
	return buildClusterAudit(current, auditlog.OperationDisconnect, model.AuditOpFail, detail)
}

// buildReconnectAudit 生成恢复审计，详情带上本次失联时长。
func buildReconnectAudit(current *model.Cluster, probeTime time.Time) *model.Audit {
	detail := "集群控制面恢复连通"
	if since, ok := lastHealthyAt(current); ok {
		detail += fmt.Sprintf("；本次失联约 %s", formatProbeAge(probeTime.Sub(since)))
	}
	return buildClusterAudit(current, auditlog.OperationReconnect, model.AuditOpSuccess, detail)
}

func buildClusterAudit(current *model.Cluster, operation string,
	status model.AuditOperationStatus, detail string) *model.Audit {
	return &model.Audit{
		Action:       model.AuditActionSystem,
		Operator:     auditOperatorSystem,
		Path:         fmt.Sprintf("/pixiu/clusters/%d", current.Id),
		ObjectType:   model.ObjectCluster,
		Status:       status,
		Cluster:      current.Name,
		ResourceName: clusterDisplayName(current),
		ResourceId:   current.Id,
		Operation:    operation,
		Detail:       truncateText(detail),
	}
}

// lastHealthyAt 推断最后一次正常连通时间：优先用显式记录，
// 旧数据回退到"上一次探测健康"的探测时间。零值（0000-00-00）视为无记录。
func lastHealthyAt(cluster *model.Cluster) (time.Time, bool) {
	if cluster.LastHealthyTime != nil && !cluster.LastHealthyTime.IsZero() {
		return *cluster.LastHealthyTime, true
	}
	if cluster.ProbeStatus == model.ClusterProbeHealthy && cluster.LastProbeTime != nil &&
		!cluster.LastProbeTime.IsZero() {
		return *cluster.LastProbeTime, true
	}
	return time.Time{}, false
}

func clusterDisplayName(cluster *model.Cluster) string {
	if cluster.AliasName != "" {
		return cluster.AliasName
	}
	return cluster.Name
}

func probeString(updates map[string]interface{}, key string) string {
	if value, ok := updates[key].(string); ok {
		return value
	}
	return ""
}

// formatProbeAge 把失联时长格式化为便于阅读的中文描述。
// 负值（多实例时钟偏差）按不足 1 分钟处理，超长按年封顶，避免出现离谱数字。
func formatProbeAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "不足 1 分钟"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时", int(d.Hours()))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d 天", int(d.Hours()/24))
	default:
		return "超过 1 年"
	}
}
