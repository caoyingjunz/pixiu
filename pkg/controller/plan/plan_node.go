/*
Copyright 2021 The Pixiu Authors.

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

package plan

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

func (p *plan) updateNodesIfNeeded(ctx context.Context, planId int64, req *types.UpdatePlanRequest) error {
	return p.syncPlanNodes(ctx, planId, req.Nodes)
}

func (p *plan) syncPlanNodesInTx(ctx context.Context, planId int64, nodes []types.CreatePlanNodeRequest, tx *gorm.DB) error {
	for i := range nodes {
		if err := p.applyPlanNode(ctx, planId, &nodes[i], tx); err != nil {
			return err
		}
	}
	return nil
}

func (p *plan) syncPlanNodes(ctx context.Context, planId int64, nodes []types.CreatePlanNodeRequest) error {
	oldNodes, err := p.factory.Plan().ListNodes(ctx, planId)
	if err != nil {
		return err
	}

	keepIDs := make(map[int64]struct{}, len(nodes))
	for i := range nodes {
		if nodes[i].NodeId > 0 {
			keepIDs[nodes[i].NodeId] = struct{}{}
		}
	}

	for i := range oldNodes {
		if _, ok := keepIDs[oldNodes[i].Id]; ok {
			continue
		}
		if err := p.disassociateNode(ctx, &oldNodes[i]); err != nil {
			return err
		}
	}

	for i := range nodes {
		if err := p.applyPlanNode(ctx, planId, &nodes[i], nil); err != nil {
			return err
		}
	}
	return nil
}

// applyPlanNode 有 node_id 则绑定/更新已有节点（plan_id/role/cri，不改 ip/auth）；无 node_id 则新建计划节点。
func (p *plan) applyPlanNode(ctx context.Context, planId int64, req *types.CreatePlanNodeRequest, tx *gorm.DB) error {
	role := strings.Join(req.Role, ",")

	if req.NodeId > 0 {
		object, err := p.factory.Plan().GetNode(ctx, req.NodeId)
		if err != nil {
			if utilerrors.IsRecordNotFound(err) {
				return errors.ErrNodeNotFound
			}
			klog.Errorf("get node %d: %v", req.NodeId, err)
			return errors.ErrServerInternal
		}
		if err = util.CheckResourceAccess(ctx, p.factory, object.UserId, types.ResourceTypeNode, req.NodeId); err != nil {
			return err
		}
		if object.PlanId != 0 && object.PlanId != planId {
			return errors.ErrConflict
		}

		updates := map[string]interface{}{
			"plan_id": planId,
			"role":    role,
			"cri":     req.CRI,
		}
		if tx != nil {
			return p.factory.Plan().TxUpdateNode(ctx, tx, req.NodeId, object.ResourceVersion, updates)
		}
		return p.factory.Plan().UpdateNode(ctx, req.NodeId, object.ResourceVersion, updates)
	}

	auth, err := req.Auth.Marshal()
	if err != nil {
		return err
	}
	node := &model.Node{
		Name:   req.Name,
		UserId: req.UserId,
		PlanId: planId,
		Role:   role,
		CRI:    req.CRI,
		Ip:     req.Ip,
		Auth:   auth,
	}
	if tx != nil {
		return p.factory.Plan().TxCreateNode(ctx, tx, node)
	}
	_, err = p.factory.Plan().CreateNode(ctx, node)
	return err
}

func (p *plan) disassociateNode(ctx context.Context, node *model.Node) error {
	updates := map[string]interface{}{
		"plan_id": int64(0),
		"role":    "",
		"cri":     "",
	}
	if err := p.factory.Plan().UpdateNode(ctx, node.Id, node.ResourceVersion, updates); err != nil {
		if err == utilerrors.ErrRecordNotFound {
			return nil
		}
		klog.Errorf("disassociate node %d from plan %d: %v", node.Id, node.PlanId, err)
		return errors.ErrServerInternal
	}
	return nil
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
		CRI:    o.CRI,
	}, nil
}
