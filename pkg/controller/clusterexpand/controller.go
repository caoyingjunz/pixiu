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
	// List 扩容历史
	List(ctx context.Context, clusterId int64, listOption types.ListOptions) (interface{}, error)
	// Get 扩容详情
	Get(ctx context.Context, clusterId, expansionId int64) (*types.ClusterExpansion, error)
	// Logs 扩容日志（SSE）
	Logs(ctx context.Context, clusterId, expansionId int64, step string, w http.ResponseWriter, r *http.Request) error

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
		createReq := &types.CreateNodeRequest{
			Name:      spec.Name,
			UserId:    user.Id,
			Ip:        spec.Ip,
			Auth:      spec.Auth,
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
				"status": model.NodeStatusAdded, "status_message": "",
			})
			return errors.NewError(fmt.Errorf("节点已加入集群，无法移除"), http.StatusConflict)
		}
	} else if object.Status == model.NodeStatusAdded {
		// 无法连接集群时不能确认节点已不存在，保守拒绝
		return errors.NewError(fmt.Errorf("无法连接集群确认节点状态，暂不允许移除"), http.StatusConflict)
	}

	updates := map[string]interface{}{
		"cluster_id":     int64(0),
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

func (c *controller) Create(ctx context.Context, clusterId int64, req *types.CreateClusterExpansionRequest) (int64, error) {
	obj, user, err := c.authorize(ctx, clusterId, true)
	if err != nil {
		return 0, err
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
			_ = c.factory.Plan().Node().UpdateFields(ctx, n.Id, map[string]interface{}{
				"status": model.NodeStatusAdded, "status_message": "",
			})
			return 0, errors.NewError(fmt.Errorf("节点 %s 已在集群中，请刷新后重试", n.Name), http.StatusConflict)
		}
	}

	options := req.Options
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
			NodeIds:   string(nodeIdsJSON),
			Options:   string(optionsJSON),
			Status:    model.UnStartPlanStatus,
		})
		if createErr != nil {
			return createErr
		}
		expansionId = expansion.Id

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

func (c *controller) Logs(ctx context.Context, clusterId, expansionId int64, step string, w http.ResponseWriter, r *http.Request) error {
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
	if step == stepBootstrap {
		var options types.ExpansionOptions
		if object.Options != "" {
			_ = json.Unmarshal([]byte(object.Options), &options)
		}
		if options.SkipBootstrap {
			return fmt.Errorf("本次扩容跳过了节点初始化，无初始化日志")
		}
	}

	rt, err := runtime.Default()
	if err != nil {
		return err
	}
	readCloser, err := rt.Logs(ctx, containerName(expansionId, step), true)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
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
		NodeIds:     nodeIds,
		Options:     options,
		Status:      o.Status,
		Step:        o.Step,
		Message:     o.Message,
		StartedAt:   o.StartedAt,
		CompletedAt: o.CompletedAt,
	}, nil
}
