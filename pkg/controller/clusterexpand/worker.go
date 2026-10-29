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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/planrender"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

const (
	stepBootstrap = "bootstrap"
	stepAddNode   = "add-node"

	defaultExpansionTimeout = 30 * time.Minute
)

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

func containerName(expansionId int64, step string) string {
	return fmt.Sprintf("pixiu-expand-%d-%s", expansionId, step)
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

// syncStatus 进程启动时的状态恢复：未完成/运行中的扩容标记失败，添加中的节点标记失败。
func (c *controller) syncStatus(ctx context.Context) error {
	now := time.Now()
	for _, status := range []model.TaskStatus{model.UnStartPlanStatus, model.RunningPlanStatus} {
		expansions, err := c.factory.ClusterExpansion().List(ctx, db.WithTaskStatus(status))
		if err != nil {
			return err
		}
		for i := range expansions {
			if err = c.factory.ClusterExpansion().Update(ctx, expansions[i].Id, map[string]interface{}{
				"status":       model.FailedPlanStatus,
				"step":         "完成",
				"message":      "服务异常中断，请重新发起扩容",
				"completed_at": now,
			}); err != nil {
				return err
			}
		}
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithNodeStatus(model.NodeStatusAdding))
	if err != nil {
		return err
	}
	for i := range nodes {
		if err = c.factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": "服务异常中断，请重试",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) syncHandler(ctx context.Context, expansionId int64) {
	expansion, err := c.factory.ClusterExpansion().Get(ctx, expansionId)
	if err != nil {
		klog.Errorf("get cluster expansion(%d): %v", expansionId, err)
		return
	}
	if expansion.Status != model.UnStartPlanStatus && expansion.Status != model.RunningPlanStatus {
		return
	}

	unlock := lockCluster(expansion.ClusterId)
	defer unlock()

	now := time.Now()
	_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
		"status":     model.RunningPlanStatus,
		"step":       "初始化",
		"message":    "",
		"started_at": now,
	})

	clusterObj, err := c.factory.Cluster().Get(ctx, expansion.ClusterId)
	if err != nil || clusterObj == nil {
		c.failExpansion(ctx, expansion.Id, nil, fmt.Errorf("集群不存在或已被删除"))
		return
	}

	nodes, err := c.loadNodes(ctx, expansion)
	if err != nil {
		c.failExpansion(ctx, expansion.Id, nil, err)
		return
	}

	var options types.ExpansionOptions
	if expansion.Options != "" {
		if err = json.Unmarshal([]byte(expansion.Options), &options); err != nil {
			c.failExpansion(ctx, expansion.Id, nodes, fmt.Errorf("解析扩容参数失败: %v", err))
			return
		}
	}

	info := c.detectCluster(ctx, clusterObj)
	if !info.Config.Supported {
		c.failExpansion(ctx, expansion.Id, nodes, fmt.Errorf("%s", info.Config.Reason))
		return
	}

	runner, err := c.resolveRunner(ctx, options.OSImage)
	if err != nil {
		c.failExpansion(ctx, expansion.Id, nodes, err)
		return
	}

	join, err := c.resolveJoin(ctx, clusterObj, info, &options)
	if err != nil {
		c.failExpansion(ctx, expansion.Id, nodes, err)
		return
	}

	dir := filepath.Join(c.cc.Worker.WorkDir, "expansions", strconv.FormatInt(expansion.Id, 10))
	defer func() {
		// 渲染目录内含 join token / SSH 私钥，任务结束即清理
		_ = os.RemoveAll(dir)
	}()

	planNodes, err := nodesToPlanNodes(nodes)
	if err != nil {
		c.failExpansion(ctx, expansion.Id, nodes, err)
		return
	}
	globals := pixiutpl.ExpandGlobals{
		KubeRelease:      join.KubeRelease,
		APIServer:        join.APIServer,
		Token:            join.Token,
		CAHash:           join.CAHash,
		ImageRepository:  join.ImageRepository,
		NetworkInterface: options.NetworkInterface,
		SetHostname:      options.SetHostname,
		RuntimeDataDir:   options.RuntimeDataDir,
		EnableCustomRepo: options.CustomRepo != nil && options.CustomRepo.Enable,
	}
	if err = planrender.RenderExpand(dir, planNodes, globals, options.CustomRepo); err != nil {
		c.failExpansion(ctx, expansion.Id, nodes, fmt.Errorf("渲染扩容配置失败: %v", err))
		return
	}

	timeout := time.Duration(c.cc.Worker.DeployTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultExpansionTimeout
	}

	var runErr error
	// bootstrap-servers 为必经步骤（kubez-ansible 同名 action，初始化节点依赖）
	_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{"step": "初始化"})
	runErr = runRunnerContainer(ctx, containerName(expansion.Id, stepBootstrap), "bootstrap-servers", runner, dir, timeout)

	joined := 0
	if runErr == nil {
		_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{"step": "加入集群"})
		runErr = runRunnerContainer(ctx, containerName(expansion.Id, stepAddNode), "add-node", runner, dir, timeout)
	}

	// 无论成败都按集群实时节点列表校正，保证已 join 的节点不误报失败
	joined, reconcileErr := c.reconcileNodes(ctx, info.Client, nodes)
	if runErr == nil {
		runErr = reconcileErr
	}

	completed := time.Now()
	if runErr != nil {
		_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
			"status":       model.FailedPlanStatus,
			"step":         "完成",
			"message":      runErr.Error(),
			"completed_at": completed,
		})
		klog.Errorf("cluster(%d) expansion(%d) failed: %v", expansion.ClusterId, expansion.Id, runErr)
		return
	}
	if joined != len(nodes) {
		message := fmt.Sprintf("扩容未完全成功：%d/%d 个节点已加入集群", joined, len(nodes))
		_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
			"status":       model.FailedPlanStatus,
			"step":         "完成",
			"message":      message,
			"completed_at": completed,
		})
		return
	}

	_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
		"status":       model.SuccessPlanStatus,
		"step":         "完成",
		"message":      "扩容成功",
		"completed_at": completed,
	})
	klog.Infof("cluster(%d) expansion(%d) succeeded", expansion.ClusterId, expansion.Id)
}

