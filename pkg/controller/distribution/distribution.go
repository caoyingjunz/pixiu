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

package distribution

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	pixiuerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type DistributionGetter interface {
	Distribution() Interface
}

type Interface interface {
	CreateDistribution(ctx context.Context, req *types.CreateDistributionRequest) error
	UpdateDistribution(ctx context.Context, req *types.UpdateDistributionRequest) error
	DeleteDistribution(ctx context.Context, id int64) error
	GetDistribution(ctx context.Context, id int64) (*types.Distribution, error)
	ListDistributions(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type distribution struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewDistribution(cc config.Config, factory db.ShareDaoFactory) Interface {
	return &distribution{
		cc:      cc,
		factory: factory,
	}
}

func (d *distribution) CreateDistribution(ctx context.Context, req *types.CreateDistributionRequest) error {
	existing, err := d.factory.Distribution().GetDistributionByFamilyName(ctx, req.Family, req.Name)
	if err != nil {
		klog.Errorf("failed to query distribution %s/%s: %v", req.Family, req.Name, err)
		return errors.ErrServerInternal
	}
	if existing != nil {
		return errors.ErrDistributionExists
	}

	object := &model.Distribution{
		Family: req.Family,
		Name:   req.Name,
		Runner: req.Runner,
	}
	if _, err = d.factory.Distribution().CreateDistribution(ctx, object); err != nil {
		if pixiuerrors.IsUniqueConstraintError(err) {
			return errors.ErrDistributionExists
		}
		klog.Errorf("failed to create distribution %s/%s: %v", req.Family, req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (d *distribution) UpdateDistribution(ctx context.Context, req *types.UpdateDistributionRequest) error {
	id := req.Id
	object, err := d.factory.Distribution().GetDistribution(ctx, id)
	if err != nil {
		klog.Errorf("failed to get distribution %d: %v", id, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrDistributionNotFound
	}

	family := object.Family
	name := object.Name
	if req.Family != nil {
		family = *req.Family
	}
	if req.Name != nil {
		name = *req.Name
	}
	if family != object.Family || name != object.Name {
		dup, err := d.factory.Distribution().GetDistributionByFamilyName(ctx, family, name)
		if err != nil {
			klog.Errorf("failed to query distribution %s/%s: %v", family, name, err)
			return errors.ErrServerInternal
		}
		if dup != nil && dup.Id != id {
			return errors.ErrDistributionExists
		}
	}

	updates := make(map[string]interface{})
	if req.Family != nil {
		updates["family"] = *req.Family
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Runner != nil {
		updates["runner"] = *req.Runner
	}
	if len(updates) == 0 {
		return errors.ErrInvalidRequest
	}
	if err := d.factory.Distribution().UpdateDistribution(ctx, id, *req.ResourceVersion, updates); err != nil {
		if pixiuerrors.IsNotUpdated(err) {
			return errors.ErrInvalidRequest
		}
		klog.Errorf("failed to update distribution %d: %v", id, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (d *distribution) DeleteDistribution(ctx context.Context, id int64) error {
	object, err := d.factory.Distribution().DeleteDistribution(ctx, id)
	if err != nil {
		klog.Errorf("failed to delete distribution %d: %v", id, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrDistributionNotFound
	}
	return nil
}

func (d *distribution) GetDistribution(ctx context.Context, id int64) (*types.Distribution, error) {
	object, err := d.factory.Distribution().GetDistribution(ctx, id)
	if err != nil {
		klog.Errorf("failed to get distribution %d: %v", id, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrDistributionNotFound
	}
	return distributionModel2Type(object), nil
}

func (d *distribution) ListDistributions(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithDistributionNameLike(listOption.NameSelector),
		db.WithDistributionFamily(listOption.Family),
	}

	var err error
	pageResult.Total, err = d.factory.Distribution().CountDistributions(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count distributions: %v", err)
		pageResult.Message = err.Error()
		return nil, err
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := d.factory.Distribution().ListDistributions(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list distributions: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	items := make([]types.Distribution, len(objects))
	for i := range objects {
		items[i] = *distributionModel2Type(&objects[i])
	}
	pageResult.Items = items
	return pageResult, nil
}

func distributionModel2Type(o *model.Distribution) *types.Distribution {
	return &types.Distribution{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Family: o.Family,
		Name:   o.Name,
		Runner: o.Runner,
	}
}
