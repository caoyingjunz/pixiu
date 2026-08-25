/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// 删除多余的节点
// 新增没有的节点
// 更新已存在的节点
func (p *plan) updateNodesIfNeeded(ctx context.Context, planId int64, req *types.UpdatePlanRequest) error {
	oldNodes, err := p.factory.Plan().ListNodes(ctx, planId)
	if err != nil {
		return err
	}
	newNodes := req.Nodes

	newMap := make(map[string]types.CreatePlanNodeRequest)
	for _, newNode := range newNodes {
		newMap[newNode.Name] = newNode
	}

	// 遍历寻找待删除节点然后执行删除
	var delNodes []string
	for _, oldNode := range oldNodes {
		name := oldNode.Name
		_, found := newMap[name]
		if !found {
			delNodes = append(delNodes, name)
		}
	}
	if len(delNodes) != 0 {
		if err = p.factory.Plan().DeleteNodesByNames(ctx, planId, delNodes); err != nil {
			klog.Errorf("failed deleting nodes %v %v", delNodes, err)
			return err
		}
	}

	for _, newNode := range newNodes {
		node, err := p.buildNodeFromRequest(ctx, planId, &newNode)
		if err != nil {
			return err
		}
		if err = p.CreateOrUpdateNode(ctx, node); err != nil {
			return err
		}
	}

	return nil
}

func (p *plan) buildNodeFromRequest(ctx context.Context, planId int64, req *types.CreatePlanNodeRequest) (*model.Node, error) {
	auth, err := req.Auth.Marshal()
	if err != nil {
		return nil, err
	}

	// 引用已有主机：从主机管理节点取库内存储的认证凭据，避免客户端传递明文凭据
	if req.NodeID > 0 {
		srcNode, e := p.factory.Plan().GetNode(ctx, req.NodeID)
		if e != nil {
			if utilerrors.IsRecordNotFound(e) {
				return nil, errors.ErrNodeNotFound
			}
			klog.Errorf("get referenced node %d: %v", req.NodeID, e)
			return nil, e
		}
		if e = util.CheckResourceAccess(ctx, p.factory, srcNode.UserId, types.ResourceTypeNode, req.NodeID); e != nil {
			return nil, e
		}
		if srcNode.Auth == "" {
			return nil, fmt.Errorf("引用的主机 %s 未配置认证凭据", srcNode.Name)
		}
		auth = srcNode.Auth
	}

	return &model.Node{
		Name:   req.Name,
		UserId: req.UserId,
		PlanId: planId,
		Role:   strings.Join(req.Role, ","),
		CRI:    req.CRI,
		Ip:     req.Ip,
		Auth:   auth,
	}, nil
}

func (p *plan) ListNodes(ctx context.Context, pid int64) ([]types.PlanNode, error) {
	objects, err := p.factory.Plan().ListNodes(ctx, pid)
	if err != nil {
		klog.Errorf("failed to get plan(%d) nodes: %v", pid, err)
		return nil, errors.ErrServerInternal
	}

	var nodes []types.PlanNode
	for _, object := range objects {
		n, err := p.modelNode2Type(&object)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	return nodes, nil
}

// CreateOrUpdateNode
// TODO: 优化
func (p *plan) CreateOrUpdateNode(ctx context.Context, object *model.Node) error {
	old, err := p.factory.Plan().GetNodeByName(ctx, object.PlanId, object.Name)
	if err != nil {
		if !utilerrors.IsRecordNotFound(err) {
			return err
		}
		// 不存在则创建
		klog.Infof("plan(%d) node(%s) not exist, try to create it.", object.PlanId, object.Name)
		_, err = p.factory.Plan().CreateNode(ctx, object)
		if err != nil {
			return err
		}
		return nil
	}

	klog.Infof("plan(%d) node(%s) already exist", object.PlanId, object.Name)
	// 已存在尝试更新
	updates := p.buildNodeUpdates(old, object)
	if len(updates) == 0 {
		return nil
	}
	klog.Infof("plan(%d) node(%s) already exist and need to update %v", object.PlanId, object.Name, updates)
	return p.factory.Plan().UpdateNode(ctx, old.Id, old.ResourceVersion, updates)
}

func (p *plan) modelNode2Type(o *model.Node) (*types.PlanNode, error) {
	auth := types.PlanNodeAuth{}
	if err := auth.Unmarshal(o.Auth); err != nil {
		return nil, err
	}

	return &types.PlanNode{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		PlanId: o.PlanId,
		Name:   o.Name,
		UserId: o.UserId,
		Role:   strings.Split(o.Role, ","),
		Ip:     o.Ip,
		Auth:   auth,
	}, nil
}

func (p *plan) buildNodeUpdates(old, object *model.Node) map[string]interface{} {
	updates := make(map[string]interface{})
	if old.Ip != object.Ip {
		updates["ip"] = object.Ip
	}
	if old.Role != object.Role {
		updates["role"] = object.Role
	}
	if old.Auth != object.Auth {
		updates["auth"] = object.Auth
	}

	return updates
}
