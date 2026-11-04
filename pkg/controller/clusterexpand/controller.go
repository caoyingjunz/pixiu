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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/node"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

type Getter interface {
	ClusterExpand() Interface
}

// Interface 集群节点扩容能力。
type Interface interface {
	// CreateOrUpdateExpansionPlan 创建或更新集群扩容计划（plans.type=1）的配置与元数据，
	CreateOrUpdateExpansionPlan(ctx context.Context, clusterId int64, req *types.CreateOrUpdateClusterExpansionPlanRequest) (*types.ClusterExpansionPlanResponse, error)
	// GetExpansionPlan 获取集群扩容计划（plans.type=1）及配置；计划不存在时 Exists=false。
	GetExpansionPlan(ctx context.Context, clusterId int64) (*types.ClusterExpansionPlanResponse, error)
	// DeleteExpansionPlan 删除集群扩容计划及其关联的扩容记录/任务/配置，并解除节点与计划的关联。
	DeleteExpansionPlan(ctx context.Context, clusterId int64) error

	// Config 返回集群扩容能力探测结果与默认参数
	Config(ctx context.Context, clusterId int64) (*types.ClusterExpansionConfig, error)
	// ListNodes 返回绑定到该集群的扩容节点（摘要，不含凭据）
	ListNodes(ctx context.Context, clusterId int64) ([]types.ClusterNodeSummary, error)
	// AddNodes 添加节点到集群（只绑定/创建，不执行）
	AddNodes(ctx context.Context, clusterId int64, req *types.CreateClusterNodesRequest) (*types.BatchOperationResult, error)
	// RemoveNode 移除未加入集群的节点（解绑回主机池）
	RemoveNode(ctx context.Context, clusterId, nodeId int64) error
	// Create 创建并启动一次扩容，返回扩容记录 ID
	Create(ctx context.Context, clusterId int64, req *types.CreateClusterExpansionRequest) (int64, error)
	// GenerateJoinParams 通过 Kubernetes API 自动生成 join 参数（bootstrap token + CA hash + apiserver）。
	GenerateJoinParams(ctx context.Context, clusterId int64) (*types.ExpansionJoinParams, error)

	// List 扩容历史
	List(ctx context.Context, clusterId int64, listOption types.ListOptions) (interface{}, error)
	// Get 扩容详情
	Get(ctx context.Context, clusterId, expansionId int64) (*types.ClusterExpansion, error)
	// Logs 扩容日志（text/plain 流；follow=true 时持续跟随）
	Logs(ctx context.Context, clusterId, expansionId int64, step string, follow bool, w http.ResponseWriter, r *http.Request) error

	// Run 启动扩容 worker
	Run(ctx context.Context, workers int) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

// authorize 校验集群访问权；write=true 时仅允许 owner / 管理员 / 超管执行变更。
func (c *controller) authorize(ctx context.Context, clusterId int64, write bool) (*model.Cluster, *model.User, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, nil, errors.ErrUnauthorized
	}

	obj, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil, errors.ErrClusterNotFound
		}
		klog.Errorf("get cluster(%d) for expansion: %v", clusterId, err)
		return nil, nil, errors.ErrServerInternal
	}
	if obj == nil {
		return nil, nil, errors.ErrClusterNotFound
	}

	if err = util.CheckResourceAccess(ctx, c.factory, obj.UserId, types.ResourceTypeCluster, clusterId); err != nil {
		return nil, nil, err
	}
	if write && user.Role != model.RoleRoot && user.Role != model.RoleAdmin && user.Id != obj.UserId {
		return nil, nil, errors.ErrForbidden
	}
	return obj, user, nil
}

func (c *controller) Config(ctx context.Context, clusterId int64) (*types.ClusterExpansionConfig, error) {
	obj, _, err := c.authorize(ctx, clusterId, false)
	if err != nil {
		return nil, err
	}
	return c.detectCluster(ctx, obj).Config, nil
}

func (c *controller) ListNodes(ctx context.Context, clusterId int64) ([]types.ClusterNodeSummary, error) {
	if _, _, err := c.authorize(ctx, clusterId, false); err != nil {
		return nil, err
	}

	objects, err := c.factory.Plan().Node().List(ctx, db.WithClusterIdEq(clusterId))
	if err != nil {
		klog.Errorf("list expansion nodes of cluster(%d): %v", clusterId, err)
		return nil, errors.ErrServerInternal
	}

	items := make([]types.ClusterNodeSummary, 0, len(objects))
	for i := range objects {
		items = append(items, modelNode2Summary(&objects[i]))
	}
	return items, nil
}

