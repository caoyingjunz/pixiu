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

package clusterexpand

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	planctrl "github.com/caoyingjunz/pixiu/pkg/controller/plan"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/kubezansible"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

const defaultExpansionTimeout = 30 * time.Minute

// expansionTaskSpecs 扩容进度任务定义，下发时按此顺序创建、worker 按此顺序执行。
var expansionTaskSpecs = []struct {
	Name   string
	Action string
}{
	{planctrl.TaskNamePrepare, kubezansible.Runner},
	{planctrl.TaskNameRender, kubezansible.Render},
	{planctrl.TaskNamePrechecks, kubezansible.Prechecks},
	{planctrl.TaskNameBootstrap, kubezansible.Bootstrap},
	{planctrl.TaskNameDeployNode, kubezansible.AddNode},
}

var (
	expansionQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cluster-expansions")
	clusterLocks   sync.Map // map[int64]*sync.Mutex
)

func lockCluster(clusterId int64) func() {
	value, _ := clusterLocks.LoadOrStore(clusterId, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// createExpansionTaskIfNotExist 初始化扩容任务：不存在则创建，已存在则跳过。
// 任务的重置由 Start 在入队前统一完成（删除并重建），此处只兜底补齐缺失任务。
func (c *controller) createExpansionTaskIfNotExist(ctx context.Context, planId int64, name, action string) error {
	_, err := c.factory.Plan().Task().GetByName(ctx, planId, name)
	if err == nil {
		return nil
	}
	if !errors.IsRecordNotFound(err) {
		klog.Infof("failed to get plan(%d) task(%s) for init: %v", planId, name, err)
		return err
	}
	_, err = c.factory.Plan().Task().Create(ctx, &model.Task{
		Name:   name,
		PlanId: planId,
		Step:   model.UnStartedPlanStep,
		Action: action,
		Status: model.UnStartPlanStatus,
	})
	if err != nil {
		klog.Errorf("failed to init plan(%d) task(%s): %v", planId, name, err)
	}
	return err
}

func (c *controller) updateTask(ctx context.Context, planId int64, name string, updates map[string]interface{}) {
	if _, err := c.factory.Plan().Task().Update(ctx, planId, name, updates); err != nil {
		klog.Errorf("update plan(%d) task(%s): %v", planId, name, err)
	}
}

// markTaskRunning 将任务标记为进行中。
func (c *controller) markTaskRunning(ctx context.Context, planId int64, name string) {
	c.updateTask(ctx, planId, name, map[string]interface{}{"status": model.RunningPlanStatus, "step": model.RunningPlanStep, "message": "", "gmt_create": time.Now()})
}

// finishTask 按执行结果结束任务。
func (c *controller) finishTask(ctx context.Context, planId int64, name string, cause error) {
	if cause != nil {
		c.updateTask(ctx, planId, name, map[string]interface{}{
			"status": model.FailedPlanStatus, "step": model.FailedPlanStep, "message": cause.Error(), "gmt_modified": time.Now(),
		})
		return
	}
	c.updateTask(ctx, planId, name, map[string]interface{}{
		"status": model.SuccessPlanStatus, "step": model.CompletedPlanStep, "message": "执行成功", "gmt_modified": time.Now(),
	})
}

func (c *controller) updatePlanStatus(ctx context.Context, planId int64, status model.TaskStatus) {
	if err := c.factory.Plan().UpdateBy(ctx, map[string]interface{}{"status": status}, db.WithId(planId)); err != nil {
		klog.Errorf("update plan(%d) status to %s: %v", planId, status, err)
	}
}

func (c *controller) Run(ctx context.Context, workers int) error {
	klog.Infof("starting cluster expansion manager")
	if err := c.syncStatus(ctx); err != nil {
		return err
	}
	if workers <= 0 {
		workers = 3
	}
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}
	return nil
}

func (c *controller) worker(ctx context.Context) {
	for c.process(ctx) {
	}
}

func (c *controller) process(ctx context.Context) bool {
	key, quit := expansionQueue.Get()
	if quit {
		return false
	}
	defer expansionQueue.Done(key)

	c.syncHandler(ctx, key.(int64))
	return true
}

// syncStatus 进程启动时的状态恢复：运行中的扩容计划标记失败，添加中的节点标记失败。
// 「未开始」表示计划已保存但从未执行，属于空闲状态，不做处理。
func (c *controller) syncStatus(ctx context.Context) error {
	plans, err := c.factory.Plan().List(ctx, db.WithPlanTypeEq(1), db.WithTaskStatus(model.RunningPlanStatus))
	if err != nil {
		return err
	}
	for i := range plans {
		c.updatePlanStatus(ctx, plans[i].Id, model.FailedPlanStatus)
		// 服务重启中断时任务可能仍停留在「未开始/运行中」，补一条失败任务用于展示原因
		c.markFirstUnfinishedTaskFailed(ctx, plans[i].Id, "服务异常中断，请重新发起扩容")
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithNodeStatus(model.NodeStatusAdding))
	if err != nil {
		return err
	}
	for i := range nodes {
		if err = c.factory.Plan().Node().InternalUpdate(ctx, nodes[i].Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": "服务异常中断，请重试",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) syncHandler(ctx context.Context, planId int64) {
	plan, err := c.factory.Plan().Get(ctx, planId)
	if err != nil || plan == nil {
		klog.Errorf("get expansion plan(%d): %v", planId, err)
		return
	}

	if plan.Type != 1 {
		klog.Infof("skip expansion plan(%d)", planId)
		return
	}
	if plan.Status != model.RunningPlanStatus {
		klog.Infof("skip expansion plan(%d) running task ", planId)
		return
	}

	clusterObj, err := c.factory.Cluster().Get(ctx, plan.ClusterId)
	if err != nil || clusterObj == nil {
		c.failExpansionPlan(ctx, plan.Id, fmt.Errorf("集群不存在或已被删除"))
		return
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithPlanIdEq(plan.Id))
	if err != nil {
		c.failExpansionPlan(ctx, plan.Id, err)
		return
	}
	if len(nodes) == 0 {
		c.failExpansionPlan(ctx, plan.Id, fmt.Errorf("扩容节点为空"))
		return
	}

	info := c.detectClusterInfo(ctx, clusterObj)
	if !info.Capabilities.Supported {
		// 不支持扩容时直接失败，避免继续拉取镜像与渲染配置
		c.failExpansionPlan(ctx, plan.Id, fmt.Errorf("%s", info.Capabilities.Reason))
		return
	}

	for _, spec := range expansionTaskSpecs {
		if err = c.createExpansionTaskIfNotExist(ctx, plan.Id, spec.Name, spec.Action); err != nil {
			c.failExpansionPlan(ctx, plan.Id, fmt.Errorf("初始化扩容任务失败: %v", err))
			return
		}
	}

	// 更新状态，开始执行
	c.updatePlanStatus(ctx, plan.Id, model.RunningPlanStatus)

	// 执行参数以已保存的扩容计划配置为准，缺失项回落到集群能力探测默认值
	var options types.ExpansionOptions
	c.applyExpansionOptions(ctx, plan.Id, &options, info.Capabilities)

	timeout := time.Duration(c.cc.Worker.DeployTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultExpansionTimeout
	}

	task := &expandTask{
		plan:    plan,
		cluster: clusterObj,
		info:    info,
		options: &options,
		nodes:   nodes,
		// 与部署任务保持一致：使用 plan id 作为渲染目录
		dir: filepath.Join(c.cc.Worker.WorkDir, strconv.FormatInt(plan.Id, 10)),
	}

	handlers := []expandHandler{
		taskPrepare{c: c},
		taskRender{c: c},
		taskPrechecks{},
		taskBootstrap{c: c, timeout: timeout},
		taskAddNode{c: c, timeout: timeout},
	}
	if err = c.syncExpansionTasks(ctx, plan.Id, task, handlers...); err != nil {
		return
	}

	// 执行成功后按集群实时节点列表校正，确认节点确实已加入
	joined, reconcileErr := c.reconcileNodes(ctx, info.Client, nodes)
	if reconcileErr != nil {
		c.updatePlanStatus(ctx, plan.Id, model.FailedPlanStatus)
		klog.Errorf("cluster(%d) expansion plan(%d) failed: %v", plan.ClusterId, plan.Id, reconcileErr)
		return
	}
	if joined != len(nodes) {
		message := fmt.Sprintf("扩容未完全成功：%d/%d 个节点已加入集群", joined, len(nodes))
		c.updateTask(ctx, plan.Id, planctrl.TaskNameDeployNode, map[string]interface{}{
			"status": model.FailedPlanStatus, "step": model.FailedPlanStep, "message": message, "gmt_modified": time.Now(),
		})
		c.updatePlanStatus(ctx, plan.Id, model.FailedPlanStatus)
		return
	}

	c.updatePlanStatus(ctx, plan.Id, model.SuccessPlanStatus)
	klog.Infof("cluster(%d) expansion plan(%d) succeeded", plan.ClusterId, plan.Id)
}

func nodesToPlanNodes(nodes []model.Node) ([]types.PlanNode, error) {
	result := make([]types.PlanNode, 0, len(nodes))
	for i := range nodes {
		auth := types.PlanNodeAuth{}
		if err := auth.Unmarshal(nodes[i].Auth); err != nil {
			return nil, err
		}
		result = append(result, types.PlanNode{
			Name: nodes[i].Name,
			Ip:   nodes[i].Ip,
			Role: []string{model.NodeRole},
			CRI:  model.ContainerdCRI,
			Auth: auth,
		})
	}
	return result, nil
}

func runRunnerContainer(ctx context.Context, name, command, image, configDir string, timeout time.Duration) error {
	rt, err := runtime.Default()
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout+60*time.Second)
	defer cancel()

	return rt.RunContainer(runCtx, &runtime.ContainerSpec{
		Name:        name,
		Image:       image,
		Env:         []string{fmt.Sprintf("COMMAND=%s", command)},
		Binds:       []string{fmt.Sprintf("%s:/configs", configDir)},
		NetworkHost: true,
		WaitTimeout: timeout,
		Labels:      map[string]string{"author": "caoyingjunz", "pixiuExpand": name},
	})
}

// reconcileNodes 按集群实时节点列表校正扩容节点状态，返回已加入集群的节点数。
func (c *controller) reconcileNodes(ctx context.Context, cs *client.ClusterSet, nodes []model.Node) (int, error) {
	names, ips, err := clusterNodeAddresses(ctx, cs)
	if err != nil {
		for i := range nodes {
			_ = c.factory.Plan().Node().InternalUpdate(ctx, nodes[i].Id, map[string]interface{}{
				"status":         model.NodeStatusFailed,
				"status_message": "无法连接集群确认节点状态，请检查日志",
			})
		}
		return 0, fmt.Errorf("查询集群节点失败: %v", err)
	}

	joined := 0
	for i := range nodes {
		node := &nodes[i]
		if nodeMatched(node.Name, node.Ip, names, ips) {
			joined++
			// 已加入集群的节点从扩容计划解绑（保留 cluster_id 供节点管理页展示）
			_ = c.factory.Plan().Node().InternalUpdate(ctx, node.Id, map[string]interface{}{
				"plan_id":        int64(0),
				"status":         model.NodeStatusAdded,
				"status_message": "",
			})
			continue
		}
		_ = c.factory.Plan().Node().InternalUpdate(ctx, node.Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": "节点未能加入集群，请查看扩容日志",
		})
	}
	return joined, nil
}

// markFirstUnfinishedTaskFailed 在没有失败任务时，把第一个未结束（未开始/运行中）的任务标记为失败，
// 保证早期失败、服务重启中断等场景下进度抽屉有失败行可展示原因。
func (c *controller) markFirstUnfinishedTaskFailed(ctx context.Context, planId int64, message string) {
	tasks, err := c.factory.Plan().Task().List(ctx, planId)
	if err != nil {
		klog.Errorf("list plan(%d) tasks: %v", planId, err)
		return
	}
	for _, task := range tasks {
		if task.Status == model.FailedPlanStatus {
			return
		}
	}
	for _, task := range tasks {
		if task.Status == model.UnStartPlanStatus || task.Status == model.RunningPlanStatus {
			c.updateTask(ctx, planId, task.Name, map[string]interface{}{
				"status": model.FailedPlanStatus, "step": model.FailedPlanStep,
				"message": message, "gmt_modified": time.Now(),
			})
			return
		}
	}
}

// failExpansionPlan 将扩容计划标记为失败：计划状态置失败、补一条失败任务、计划节点置添加失败。
func (c *controller) failExpansionPlan(ctx context.Context, planId int64, cause error) {
	c.updatePlanStatus(ctx, planId, model.FailedPlanStatus)
	c.markFirstUnfinishedTaskFailed(ctx, planId, cause.Error())

	// 按计划关联反查节点，避免早期失败时「添加中」状态残留
	nodes, err := c.factory.Plan().Node().List(ctx, db.WithPlanIdEq(planId))
	if err != nil {
		klog.Errorf("list plan(%d) nodes for failure handling: %v", planId, err)
	}
	for i := range nodes {
		_ = c.factory.Plan().Node().InternalUpdate(ctx, nodes[i].Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": cause.Error(),
		})
	}
	klog.Errorf("expansion plan(%d) failed: %v", planId, cause)
}
