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

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 创建前预检查
// 1. plan 必须存在
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
	return nil
}

func (p *plan) GetNode(ctx context.Context, pid int64, nodeId int64) (*types.PlanNode, error) {
	return nil, nil
}

func (p *plan) ListNodes(ctx context.Context, pid int64) ([]types.PlanNode, error) {
	return nil, nil
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
	}
}