func (c *controller) AddNodes(ctx context.Context, clusterId int64, req *types.CreateClusterNodesRequest) (*types.BatchOperationResult, error) {
	obj, user, err := c.authorize(ctx, clusterId, true)
	if err != nil {
		return nil, err
	}
	info := c.detectCluster(ctx, obj)
	if !info.Config.Supported {
		return nil, errors.NewError(fmt.Errorf("%s", info.Config.Reason), http.StatusBadRequest)
	}
	if len(req.NodeIds) == 0 && len(req.Nodes) == 0 {
		return nil, errors.ErrInvalidRequest
	}

	result := &types.BatchOperationResult{
		Succeeded: make([]types.BatchOperationItem, 0),
		Skipped:   make([]types.BatchOperationItem, 0),
		Failed:    make([]types.BatchOperationFailure, 0),
	}

	// 拉取集群实时节点列表：名称或 IP 已存在的节点不允许添加
	names, ips, listErr := clusterNodeAddresses(ctx, info.Client)
	if listErr != nil {
		return nil, errors.NewError(fmt.Errorf("查询集群节点失败: %v", listErr), http.StatusBadGateway)
	}

	// 1) 关联主机库已有主机
	for _, nodeId := range req.NodeIds {
		object, getErr := c.factory.Plan().Node().Get(ctx, nodeId)
		if getErr != nil {
			result.Failed = append(result.Failed, types.BatchOperationFailure{Key: nodeId, Reason: "主机不存在"})
			continue
		}
		if accessErr := util.CheckResourceAccess(ctx, c.factory, object.UserId, types.ResourceTypeNode, nodeId); accessErr != nil {
			result.Failed = append(result.Failed, types.BatchOperationFailure{Key: nodeId, Name: object.Name, Reason: "无权操作该主机"})
			continue
		}
		// 扩容仅支持 worker 节点：非 node 角色的主机（如 master/storage）不允许加入
		if object.Role != "" && object.Role != model.NodeRole {
			result.Failed = append(result.Failed, types.BatchOperationFailure{Key: nodeId, Name: object.Name, Reason: "仅支持添加 node 角色的节点"})
			continue
		}
		if nodeMatched(object.Name, object.Ip, names, ips) {
			result.Failed = append(result.Failed, types.BatchOperationFailure{Key: nodeId, Name: object.Name, Reason: "名称或 IP 已存在于集群中"})
			continue
		}
		switch {
		case object.PlanId != 0:
			result.Skipped = append(result.Skipped, types.BatchOperationItem{Key: nodeId, Name: object.Name})
			continue
		case object.ClusterId == clusterId:
			result.Skipped = append(result.Skipped, types.BatchOperationItem{Key: nodeId, Name: object.Name})
			continue
		case object.ClusterId != 0:
			result.Failed = append(result.Failed, types.BatchOperationFailure{Key: nodeId, Name: object.Name, Reason: "已绑定其他集群"})
			continue
		}
		updates := map[string]interface{}{
			"cluster_id":     clusterId,
			"plan_id":        int64(0),
			"role":           model.NodeRole,
			"cri":            model.ContainerdCRI,
			"status":         model.NodeStatusUnadded,
			"status_message": "",
		}
		if updateErr := c.factory.Plan().Node().UpdateFields(ctx, nodeId, updates); updateErr != nil {
			klog.Errorf("bind node(%d) to cluster(%d): %v", nodeId, clusterId, updateErr)
			result.Failed = append(result.Failed, types.BatchOperationFailure{Key: nodeId, Name: object.Name, Reason: "绑定集群失败"})
			continue
		}
		result.Succeeded = append(result.Succeeded, types.BatchOperationItem{Key: nodeId, Name: object.Name})
	}

	// 2) 手动新增主机并直接绑定集群
	nodeCtrl := node.NewNode(c.cc, c.factory)
	for i := range req.Nodes {
		spec := req.Nodes[i]
		if nodeMatched(spec.Name, spec.Ip, names, ips) {
			result.Failed = append(result.Failed, types.BatchOperationFailure{Name: spec.Name, Reason: "名称或 IP 已存在于集群中"})
			continue
		}
		// 幂等：同一 IP 已存在于主机库时复用，避免重试时重复创建主机
		existing, getErr := c.factory.Plan().Node().GetByIP(ctx, spec.Ip)
		switch {
		case getErr == nil && existing != nil:
			if existing.ClusterId == clusterId {
				result.Skipped = append(result.Skipped, types.BatchOperationItem{Key: existing.Id, Name: existing.Name})
				continue
			}
			if existing.PlanId != 0 || existing.ClusterId != 0 {
				result.Failed = append(result.Failed, types.BatchOperationFailure{Key: existing.Id, Name: existing.Name, Reason: "IP 已被其他集群或计划占用"})
				continue
			}
			updates := map[string]interface{}{
				"cluster_id":     clusterId,
				"plan_id":        int64(0),
				"role":           model.NodeRole,
				"cri":            model.ContainerdCRI,
				"status":         model.NodeStatusUnadded,
				"status_message": "",
			}
			if updateErr := c.factory.Plan().Node().UpdateFields(ctx, existing.Id, updates); updateErr != nil {
				klog.Errorf("bind existing node(%d) to cluster(%d): %v", existing.Id, clusterId, updateErr)
				result.Failed = append(result.Failed, types.BatchOperationFailure{Key: existing.Id, Name: existing.Name, Reason: "绑定集群失败"})
				continue
			}
			result.Succeeded = append(result.Succeeded, types.BatchOperationItem{Key: existing.Id, Name: existing.Name})
			continue
		case getErr != nil && !utilerrors.IsRecordNotFound(getErr):
			result.Failed = append(result.Failed, types.BatchOperationFailure{Name: spec.Name, Reason: "查询主机失败"})
			continue
		}
		createReq := &types.CreateNodeRequest{
			Name:      spec.Name,
			UserId:    user.Id,
			Ip:        spec.Ip,
			Auth:      spec.Auth,
			PlanId:    0,
			ClusterId: clusterId,
		}
		if createErr := nodeCtrl.Create(ctx, createReq); createErr != nil {
			result.Failed = append(result.Failed, types.BatchOperationFailure{Name: spec.Name, Reason: createErr.Error()})
			continue
		}
		object, getErr := c.factory.Plan().Node().GetByIP(ctx, spec.Ip)
		if getErr != nil || object == nil {
			result.Succeeded = append(result.Succeeded, types.BatchOperationItem{Name: spec.Name})
			continue
		}
		result.Succeeded = append(result.Succeeded, types.BatchOperationItem{Key: object.Id, Name: object.Name})
	}

	return result, nil
}

