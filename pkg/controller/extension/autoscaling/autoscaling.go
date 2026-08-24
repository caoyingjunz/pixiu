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

// Package autoscaling 提供定时扩缩容（CronHPA）管理能力：
// 规则持久化在 pixiu 数据库，由后端 jobmanager 周期评估并到点直接操作集群，
// 无需在集群侧安装第三方控制器。
package autoscaling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	// maxJobsPerCronHpa 单条规则的定时任务数上限
	maxJobsPerCronHpa = 20
	// maxTargetSize 目标副本数上限
	maxTargetSize = int32(1000)
	// catchupMaxSteps 补跑推进的最大迭代次数，防御异常 cron 表达式造成死循环
	catchupMaxSteps = 5000
	// dbOpTimeout 评估轮内单次 DB 操作超时，避免慢 SQL 拖长整轮评估、
	// 导致 SkipIfStillRunning 跳过后续节拍而丢失触发
	dbOpTimeout = 10 * time.Second
	// 执行历史查询限制
	historyDefaultLimit = 100
	historyMaxLimit     = 500
	// 单次集群操作超时
	kubeOpTimeout = 30 * time.Second
	// 目标存在性预检超时
	targetProbeTimeout = 15 * time.Second
)

type Interface interface {
	Create(ctx context.Context, req *types.CronHpaRequest) (*types.CronHpa, error)
	Get(ctx context.Context, id int64) (*types.CronHpa, error)
	List(ctx context.Context, opts *types.CronHpaListOptions) ([]types.CronHpa, error)
	Update(ctx context.Context, id int64, req *types.CronHpaRequest) (*types.CronHpa, error)
	Delete(ctx context.Context, id int64) error
	SetStatus(ctx context.Context, id int64, status string) error
	ListHistories(ctx context.Context, id int64, limit int) ([]types.CronHpaHistory, error)
	// EvaluateOnce 扫描所有启用规则并执行到期任务，由 jobmanager 周期调用
	EvaluateOnce(ctx context.Context) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:      cfg,
		factory: f,
	}
}

func (c *controller) Create(ctx context.Context, req *types.CronHpaRequest) (*types.CronHpa, error) {
	if err := validateCronHpaRequest(req); err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	cluster, err := c.getCluster(ctx, req.ClusterName)
	if err != nil {
		return nil, err
	}
	if err = c.checkTargetExists(ctx, cluster, req); err != nil {
		return nil, err
	}
	// 同集群同命名空间下规则名唯一
	existing, err := c.factory.CronHpa().List(ctx,
		db.WithClusterName(req.ClusterName), db.WithNamespace(req.Namespace), db.WithName(req.Name))
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, apierrors.NewError(fmt.Errorf("定时扩缩容规则 %s 已存在", req.Name), http.StatusConflict)
	}

	now := time.Now()
	jobs := make([]types.CronHpaJob, 0, len(req.Jobs))
	for _, job := range req.Jobs {
		// 从创建时刻开始计时，首次触发为创建后的下一个调度点
		job.LastFireTime = &now
		job.State = types.CronHpaJobStateSubmitted
		job.Message = ""
		jobs = append(jobs, job)
	}
	jobsData, err := json.Marshal(jobs)
	if err != nil {
		return nil, err
	}
	excludeData, err := json.Marshal(req.ExcludeDates)
	if err != nil {
		return nil, err
	}

	object := &model.CronHpa{
		Name:         req.Name,
		ClusterName:  req.ClusterName,
		Namespace:    req.Namespace,
		TargetKind:   req.TargetKind,
		TargetName:   req.TargetName,
		Jobs:         string(jobsData),
		ExcludeDates: string(excludeData),
		Status:       model.CronHpaStatusActive,
		Description:  req.Description,
		CreateUser:   currentUser(ctx),
	}
	created, err := c.factory.CronHpa().Create(ctx, object)
	if err != nil {
		return nil, err
	}
	return convertCronHpa(created)
}

func (c *controller) Get(ctx context.Context, id int64) (*types.CronHpa, error) {
	object, err := c.factory.CronHpa().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("定时扩缩容规则不存在"), http.StatusNotFound)
	}
	return convertCronHpa(object)
}

func (c *controller) List(ctx context.Context, opts *types.CronHpaListOptions) ([]types.CronHpa, error) {
	dbOpts := []db.Options{}
	if opts != nil {
		dbOpts = append(dbOpts, db.WithClusterName(opts.Cluster), db.WithNamespace(opts.Namespace))
	}
	objects, err := c.factory.CronHpa().List(ctx, dbOpts...)
	if err != nil {
		return nil, err
	}
	result := make([]types.CronHpa, 0, len(objects))
	for i := range objects {
		item, convErr := convertCronHpa(&objects[i])
		if convErr != nil {
			klog.Warningf("[CronHpa] skip rule %d with invalid jobs data: %v", objects[i].Id, convErr)
			continue
		}
		result = append(result, *item)
	}
	return result, nil
}

