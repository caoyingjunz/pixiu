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
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type NodeGetter interface {
	Node() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateNodeRequest) error
	Update(ctx context.Context, nodeId int64, req *types.UpdateNodeRequest) error
	Delete(ctx context.Context, nodeId int64) error
	Get(ctx context.Context, nodeId int64) (*types.NodeResult, error)
	List(ctx context.Context, opt types.ListOptions) (interface{}, error)
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
	authStr, err := req.Auth.Marshal()
	if err != nil {
		klog.Errorf("marshal node auth: %v", err)
		return errors.ErrInvalidRequest
	}

	object := &model.Node{
		Name: req.Name,
		Ip:   req.Ip,
		Auth: authStr,
	}

	_, err = n.factory.Plan().CreateNode(ctx, object)
	if err != nil {
		klog.Errorf("failed to create node %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (n *nodeController) Update(ctx context.Context, nodeId int64, req *types.UpdateNodeRequest) error {
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
	return model2Node(object), nil
}

func (n *nodeController) List(ctx context.Context, opt types.ListOptions) (interface{}, error) {
	filterOpts := []db.Options{
		db.WithNameLike(opt.NameSelector),
	}

	total, err := n.factory.Plan().CountNodes(ctx, filterOpts...)
	if err != nil {
		klog.Errorf("count nodes: %v", err)
		return nil, errors.ErrServerInternal
	}

	page := opt.Page
	if page <= 0 {
		page = 1
	}
	limit := opt.PageRequest.Limit
	if limit <= 0 {
		limit = 20
	}

	paginationOpts := append(filterOpts,
		db.WithOffset((page-1)*limit),
		db.WithLimit(int(limit)),
		db.WithOrderByDesc(),
	)

	objects, err := n.factory.Plan().ListAllNodes(ctx, paginationOpts...)
	if err != nil {
		klog.Errorf("list nodes: %v", err)
		return nil, errors.ErrServerInternal
	}

	items := make([]types.NodeResult, 0, len(objects))
	for i := range objects {
		items = append(items, *model2Node(&objects[i]))
	}

	return types.PageResponse{
		PageRequest: types.PageRequest{Page: page, Limit: int(limit)},
		Total:       int(total),
		Items:       items,
	}, nil
}

func model2Node(o *model.Node) *types.NodeResult {
	return &types.NodeResult{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name: o.Name,
		Ip:   o.Ip,
		Auth: o.Auth,
	}
}