func (c *controller) RemoveNode(ctx context.Context, clusterId, nodeId int64) error {
	obj, _, err := c.authorize(ctx, clusterId, true)
	if err != nil {
		return err
	}

	object, err := c.factory.Plan().Node().Get(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		return errors.ErrServerInternal
	}
	if object.ClusterId != clusterId {
		return errors.ErrNodeNotFound
	}
	if object.Status == model.NodeStatusAdding {
		return errors.NewError(fmt.Errorf("节点正在添加中，请等待扩容任务结束"), http.StatusConflict)
	}

	// 状态可能陈旧：能连上集群时以实时节点列表为准
	info := c.detectCluster(ctx, obj)
	if info.Config.Supported && info.Client != nil {
		names, ips, listErr := clusterNodeAddresses(ctx, info.Client)
		if listErr == nil && nodeMatched(object.Name, object.Ip, names, ips) {
			_ = c.factory.Plan().Node().UpdateFields(ctx, nodeId, map[string]interface{}{
				"plan_id": int64(0),
				"status":  model.NodeStatusAdded, "status_message": "",
			})
			return errors.NewError(fmt.Errorf("节点已加入集群，无法移除"), http.StatusConflict)
		}
	} else if object.Status == model.NodeStatusAdded {
		// 无法连接集群时不能确认节点已不存在，保守拒绝
		return errors.NewError(fmt.Errorf("无法连接集群确认节点状态，暂不允许移除"), http.StatusConflict)
	}

	updates := map[string]interface{}{
		"cluster_id":     int64(0),
		"plan_id":        int64(0),
		"role":           "",
		"cri":            "",
		"status":         model.NodeStatusUnadded,
		"status_message": "",
	}
	if err = c.factory.Plan().Node().UpdateFields(ctx, nodeId, updates); err != nil {
		klog.Errorf("unbind node(%d) from cluster(%d): %v", nodeId, clusterId, err)
		return errors.ErrServerInternal
	}
	return nil
}

// ensureExpansionPlan 返回集群对应的扩容计划（plans.type=1），不存在则自动创建。
// 扩容计划由系统管理：不创建 configs/nodes/cluster 等子资源，节点通过 plan_id 挂在计划下。
func (c *controller) ensureExpansionPlan(ctx context.Context, factory db.ShareDaoFactory, cluster *model.Cluster) (*model.Plan, error) {
	plans, err := factory.Plan().List(ctx, db.WithPlanTypeEq(1), db.WithPlanClusterIdEq(cluster.Id))
	if err != nil {
		klog.Errorf("list expansion plans of cluster(%d): %v", cluster.Id, err)
		return nil, errors.ErrServerInternal
	}
	if len(plans) > 0 {
		return &plans[0], nil
	}

	created, err := factory.Plan().Create(ctx, &model.Plan{
		Name:        fmt.Sprintf("expand-cluster-%d", cluster.Id),
		UserId:      cluster.UserId,
		Description: fmt.Sprintf("集群 %s 扩容计划（添加节点时自动创建）", cluster.AliasName),
		Status:      model.UnStartPlanStatus,
		Type:        1,
		ClusterId:   cluster.Id,
	})
	if err != nil {
		klog.Errorf("create expansion plan for cluster(%d): %v", cluster.Id, err)
		return nil, errors.ErrServerInternal
	}
	klog.Infof("created expansion plan(%d) for cluster(%d)", created.Id, cluster.Id)
	return created, nil
}

// syncExpansionPlanNodes 将选中节点关联到计划，并解绑该计划下未选中的节点。
// 返回同步后实际关联到该计划的节点 ID，供保存接口直接构造响应。
func (c *controller) syncExpansionPlanNodes(ctx context.Context, factory db.ShareDaoFactory, clusterId, planId int64, nodeIds []int64) ([]int64, error) {
	selected := make(map[int64]struct{}, len(nodeIds))
	for _, id := range nodeIds {
		selected[id] = struct{}{}
	}
	nodes, err := factory.Plan().Node().List(ctx, db.WithClusterIdEq(clusterId))
	if err != nil {
		klog.Errorf("list expansion nodes of cluster(%d): %v", clusterId, err)
		return nil, err
	}
	associated := make([]int64, 0, len(nodes))
	for i := range nodes {
		_, want := selected[nodes[i].Id]
		if want {
			if nodes[i].PlanId != planId {
				if err = factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{"plan_id": planId}); err != nil {
					klog.Errorf("associate node(%d) to plan(%d): %v", nodes[i].Id, planId, err)
					return nil, err
				}
			}
			associated = append(associated, nodes[i].Id)
		} else if nodes[i].PlanId == planId {
			if err = factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{"plan_id": int64(0)}); err != nil {
				klog.Errorf("unbind node(%d) from plan(%d): %v", nodes[i].Id, planId, err)
				return nil, err
			}
		}
	}
	return associated, nil
}