func (c *controller) Update(ctx context.Context, id int64, req *types.CronHpaRequest) (*types.CronHpa, error) {
	object, err := c.factory.CronHpa().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("定时扩缩容规则不存在"), http.StatusNotFound)
	}
	if err = validateCronHpaRequest(req); err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	// 集群归属不可变更，始终使用规则原集群做目标校验
	cluster, err := c.getCluster(ctx, object.ClusterName)
	if err != nil {
		return nil, err
	}
	if err = c.checkTargetExists(ctx, cluster, req); err != nil {
		return nil, err
	}
	// 规则名唯一（排除自身）
	existing, err := c.factory.CronHpa().List(ctx,
		db.WithClusterName(object.ClusterName), db.WithNamespace(req.Namespace), db.WithName(req.Name))
	if err != nil {
		return nil, err
	}
	for _, e := range existing {
		if e.Id != id {
			return nil, apierrors.NewError(fmt.Errorf("定时扩缩容规则 %s 已存在", req.Name), http.StatusConflict)
		}
	}

	// 更新后所有任务重新计时，从更新时刻开始等待下一个调度点
	now := time.Now()
	jobs := make([]types.CronHpaJob, 0, len(req.Jobs))
	for _, job := range req.Jobs {
		job.LastFireTime = &now
		job.State = types.CronHpaJobStateSubmitted
		job.Message = ""
		jobs = append(jobs, job)
	}
	jobsData, err := json.Marshal(jobs)
	if err != nil {
		return nil, err
	}
	excludeData, err := json.Marshal(req.ExcludeDates)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":          req.Name,
		"namespace":     req.Namespace,
		"target_kind":   req.TargetKind,
		"target_name":   req.TargetName,
		"jobs":          string(jobsData),
		"exclude_dates": string(excludeData),
		"description":   req.Description,
	}
	if err = c.factory.CronHpa().InternalUpdate(ctx, id, updates); err != nil {
		return nil, err
	}
	return c.Get(ctx, id)
}

func (c *controller) Delete(ctx context.Context, id int64) error {
	object, err := c.factory.CronHpa().Get(ctx, id)
	if err != nil {
		return err
	}
	if object == nil {
		return apierrors.NewError(fmt.Errorf("定时扩缩容规则不存在"), http.StatusNotFound)
	}
	return c.factory.CronHpa().Delete(ctx, id)
}

func (c *controller) SetStatus(ctx context.Context, id int64, status string) error {
	object, err := c.factory.CronHpa().Get(ctx, id)
	if err != nil {
		return err
	}
	if object == nil {
		return apierrors.NewError(fmt.Errorf("定时扩缩容规则不存在"), http.StatusNotFound)
	}
	switch status {
	case types.CronHpaStatusActive, types.CronHpaStatusPaused:
	default:
		return apierrors.NewError(fmt.Errorf("status 仅支持 %s / %s", types.CronHpaStatusActive, types.CronHpaStatusPaused), http.StatusBadRequest)
	}
	if object.Status == model.CronHpaStatus(status) {
		return nil
	}
	return c.factory.CronHpa().InternalUpdate(ctx, id, map[string]interface{}{"status": status})
}

func (c *controller) ListHistories(ctx context.Context, id int64, limit int) ([]types.CronHpaHistory, error) {
	object, err := c.factory.CronHpa().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("定时扩缩容规则不存在"), http.StatusNotFound)
	}
	if limit <= 0 {
		limit = historyDefaultLimit
	}
	if limit > historyMaxLimit {
		limit = historyMaxLimit
	}
	histories, err := c.factory.CronHpa().ListHistories(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	result := make([]types.CronHpaHistory, 0, len(histories))
	for _, h := range histories {
		result = append(result, types.CronHpaHistory{
			Id:               h.Id,
			CronHpaId:        h.CronHpaId,
			JobName:          h.JobName,
			ScheduledTime:    h.ScheduledTime,
			ExecutedAt:       h.ExecutedAt,
			PreviousReplicas: h.PreviousReplicas,
			DesiredReplicas:  h.DesiredReplicas,
			Result:           h.Result,
			Message:          h.Message,
		})
	}
	return result, nil
}

// getCluster 按名称获取集群，不存在时返回 404
func (c *controller) getCluster(ctx context.Context, clusterName string) (*model.Cluster, error) {
	cluster, err := c.factory.Cluster().GetBy(ctx, db.WithName(clusterName))
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, apierrors.ErrClusterNotFound
	}
	return cluster, nil
}

// buildClusterSet 复用集群拨测同款机制构造客户端，直连/隧道集群均可达
func buildClusterSet(cluster *model.Cluster) (*client.ClusterSet, error) {
	return client.NewClusterSetWithOptions(cluster.KubeConfig, client.ClusterSetOptions{
		ClusterName: cluster.Name,
		ConnectMode: cluster.ConnectMode,
	})
}

