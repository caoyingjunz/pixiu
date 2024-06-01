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

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
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

	// 获取节点认证信息
	auth, err := req.Auth.Marshal()
	if err != nil {
		klog.Errorf("failed to parse node(%s) auth: %v", req.Name, err)
		return err
	}
	if _, err = p.factory.Plan().CreatNode(ctx, &model.Node{
		Name:   req.Name,
		PlanId: pid,
		Role:   req.Role,
		Ip:     req.Ip,
		Auth:   auth,
	}); err != nil {
		klog.Errorf("failed to create node(%s): %v", req.Name, err)
		return err
	}

	return nil
}

func (p *plan) UpdateNode(ctx context.Context, pid int64, nodeId int64, req *types.UpdatePlanNodeRequest) error {
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

	return p.modelNode2Type(object), nil
}

func (p *plan) ListNodes(ctx context.Context, pid int64) ([]types.PlanNode, error) {
	objects, err := p.factory.Plan().ListNodes(ctx, pid)
	if err != nil {
		klog.Errorf("failed to get plan(%d) nodes: %v", pid, err)
		return nil, errors.ErrServerInternal
	}

	var nodes []types.PlanNode
	for _, object := range objects {
		nodes = append(nodes, *p.modelNode2Type(&object))
	}
	return nodes, nil
}

func (p *plan) modelNode2Type(o *model.Node) *types.PlanNode {
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
		Role:   o.Role,
		Ip:     o.Ip,
		Auth: types.PlanNodeAuth{
			Type: types.NoneAuth,
		},
	}
}