func (c *controller) Create(ctx context.Context, clusterId int64, req *types.CreateClusterExpansionRequest) (int64, error) {
	obj, user, err := c.authorize(ctx, clusterId, true)
	if err != nil {
		return 0, err
	}

	// 执行扩容前先确保集群对应的扩容计划（type=1）存在，扩容记录关联到该计划
	expansionPlan, planErr := c.ensureExpansionPlan(ctx, c.factory, obj)
	if planErr != nil {
		return 0, planErr
	}

	info := c.detectCluster(ctx, obj)
	if !info.Config.Supported {
		return 0, errors.NewError(fmt.Errorf("%s", info.Config.Reason), http.StatusBadRequest)
	}

	unlock := lockCluster(clusterId)
	defer unlock()

	// 同一集群串行：存在未开始/运行中的扩容时拒绝
	active, err := c.factory.ClusterExpansion().Count(ctx, db.WithClusterIdEq(clusterId), db.WithTaskStatus(model.UnStartPlanStatus))
	if err != nil {
		return 0, errors.ErrServerInternal
	}
	if active > 0 {
		return 0, errors.NewError(fmt.Errorf("该集群存在进行中的扩容任务，请稍后重试"), http.StatusConflict)
	}
	running, err := c.factory.ClusterExpansion().Count(ctx, db.WithClusterIdEq(clusterId), db.WithTaskStatus(model.RunningPlanStatus))
	if err != nil {
		return 0, errors.ErrServerInternal
	}
	if running > 0 {
		return 0, errors.NewError(fmt.Errorf("该集群存在进行中的扩容任务，请稍后重试"), http.StatusConflict)
	}

	// 去重节点 ID
	nodeIds := make([]int64, 0, len(req.NodeIds))
	seen := make(map[int64]struct{}, len(req.NodeIds))
	for _, id := range req.NodeIds {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		nodeIds = append(nodeIds, id)
	}
	if len(nodeIds) == 0 {
		return 0, errors.ErrInvalidRequest
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithIdIn(nodeIds))
	if err != nil {
		return 0, errors.ErrServerInternal
	}
	if len(nodes) != len(nodeIds) {
		return 0, errors.NewError(fmt.Errorf("部分节点不存在"), http.StatusBadRequest)
	}
	for i := range nodes {
		n := &nodes[i]
		if n.ClusterId != clusterId {
			return 0, errors.NewError(fmt.Errorf("节点 %s 不属于该集群", n.Name), http.StatusBadRequest)
		}
		if n.Status == model.NodeStatusAdding {
			return 0, errors.NewError(fmt.Errorf("节点 %s 正在添加中", n.Name), http.StatusConflict)
		}
		// status=已加入 可能是 K8s 外部删除后的陈旧标记，交由下方实时节点列表判定
	}

	// 已在集群中的节点直接标记为已加入并拒绝本次执行
	names, ips, err := clusterNodeAddresses(ctx, info.Client)
	if err != nil {
		return 0, errors.NewError(fmt.Errorf("查询集群节点失败: %v", err), http.StatusBadGateway)
	}
	for i := range nodes {
		n := &nodes[i]
		if nodeMatched(n.Name, n.Ip, names, ips) {
			// 已在集群中的节点标记已加入，并解除与扩容计划的关联
			_ = c.factory.Plan().Node().UpdateFields(ctx, n.Id, map[string]interface{}{
				"plan_id": int64(0),
				"status":  model.NodeStatusAdded, "status_message": "",
			})
			return 0, errors.NewError(fmt.Errorf("节点 %s 已在集群中，请刷新后重试", n.Name), http.StatusConflict)
		}
	}

	// 节点校验通过后，按去重后的勾选节点同步计划关联：选中关联、未选中解绑
	if _, planErr = c.syncExpansionPlanNodes(ctx, c.factory, clusterId, expansionPlan.Id, nodeIds); planErr != nil {
		return 0, errors.ErrServerInternal
	}

	options := req.Options
	// 优先级：请求体显式字段 > 已保存的扩容计划配置 > 集群能力探测默认值
	c.applyExpansionPlanDefaults(ctx, expansionPlan.Id, &options)
	if options.ImageRepository == "" {
		options.ImageRepository = info.Config.ImageRepository
	}
	if options.NetworkInterface == "" {
		options.NetworkInterface = info.Config.DefaultNetworkInterface
	}
	if options.NetworkInterface == "" {
		options.NetworkInterface = "eth0"
	}
	if options.OSImage == "" {
		options.OSImage = info.Config.DefaultOSImage
	}
	// runner 镜像无论是否跳过 bootstrap 都必须可用（add-node 容器本身来自 runner 镜像）
	if _, err = c.resolveRunner(ctx, options.OSImage); err != nil {
		return 0, errors.NewError(fmt.Errorf("解析 runner 失败: %v", err), http.StatusBadRequest)
	}

	nodeIdsJSON, err := json.Marshal(nodeIds)
	if err != nil {
		return 0, errors.ErrServerInternal
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return 0, errors.ErrServerInternal
	}

	var expansionId int64
	txErr := c.factory.Transaction(ctx, func(tx db.ShareDaoFactory) error {
		expansion, createErr := tx.ClusterExpansion().Create(ctx, &model.ClusterExpansion{
			ClusterId: clusterId,
			UserId:    user.Id,
			PlanId:    expansionPlan.Id,
			NodeIds:   string(nodeIdsJSON),
			Options:   string(optionsJSON),
			Status:    model.UnStartPlanStatus,
		})
		if createErr != nil {
			return createErr
		}
		expansionId = expansion.Id

		// 每次扩容重置该计划的进度任务，避免历史步骤残留
		if taskErr := tx.Plan().Task().Delete(ctx, expansionPlan.Id); taskErr != nil {
			return taskErr
		}
		for _, t := range expansionTaskSpecs {
			if _, taskErr := tx.Plan().Task().Create(ctx, &model.Task{
				Name:   t.Name,
				PlanId: expansionPlan.Id,
				Step:   model.UnStartedPlanStep,
				Action: t.Action,
				Status: model.UnStartPlanStatus,
			}); taskErr != nil {
				return taskErr
			}
		}

		for i := range nodes {
			if updateErr := tx.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
				"status": model.NodeStatusAdding, "status_message": "",
			}); updateErr != nil {
				return updateErr
			}
		}
		return nil
	})
	if txErr != nil {
		klog.Errorf("create expansion for cluster(%d): %v", clusterId, txErr)
		return 0, errors.ErrServerInternal
	}

	expansionQueue.Add(expansionId)

	return expansionId, nil
}

