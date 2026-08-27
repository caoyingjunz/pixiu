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

package node

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

type NodeGetter interface {
	Node() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateNodeRequest) error
	Update(ctx context.Context, nodeId int64, req *types.UpdateNodeRequest) error
	Delete(ctx context.Context, nodeId int64) error
	Get(ctx context.Context, nodeId int64) (*types.NodeResult, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
	CheckConnectivity(ctx context.Context, req *types.NodeConnectivityRequest) (*types.NodeConnectivityResult, error)
}

type nodeController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewNode(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &nodeController{
		cc:      cfg,
		factory: f,
	}
}

func (n *nodeController) Create(ctx context.Context, req *types.CreateNodeRequest) error {
	if err := n.preCreate(ctx, req); err != nil {
		klog.Errorf("pre-create check failed for node %s: %v", req.Name, err)
		return err
	}

	authStr, err := req.Auth.Marshal()
	if err != nil {
		klog.Errorf("marshal node auth: %v", err)
		return errors.ErrInvalidRequest
	}
	object := &model.Node{
		Name:   req.Name,
		UserId: req.UserId,
		Ip:     req.Ip,
		Auth:   authStr,
	}
	if _, err = n.factory.Plan().CreateNode(ctx, object); err != nil {
		klog.Errorf("failed to create node %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

// 创建前置检查：ip 全局唯一，不允许与已存在节点冲突
func (n *nodeController) preCreate(ctx context.Context, req *types.CreateNodeRequest) error {
	_, err := n.factory.Plan().GetNodeByIP(ctx, req.Ip)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil
		}
		klog.Errorf("get node by ip %s: %v", req.Ip, err)
		return errors.ErrServerInternal
	}
	return errors.ErrNodeIPExists
}

// 更新前置检查：资源存在 + 非超级管理员只能更新自己的节点
func (n *nodeController) preUpdate(ctx context.Context, nodeId int64) error {
	object, err := n.factory.Plan().GetNode(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("get node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	if err = util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, nodeId); err != nil {
		return err
	}
	return nil
}

func (n *nodeController) Update(ctx context.Context, nodeId int64, req *types.UpdateNodeRequest) error {
	if err := n.preUpdate(ctx, nodeId); err != nil {
		klog.Errorf("pre-update check failed for node(%d): %v", nodeId, err)
		return err
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Ip != nil {
		updates["ip"] = *req.Ip
	}
	if req.Auth != nil {
		authStr, e := req.Auth.Marshal()
		if e != nil {
			klog.Errorf("marshal auth: %v", e)
			return errors.ErrInvalidRequest
		}
		updates["auth"] = authStr
	}
	if len(updates) == 0 {
		return nil
	}

	if err := n.factory.Plan().UpdateNode(ctx, nodeId, req.ResourceVersion, updates); err != nil {
		if err == utilerrors.ErrRecordNotFound {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("update node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (n *nodeController) Delete(ctx context.Context, nodeId int64) error {
	// 非超级管理员只能删除自己的节点
	object, err := n.factory.Plan().GetNode(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("get node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	if err := util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, nodeId); err != nil {
		return err
	}

	if _, err := n.factory.Plan().DeleteNode(ctx, nodeId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrNodeNotFound
		}
		klog.Errorf("delete node %d: %v", nodeId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (n *nodeController) Get(ctx context.Context, nodeId int64) (*types.NodeResult, error) {
	object, err := n.factory.Plan().GetNode(ctx, nodeId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, errors.ErrNodeNotFound
		}
		klog.Errorf("get node %d: %v", nodeId, err)
		return nil, errors.ErrServerInternal
	}

	// 非超级管理员只能查看自己的节点或被 scope 授权的节点
	if err = util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, nodeId); err != nil {
		return nil, err
	}

	return model2Node(object), nil
}

// CheckConnectivity 节点 SSH 连通性检测：node_id>0 走库内认证（需 owner 校验），否则用用户传入的 host+凭据直接检测
func (n *nodeController) CheckConnectivity(ctx context.Context, req *types.NodeConnectivityRequest) (*types.NodeConnectivityResult, error) {
	var (
		webssh = &types.WebSSHRequest{}
		err    error
	)
	if req.NodeId > 0 {
		// 模式A：从库取认证
		object, e := n.factory.Plan().GetNode(ctx, req.NodeId)
		if e != nil {
			if utilerrors.IsRecordNotFound(e) {
				return nil, errors.ErrNodeNotFound
			}
			klog.Errorf("get node %d: %v", req.NodeId, e)
			return nil, errors.ErrServerInternal
		}
		if e := util.CheckResourceAccess(ctx, n.factory, object.UserId, types.ResourceTypeNode, req.NodeId); e != nil {
			return nil, e
		}
		var auth types.PlanNodeAuth
		if e := auth.Unmarshal(object.Auth); e != nil {
			return nil, errors.ErrInvalidRequest
		}
		if webssh, err = sshutil.ResolveAuth(&auth); err != nil {
			return nil, errors.ErrInvalidRequest
		}
		webssh.Host = object.Ip
	} else {
		// 模式B：用户直接传凭据（不落库）
		if req.Host == "" {
			return nil, errors.ErrInvalidRequest
		}
		webssh.Host = req.Host
		webssh.Port = req.Port
		webssh.User = req.User
		if webssh.User == "" {
			webssh.User = "root"
		}
		webssh.Password = req.Password
		webssh.PrivateKey = req.PrivateKey
	}

	result := &types.NodeConnectivityResult{Host: webssh.Host, Port: webssh.Port, User: webssh.User}
	if webssh.Port == 0 {
		webssh.Port = 22
		result.Port = 22
	}
	client, e := sshutil.NewSSHClient(webssh)
	if e != nil {
		// 连通性失败返回 200 + connected=false + message，不将业务失败当 HTTP 错误
		result.Message = e.Error()
		return result, nil
	}
	defer client.Close()
	result.Connected = true
	return result, nil
}

func (n *nodeController) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	// 资源级授权：非超管用户叠加 scope 授权的 node id（超管走 listOption.UserId 现状即可）
	authorizedNodeIDs, err := util.AuthorizedResourceIDs(ctx, n.factory, types.ResourceTypeNode)
	if err != nil {
		return nil, err
	}

	opts := []db.Options{
		db.WithUserOrResourceIDs(listOption.UserId, authorizedNodeIDs),
		db.WithNameLike(listOption.NameSelector),
	}
	if listOption.PlanId != nil {
		opts = append(opts, db.WithPlanIdEq(*listOption.PlanId))
	}

	pageResult.Total, err = n.factory.Plan().CountNodes(ctx, opts...)
	if err != nil {
		klog.Errorf("count nodes: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := n.factory.Plan().ListAllNodes(ctx, opts...)
	if err != nil {
		klog.Errorf("list nodes: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	items := make([]types.NodeResult, 0)
	for i := range objects {
		items = append(items, *model2Node(&objects[i]))
	}

	pageResult.Items = items
	return pageResult, nil
}

func model2Node(o *model.Node) *types.NodeResult {
	auth := types.NodeAuthResult{}
	if o.Auth != "" {
		var spec types.PlanNodeAuth
		if err := spec.Unmarshal(o.Auth); err == nil {
			auth = types.NodeAuthResult{Type: spec.Type, Port: spec.Port}
		}
	}
	return &types.NodeResult{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:   o.Name,
		UserId: o.UserId,
		Ip:     o.Ip,
		Auth:   auth,
	}
}
