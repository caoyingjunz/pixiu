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
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// 创建前预检查
// 1. 创建 node 时 plan 必须存在
func (p *plan) preCreateNode(ctx context.Context, pid int64, req *types.CreatePlanNodeRequest) error {
	_, err := p.Get(ctx, pid)
	if err != nil {
		return err
	}

	return nil
}

func (p *plan) CreateNode(ctx context.Context, pid int64, req *types.CreatePlanNodeRequest) error {
	if err := p.preCreateNode(ctx, pid, req); err != nil {
		return err
	}

	if err := p.createNode(ctx, pid, req); err != nil {
		return err
	}
	return nil
}

func (p *plan) CreateNodes(ctx context.Context, planId int64, nodes []types.CreatePlanNodeRequest) error {
	_, err := p.Get(ctx, planId)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if err = p.createNode(ctx, planId, &node); err != nil {
			return err
		}
	}
	return nil
}
func (p *plan) UpdateNode(ctx context.Context, pid int64, nodeId int64, req *types.UpdatePlanNodeRequest) error {
	return nil
}

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
		node, err := p.buildNodeFromRequest(planId, &newNode)
		if err != nil {
			return err
		}
		if err = p.CreateOrUpdateNode(ctx, node); err != nil {
			return err
		}
	}

	return nil
}

func (p *plan) buildNodeFromRequest(planId int64, req *types.CreatePlanNodeRequest) (*model.Node, error) {
	auth, err := req.Auth.Marshal()
	if err != nil {
		return nil, err
	}

	return &model.Node{
		Name:   req.Name,
		PlanId: planId,
		Role:   strings.Join(req.Role, ","),
		CRI:    req.CRI,
		Ip:     req.Ip,
		Auth:   auth,
	}, nil
}

func (p *plan) createNode(ctx context.Context, planId int64, req *types.CreatePlanNodeRequest) error {
	node, err := p.buildNodeFromRequest(planId, req)
	if err != nil {
		klog.Errorf("failed to build plan(%d) node from request: %v", planId, err)
		return err
	}
	if _, err = p.factory.Plan().CreateNode(ctx, node); err != nil {
		klog.Errorf("failed to create node(%s): %v", req.Name, err)
		return err
	}

	return nil
}

func (p *plan) DeleteNode(ctx context.Context, pid int64, nodeId int64) error {
	if _, err := p.factory.Plan().DeleteNode(ctx, nodeId); err != nil {
		klog.Errorf("failed to delete plan(%d) node(%d): %v", pid, nodeId, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (p *plan) GetNode(ctx context.Context, pid int64, nodeId int64) (*types.PlanNode, error) {
	object, err := p.factory.Plan().GetNode(ctx, nodeId)
	if err != nil {
		klog.Errorf("failed to get plan(%d) node(%d): %v", pid, nodeId, err)
		return nil, errors.ErrServerInternal
	}

	return p.modelNode2Type(object)
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