// checkNoActiveExpansion 存在未开始/运行中的扩容记录时返回冲突。
func (c *controller) checkNoActiveExpansion(ctx context.Context, clusterId int64, message string) error {
	for _, status := range []model.TaskStatus{model.UnStartPlanStatus, model.RunningPlanStatus} {
		count, err := c.factory.ClusterExpansion().Count(ctx, db.WithClusterIdEq(clusterId), db.WithTaskStatus(status))
		if err != nil {
			klog.Errorf("count %s expansion of cluster(%d): %v", status, clusterId, err)
			return errors.ErrServerInternal
		}
		if count > 0 {
			return errors.NewError(fmt.Errorf("%s", message), http.StatusConflict)
		}
	}
	return nil
}

// CreateOrUpdateExpansionPlan 创建或更新集群扩容计划（plans.type=1）的配置与元数据，
// 并在同一事务内完成后直接构造响应，避免调用方二次查询。
// 与 plan 控制器一致：存在则 Update，不存在则 Create；EnablePublicIp=false 时清空 api_server。
func (c *controller) CreateOrUpdateExpansionPlan(ctx context.Context, clusterId int64, req *types.CreateOrUpdateClusterExpansionPlanRequest) (*types.ClusterExpansionPlanResponse, error) {
	obj, _, err := c.authorize(ctx, clusterId, true)
	if err != nil {
		return nil, err
	}
	if err = c.checkNoActiveExpansion(ctx, clusterId, "扩容进行中，无法修改配置"); err != nil {
		return nil, err
	}
	if len(req.NodeIds) == 0 {
		return nil, errors.ErrInvalidRequest
	}

	planConfig, err := buildExpansionPlanConfig(&req.Config)
	if err != nil {
		return nil, err
	}

	var (
		expansionPlan *model.Plan
		associatedIds []int64
	)
	txErr := c.factory.Transaction(ctx, func(tx db.ShareDaoFactory) error {
		var txErr error
		expansionPlan, txErr = c.ensureExpansionPlan(ctx, tx, obj)
		if txErr != nil {
			return txErr
		}
		// 按勾选的节点同步计划关联：选中关联、未选中解绑
		associatedIds, txErr = c.syncExpansionPlanNodes(ctx, tx, clusterId, expansionPlan.Id, req.NodeIds)
		if txErr != nil {
			return errors.ErrServerInternal
		}

		planConfig.PlanId = expansionPlan.Id
		oldConfig, getErr := tx.Plan().Config().GetByPlan(ctx, expansionPlan.Id)
		switch {
		case getErr == nil && oldConfig != nil:
			updates := map[string]interface{}{
				"region":     planConfig.Region,
				"os_image":   planConfig.OSImage,
				"kubernetes": planConfig.Kubernetes,
				"network":    planConfig.Network,
				"runtime":    planConfig.Runtime,
				"component":  planConfig.Component,
			}
			if txErr = tx.Plan().Config().Update(ctx, oldConfig.Id, oldConfig.ResourceVersion, updates); txErr != nil {
				klog.Errorf("update plan(%d) config: %v", expansionPlan.Id, txErr)
				return errors.ErrServerInternal
			}
		case utilerrors.IsRecordNotFound(getErr) || (getErr == nil && oldConfig == nil):
			if txErr = tx.Plan().Config().Create(ctx, nil, planConfig); txErr != nil {
				klog.Errorf("create plan(%d) config: %v", expansionPlan.Id, txErr)
				return errors.ErrServerInternal
			}
		default:
			klog.Errorf("get plan(%d) config: %v", expansionPlan.Id, getErr)
			return errors.ErrServerInternal
		}

		// 同步计划元数据（description/kubernetes_version）
		// 扩容计划名称由系统生成（expand-cluster-<id>），不随前端更新，避免与部署计划名称唯一索引冲突
		if expansionPlan.Description != req.Description ||
			expansionPlan.KubernetesVersion != req.Config.Kubernetes.KubernetesVersion {
			updates := map[string]interface{}{
				"description":        req.Description,
				"kubernetes_version": req.Config.Kubernetes.KubernetesVersion,
			}
			if txErr = tx.Plan().Update(ctx, expansionPlan.Id, expansionPlan.ResourceVersion, updates); txErr != nil {
				klog.Errorf("update plan(%d): %v", expansionPlan.Id, txErr)
				return errors.ErrServerInternal
			}
		}
		expansionPlan.Description = req.Description
		expansionPlan.KubernetesVersion = req.Config.Kubernetes.KubernetesVersion
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	resp, respErr := buildExpansionPlanResponse(expansionPlan, planConfig, associatedIds)
	if respErr != nil {
		klog.Errorf("build plan(%d) response: %v", expansionPlan.Id, respErr)
		return nil, errors.ErrServerInternal
	}
	return resp, nil
}

// buildExpansionPlanResponse 根据计划、配置与关联节点构造对外响应。
// cfg 为 nil 时仅返回计划元数据，不返回配置。
func buildExpansionPlanResponse(plan *model.Plan, cfg *model.Config, nodeIds []int64) (*types.ClusterExpansionPlanResponse, error) {
	if plan == nil {
		return nil, fmt.Errorf("expansion plan is nil")
	}
	resp := &types.ClusterExpansionPlanResponse{
		Exists:      true,
		PlanId:      plan.Id,
		Name:        plan.Name,
		Description: plan.Description,
		NodeIds:     append(make([]int64, 0, len(nodeIds)), nodeIds...),
	}
	if cfg == nil {
		return resp, nil
	}

	planConfig := &types.ExpansionPlanConfigResponse{
		OSImage:     cfg.OSImage,
		Description: plan.Description,
	}
	if err := planConfig.Kubernetes.Unmarshal(cfg.Kubernetes); err != nil {
		return nil, fmt.Errorf("unmarshal kubernetes config: %w", err)
	}
	if err := planConfig.Network.Unmarshal(cfg.Network); err != nil {
		return nil, fmt.Errorf("unmarshal network config: %w", err)
	}
	if err := planConfig.Runtime.Unmarshal(cfg.Runtime); err != nil {
		return nil, fmt.Errorf("unmarshal runtime config: %w", err)
	}
	if err := planConfig.Component.Unmarshal(cfg.Component); err != nil {
		return nil, fmt.Errorf("unmarshal component config: %w", err)
	}
	resp.Config = planConfig
	return resp, nil
}

// GetExpansionPlan 获取集群扩容计划（plans.type=1）及配置。
// 计划不存在时返回 Exists=false；计划存在但无 configs 行时 Config 留空。
func (c *controller) GetExpansionPlan(ctx context.Context, clusterId int64) (*types.ClusterExpansionPlanResponse, error) {
	if _, _, err := c.authorize(ctx, clusterId, false); err != nil {
		return nil, err
	}

	plans, err := c.factory.Plan().List(ctx, db.WithPlanTypeEq(1), db.WithPlanClusterIdEq(clusterId))
	if err != nil {
		klog.Errorf("list expansion plans of cluster(%d): %v", clusterId, err)
		return nil, errors.ErrServerInternal
	}
	if len(plans) == 0 {
		return &types.ClusterExpansionPlanResponse{Exists: false}, nil
	}

	plan := &plans[0]
	nodes, err := c.factory.Plan().Node().List(ctx, db.WithPlanIdEq(plan.Id))
	if err != nil {
		klog.Errorf("list plan(%d) nodes: %v", plan.Id, err)
		return nil, errors.ErrServerInternal
	}
	nodeIds := make([]int64, 0, len(nodes))
	for i := range nodes {
		nodeIds = append(nodeIds, nodes[i].Id)
	}

	cfg, err := c.factory.Plan().Config().GetByPlan(ctx, plan.Id)
	if err != nil {
		if !utilerrors.IsRecordNotFound(err) {
			klog.Errorf("get plan(%d) config: %v", plan.Id, err)
			return nil, errors.ErrServerInternal
		}
		cfg = nil
	}

	resp, respErr := buildExpansionPlanResponse(plan, cfg, nodeIds)
	if respErr != nil {
		klog.Errorf("build plan(%d) response: %v", plan.Id, respErr)
		return nil, errors.ErrServerInternal
	}
	return resp, nil
}

// applyExpansionPlanDefaults 用已保存的扩容计划配置填充请求中未显式提供的执行参数。
// 显式零值无法与“未提供”区分，因此 bool 字段按 false 视为未提供；请求体非零值始终优先。
func (c *controller) applyExpansionPlanDefaults(ctx context.Context, planId int64, options *types.ExpansionOptions) {
	cfg, err := c.factory.Plan().Config().GetByPlan(ctx, planId)
	if err != nil {
		if !utilerrors.IsRecordNotFound(err) {
			klog.Warningf("get expansion plan(%d) config: %v", planId, err)
		}
		return
	}
	if cfg == nil {
		return
	}

	var (
		kubernetes  types.KubernetesSpec
		network     types.NetworkSpec
		runtimeSpec types.RuntimeSpec
		component   types.ComponentSpec
	)
	if err = kubernetes.Unmarshal(cfg.Kubernetes); err != nil {
		klog.Warningf("unmarshal expansion plan(%d) kubernetes config: %v", planId, err)
		return
	}
	if err = network.Unmarshal(cfg.Network); err != nil {
		klog.Warningf("unmarshal expansion plan(%d) network config: %v", planId, err)
		return
	}
	if err = runtimeSpec.Unmarshal(cfg.Runtime); err != nil {
		klog.Warningf("unmarshal expansion plan(%d) runtime config: %v", planId, err)
		return
	}
	if err = component.Unmarshal(cfg.Component); err != nil {
		klog.Warningf("unmarshal expansion plan(%d) component config: %v", planId, err)
		return
	}

	if options.OSImage == "" {
		options.OSImage = cfg.OSImage
	}
	if options.NetworkInterface == "" {
		options.NetworkInterface = network.NetworkInterface
	}
	if options.ImageRepository == "" {
		options.ImageRepository = kubernetes.ImageRepository
	}
	if options.RuntimeDataDir == "" {
		options.RuntimeDataDir = runtimeSpec.DataDir
	}
	if options.CustomRepo == nil {
		options.CustomRepo = component.CustomRepo
	}
	if !options.SetHostname {
		options.SetHostname = kubernetes.SetHostname
	}
	if options.APIServer == "" {
		options.APIServer = kubernetes.KubeAPIServerAddress
		if options.APIServer == "" {
			options.APIServer = kubernetes.ApiServer
		}
	}
	if options.Token == "" {
		options.Token = kubernetes.BootstrapToken
	}
	if options.CAHash == "" {
		options.CAHash = kubernetes.CAHash
	}
}

// buildExpansionPlanConfig 将请求配置 marshal 为 configs 模型，字段映射沿用 plan 控制器的 buildPlanConfig。
// join 参数（bootstrap_token / ca_hash）位于 KubernetesSpec 内，随 kubernetes JSON 一并持久化。
func buildExpansionPlanConfig(req *types.CreatePlanConfigRequest) (*model.Config, error) {
	ks := req.Kubernetes
	if ks.EnablePublicIp {
		if len(ks.ApiServer) == 0 {
			return nil, errors.NewError(fmt.Errorf("启用 ApiServer 地址，但是未配置关联 IP"), http.StatusBadRequest)
		}
	} else {
		ks.ApiServer = ""
	}
	kubeConfig, err := ks.Marshal()
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	networkConfig, err := req.Network.Marshal()
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	runtimeConfig, err := req.Runtime.Marshal()
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	componentConfig, err := req.Component.Marshal()
	if err != nil {
		return nil, errors.ErrServerInternal
	}

	return &model.Config{
		Region:     req.Region,
		OSImage:    req.OSImage,
		Kubernetes: kubeConfig,
		Network:    networkConfig,
		Runtime:    runtimeConfig,
		Component:  componentConfig,
	}, nil
}

// DeleteExpansionPlan 删除集群扩容计划（plans.type=1）。计划不存在时幂等返回成功；
// 存在未开始/运行中的扩容记录时拒绝删除。
func (c *controller) DeleteExpansionPlan(ctx context.Context, clusterId int64) error {
	if _, _, err := c.authorize(ctx, clusterId, true); err != nil {
		return err
	}
	if err := c.checkNoActiveExpansion(ctx, clusterId, "扩容进行中，无法删除"); err != nil {
		return err
	}

	plans, err := c.factory.Plan().List(ctx, db.WithPlanTypeEq(1), db.WithPlanClusterIdEq(clusterId))
	if err != nil {
		klog.Errorf("list expansion plans of cluster(%d): %v", clusterId, err)
		return errors.ErrServerInternal
	}
	if len(plans) == 0 {
		return nil
	}

	txErr := c.factory.Transaction(ctx, func(tx db.ShareDaoFactory) error {
		// a) 删除集群下全部扩容记录
		if delErr := tx.ClusterExpansion().DeleteByClusterId(ctx, clusterId); delErr != nil {
			return delErr
		}
		for i := range plans {
			planId := plans[i].Id
			// c) 删除计划的任务与配置
			if delErr := tx.Plan().Task().Delete(ctx, planId); delErr != nil {
				return delErr
			}
			if delErr := tx.Plan().Config().DeleteByPlan(ctx, planId); delErr != nil {
				return delErr
			}
			// d) 节点仅解除与计划的关联，保留集群绑定与节点属性
			nodes, listErr := tx.Plan().Node().List(ctx, db.WithPlanIdEq(planId))
			if listErr != nil {
				return listErr
			}
			for j := range nodes {
				if updErr := tx.Plan().Node().UpdateFields(ctx, nodes[j].Id, map[string]interface{}{
					"plan_id": int64(0),
				}); updErr != nil {
					return updErr
				}
			}
			// e) 删除计划记录
			if _, delErr := tx.Plan().Delete(ctx, planId); delErr != nil {
				return delErr
			}
		}
		return nil
	})
	if txErr != nil {
		klog.Errorf("delete expansion plan of cluster(%d): %v", clusterId, txErr)
		return errors.ErrServerInternal
	}
	return nil
}

func (c *controller) List(ctx context.Context, clusterId int64, listOption types.ListOptions) (interface{}, error) {
	if _, _, err := c.authorize(ctx, clusterId, false); err != nil {
		return nil, err
	}
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{db.WithClusterIdEq(clusterId)}
	total, err := c.factory.ClusterExpansion().Count(ctx, opts...)
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	pageResult.Total = total

	opts = append(opts,
		db.WithOrderByDesc(),
		db.WithOffset((listOption.Page-1)*listOption.Limit),
		db.WithLimit(listOption.Limit),
	)
	objects, err := c.factory.ClusterExpansion().List(ctx, opts...)
	if err != nil {
		return nil, errors.ErrServerInternal
	}

	items := make([]types.ClusterExpansion, 0, len(objects))
	for i := range objects {
		item, mapErr := modelExpansion2Type(&objects[i])
		if mapErr != nil {
			return nil, errors.ErrServerInternal
		}
		items = append(items, *item)
	}
	pageResult.Items = items
	return pageResult, nil
}

func (c *controller) Get(ctx context.Context, clusterId, expansionId int64) (*types.ClusterExpansion, error) {
	if _, _, err := c.authorize(ctx, clusterId, false); err != nil {
		return nil, err
	}

	object, err := c.factory.ClusterExpansion().Get(ctx, expansionId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, errors.NewError(fmt.Errorf("扩容任务不存在"), http.StatusNotFound)
		}
		return nil, errors.ErrServerInternal
	}
	if object.ClusterId != clusterId {
		return nil, errors.NewError(fmt.Errorf("扩容任务不存在"), http.StatusNotFound)
	}
	return modelExpansion2Type(object)
}

func (c *controller) Logs(ctx context.Context, clusterId, expansionId int64, step string, follow bool, w http.ResponseWriter, r *http.Request) error {
	if _, _, err := c.authorize(ctx, clusterId, false); err != nil {
		return err
	}

	object, err := c.factory.ClusterExpansion().Get(ctx, expansionId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.NewError(fmt.Errorf("扩容任务不存在"), http.StatusNotFound)
		}
		return errors.ErrServerInternal
	}
	if object.ClusterId != clusterId {
		return errors.NewError(fmt.Errorf("扩容任务不存在"), http.StatusNotFound)
	}
	if object.Status == model.UnStartPlanStatus {
		return fmt.Errorf("任务尚未开始")
	}

	step = strings.TrimSpace(step)
	if step == "" {
		step = stepAddNode
	}
	if step != stepBootstrap && step != stepAddNode {
		return fmt.Errorf("不支持的日志步骤: %s", step)
	}

	action := actionBootstrap
	if step == stepAddNode {
		action = actionAddNode
	}

	rt, err := runtime.Default()
	if err != nil {
		return err
	}
	// 日志按扩容记录隔离：runner 名称在 action-planId 后追加 expansionId
	readCloser, err := rt.Logs(ctx, fmt.Sprintf("%s-%d-%d", action, object.PlanId, object.Id), follow)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flush, _ := w.(http.Flusher)

	scanner := bufio.NewScanner(readCloser)
	for scanner.Scan() {
		line := append(scanner.Bytes(), '\n')
		if _, err = w.Write(line); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		flush.Flush()
	}
	if err = scanner.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func modelNode2Summary(o *model.Node) types.ClusterNodeSummary {
	return types.ClusterNodeSummary{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:          o.Name,
		Ip:            o.Ip,
		Role:          o.Role,
		CRI:           o.CRI,
		Status:        o.Status,
		StatusMessage: o.StatusMessage,
	}
}

func modelExpansion2Type(o *model.ClusterExpansion) (*types.ClusterExpansion, error) {
	var nodeIds []int64
	if o.NodeIds != "" {
		if err := json.Unmarshal([]byte(o.NodeIds), &nodeIds); err != nil {
			return nil, err
		}
	}
	var options types.ExpansionOptions
	if o.Options != "" {
		if err := json.Unmarshal([]byte(o.Options), &options); err != nil {
			return nil, err
		}
	}
	// bootstrap token 属于敏感凭据，不在 API 响应中回显
	options.Token = ""
	return &types.ClusterExpansion{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		ClusterId:   o.ClusterId,
		PlanId:      o.PlanId,
		NodeIds:     nodeIds,
		Options:     options,
		Status:      o.Status,
		Step:        o.Step,
		Message:     o.Message,
		StartedAt:   o.StartedAt,
		CompletedAt: o.CompletedAt,
	}, nil
}