func (c *controller) loadNodes(ctx context.Context, expansion *model.ClusterExpansion) ([]model.Node, error) {
	var ids []int64
	if expansion.NodeIds != "" {
		if err := json.Unmarshal([]byte(expansion.NodeIds), &ids); err != nil {
			return nil, fmt.Errorf("解析扩容节点失败: %v", err)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("扩容节点为空")
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithIdIn(ids))
	if err != nil {
		return nil, err
	}
	if len(nodes) != len(ids) {
		return nil, fmt.Errorf("部分扩容节点已不存在")
	}
	for i := range nodes {
		if nodes[i].ClusterId != expansion.ClusterId {
			return nil, fmt.Errorf("节点 %s 不属于该集群", nodes[i].Name)
		}
	}
	return nodes, nil
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
			_ = c.factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
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
			_ = c.factory.Plan().Node().UpdateFields(ctx, node.Id, map[string]interface{}{
				"status":         model.NodeStatusAdded,
				"status_message": "",
			})
			continue
		}
		_ = c.factory.Plan().Node().UpdateFields(ctx, node.Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": "节点未能加入集群，请查看扩容日志",
		})
	}
	return joined, nil
}

func (c *controller) failExpansion(ctx context.Context, expansionId int64, nodes []model.Node, cause error) {
	now := time.Now()
	_ = c.factory.ClusterExpansion().Update(ctx, expansionId, map[string]interface{}{
		"status":       model.FailedPlanStatus,
		"step":         "完成",
		"message":      cause.Error(),
		"completed_at": now,
	})
	for i := range nodes {
		_ = c.factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": cause.Error(),
		})
	}
	klog.Errorf("cluster expansion(%d) failed: %v", expansionId, cause)
}
