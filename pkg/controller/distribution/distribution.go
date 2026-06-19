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
	pixiuerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type DistributionGetter interface {
	Distribution() Interface
}

type Interface interface {
	CreateDistribution(ctx context.Context, req *types.CreateDistributionRequest) error
	UpdateDistribution(ctx context.Context, id int64, req *types.UpdateDistributionRequest) error
	DeleteDistribution(ctx context.Context, id int64) error
	GetDistribution(ctx context.Context, id int64) (*types.Distribution, error)
	ListDistributions(ctx context.Context, req *types.ListDistributionRequest) (*types.PageResponse, error)
	GetDistributionsMeta(ctx context.Context) (*types.DistributionsMeta, error)
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

const (
	distributionFamilyCentos    = "centos"
	distributionFamilyUbuntu    = "ubuntu"
	distributionFamilyDebian    = "debian"
	distributionFamilyOpenEuler = "openEuler"
	distributionFamilyRocky     = "rocky"
)

func defaultDistributionSeeds(cfg config.Config) []model.Distribution {
	seeds := make([]model.Distribution, 0)
	engines := cfg.Worker.Engines
	if len(engines) == 0 {
		engines = []config.Engine{
			{
				Image:       "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2",
				OSSupported: []string{"centos7", "debian10", "ubuntu18.04"},
			},
			{
				Image:       "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.2",
				OSSupported: []string{"debian11", "ubuntu20.04", "ubuntu22.04", "rocky8.5", "rocky9.2", "rocky9.3", "openEuler22.03", "openEuler24.03"},
			},
		}
	}
	for _, engine := range engines {
		for _, version := range engine.OSSupported {
			seeds = append(seeds, model.Distribution{
				Family:      inferDistributionFamily(version),
				Version:     version,
				EngineImage: engine.Image,
			})
		}
	}
	return seeds
}

func inferDistributionFamily(version string) string {
	switch {
	case len(version) >= 6 && version[:6] == "centos":
		return distributionFamilyCentos
	case len(version) >= 6 && version[:6] == "ubuntu":
		return distributionFamilyUbuntu
	case len(version) >= 6 && version[:6] == "debian":
		return distributionFamilyDebian
	case len(version) >= 9 && version[:9] == "openEuler":
		return distributionFamilyOpenEuler
	case len(version) >= 5 && version[:5] == "rocky":
		return distributionFamilyRocky
	default:
		return version
	}
}

func (d *distribution) ensureDefaultDistributions(ctx context.Context) error {
	count, err := d.factory.Distribution().CountDistributions(ctx)
	if err != nil {
		klog.Errorf("failed to count distributions: %v", err)
		return errors.ErrServerInternal
	}
	if count > 0 {
		return nil
	}

	for _, seed := range defaultDistributionSeeds(d.cc) {
		object := seed
		if _, err = d.factory.Distribution().CreateDistribution(ctx, &object); err != nil {
			if pixiuerrors.IsUniqueConstraintError(err) {
				continue
			}
			klog.Errorf("failed to seed distribution %s/%s: %v", seed.Family, seed.Version, err)
			return errors.ErrServerInternal
		}
	}
	return nil
}

func (d *distribution) CreateDistribution(ctx context.Context, req *types.CreateDistributionRequest) error {
	existing, err := d.factory.Distribution().GetDistributionByFamilyVersion(ctx, req.Family, req.Version)
	if err != nil {
		klog.Errorf("failed to query distribution %s/%s: %v", req.Family, req.Version, err)
		return errors.ErrServerInternal
	}
	if existing != nil {
		return errors.ErrDistributionExists
	}

	object := &model.Distribution{
		Family:      req.Family,
		Version:     req.Version,
		EngineImage: req.EngineImage,
	}
	if _, err = d.factory.Distribution().CreateDistribution(ctx, object); err != nil {
		if pixiuerrors.IsUniqueConstraintError(err) {
			return errors.ErrDistributionExists
		}
		klog.Errorf("failed to create distribution %s/%s: %v", req.Family, req.Version, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (d *distribution) UpdateDistribution(ctx context.Context, id int64, req *types.UpdateDistributionRequest) error {
	object, err := d.factory.Distribution().GetDistribution(ctx, id)
	if err != nil {
		klog.Errorf("failed to get distribution %d: %v", id, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrDistributionNotFound
	}

	family := object.Family
	version := object.Version
	if req.Family != nil {
		family = *req.Family
	}
	if req.Version != nil {
		version = *req.Version
	}
	if family != object.Family || version != object.Version {
		dup, err := d.factory.Distribution().GetDistributionByFamilyVersion(ctx, family, version)
		if err != nil {
			klog.Errorf("failed to query distribution %s/%s: %v", family, version, err)
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
	if req.Version != nil {
		updates["version"] = *req.Version
	}
	if req.EngineImage != nil {
		updates["engine_image"] = *req.EngineImage
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

func (d *distribution) ListDistributions(ctx context.Context, req *types.ListDistributionRequest) (*types.PageResponse, error) {
	if err := d.ensureDefaultDistributions(ctx); err != nil {
		return nil, err
	}

	opts := []db.Options{}
	if req != nil {
		if req.Family != "" {
			opts = append(opts, db.WithDistributionFamily(req.Family))
		}
		if req.VersionSelector != "" {
			opts = append(opts, db.WithDistributionVersionLike(req.VersionSelector))
		}
	}

	total, err := d.factory.Distribution().CountDistributions(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count distributions: %v", err)
		return nil, errors.ErrServerInternal
	}

	pageReq := types.PageRequest{}
	if req != nil {
		pageReq = req.PageRequest
		if req.Page > 0 && req.Limit > 0 {
			opts = append(opts, db.WithOffset((req.Page-1)*req.Limit), db.WithLimit(req.Limit))
		}
	}

	objects, err := d.factory.Distribution().ListDistributions(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list distributions: %v", err)
		return nil, errors.ErrServerInternal
	}

	items := make([]types.Distribution, len(objects))
	for i := range objects {
		items[i] = *distributionModel2Type(&objects[i])
	}

	return &types.PageResponse{
		PageRequest: pageReq,
		Total:       int(total),
		Items:       items,
	}, nil
}

func (d *distribution) GetDistributionsMeta(ctx context.Context) (*types.DistributionsMeta, error) {
	if err := d.ensureDefaultDistributions(ctx); err != nil {
		return nil, err
	}

	objects, err := d.factory.Distribution().ListDistributions(ctx)
	if err != nil {
		klog.Errorf("failed to list distributions: %v", err)
		return nil, errors.ErrServerInternal
	}

	meta := &types.DistributionsMeta{}
	for _, object := range objects {
		switch object.Family {
		case distributionFamilyCentos:
			meta.Centos = append(meta.Centos, object.Version)
		case distributionFamilyUbuntu:
			meta.Ubuntu = append(meta.Ubuntu, object.Version)
		case distributionFamilyDebian:
			meta.Debian = append(meta.Debian, object.Version)
		case distributionFamilyOpenEuler:
			meta.OpenEuler = append(meta.OpenEuler, object.Version)
		case distributionFamilyRocky:
			meta.Rocky = append(meta.Rocky, object.Version)
		default:
			klog.Warningf("unknown distribution family %s for version %s", object.Family, object.Version)
		}
	}
	return meta, nil
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
		Family:      o.Family,
		Version:     o.Version,
		EngineImage: o.EngineImage,
	}
}