// checkTargetExists 创建/更新前预检目标资源是否存在，尽早暴露配置错误
func (c *controller) checkTargetExists(ctx context.Context, cluster *model.Cluster, req *types.CronHpaRequest) error {
	cs, err := buildClusterSet(cluster)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("无法连接集群 %s: %v", req.ClusterName, err), http.StatusBadRequest)
	}
	probeCtx, cancel := context.WithTimeout(ctx, targetProbeTimeout)
	defer cancel()

	switch req.TargetKind {
	case types.CronHpaTargetKindDeployment:
		_, err = cs.Client.AppsV1().Deployments(req.Namespace).Get(probeCtx, req.TargetName, metav1.GetOptions{})
	case types.CronHpaTargetKindStatefulSet:
		_, err = cs.Client.AppsV1().StatefulSets(req.Namespace).Get(probeCtx, req.TargetName, metav1.GetOptions{})
	case types.CronHpaTargetKindHpa:
		_, err = cs.Client.AutoscalingV2().HorizontalPodAutoscalers(req.Namespace).Get(probeCtx, req.TargetName, metav1.GetOptions{})
	}
	if err != nil {
		return apierrors.NewError(fmt.Errorf("目标资源 %s/%s 不存在或不可访问: %v", req.TargetKind, req.TargetName, err), http.StatusBadRequest)
	}
	return nil
}

// validateCronHpaRequest 校验创建/更新请求的合法性
func validateCronHpaRequest(req *types.CronHpaRequest) error {
	if req == nil {
		return fmt.Errorf("请求体不能为空")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ClusterName = strings.TrimSpace(req.ClusterName)
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.TargetName = strings.TrimSpace(req.TargetName)
	if req.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if req.ClusterName == "" {
		return fmt.Errorf("集群名称不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.TargetName == "" {
		return fmt.Errorf("目标资源名称不能为空")
	}
	switch req.TargetKind {
	case types.CronHpaTargetKindDeployment, types.CronHpaTargetKindStatefulSet, types.CronHpaTargetKindHpa:
	default:
		return fmt.Errorf("目标类型仅支持 %s / %s / %s",
			types.CronHpaTargetKindDeployment, types.CronHpaTargetKindStatefulSet, types.CronHpaTargetKindHpa)
	}
	if len(req.Jobs) == 0 {
		return fmt.Errorf("至少需要一条定时任务")
	}
	if len(req.Jobs) > maxJobsPerCronHpa {
		return fmt.Errorf("定时任务数量不能超过 %d 条", maxJobsPerCronHpa)
	}
	names := make(map[string]struct{}, len(req.Jobs))
	for i := range req.Jobs {
		job := &req.Jobs[i]
		job.Name = strings.TrimSpace(job.Name)
		job.Schedule = strings.TrimSpace(job.Schedule)
		if job.Name == "" {
			return fmt.Errorf("第 %d 条定时任务名称不能为空", i+1)
		}
		if _, duplicated := names[job.Name]; duplicated {
			return fmt.Errorf("定时任务名称 %s 重复", job.Name)
		}
		names[job.Name] = struct{}{}
		if _, err := cron.ParseStandard(job.Schedule); err != nil {
			return fmt.Errorf("定时任务 %s 的 cron 表达式 %q 无效: %v", job.Name, job.Schedule, err)
		}
		if job.TargetSize < 0 || job.TargetSize > maxTargetSize {
			return fmt.Errorf("定时任务 %s 的目标副本数需在 0-%d 之间", job.Name, maxTargetSize)
		}
	}
	if req.ExcludeDates == nil {
		req.ExcludeDates = []string{}
	}
	for _, pattern := range req.ExcludeDates {
		if _, err := cron.ParseStandard(pattern); err != nil {
			return fmt.Errorf("排除日期表达式 %q 无效: %v", pattern, err)
		}
	}
	return nil
}

// currentUser 从请求上下文提取操作人（缺失时留空）
func currentUser(ctx context.Context) string {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil || user == nil {
		return ""
	}
	return user.Name
}

// convertCronHpa 数据库模型转前端对象（jobs/exclude_dates 反序列化）
func convertCronHpa(object *model.CronHpa) (*types.CronHpa, error) {
	var jobs []types.CronHpaJob
	if err := json.Unmarshal([]byte(object.Jobs), &jobs); err != nil {
		return nil, err
	}
	var excludeDates []string
	if object.ExcludeDates != "" && object.ExcludeDates != "null" {
		if err := json.Unmarshal([]byte(object.ExcludeDates), &excludeDates); err != nil {
			return nil, err
		}
	}
	return &types.CronHpa{
		Id:           object.Id,
		GmtCreate:    object.GmtCreate,
		GmtModified:  object.GmtModified,
		Name:         object.Name,
		ClusterName:  object.ClusterName,
		Namespace:    object.Namespace,
		TargetKind:   object.TargetKind,
		TargetName:   object.TargetName,
		Jobs:         jobs,
		ExcludeDates: excludeDates,
		Status:       string(object.Status),
		Description:  object.Description,
		CreateUser:   object.CreateUser,
	}, nil
}
