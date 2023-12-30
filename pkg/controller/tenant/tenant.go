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

package tenant

import (
	"context"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type TenantGetter interface {
	Tenant() Interface
}

type Interface interface {
	Create(ctx context.Context, ten *types.Tenant) error
	Update(ctx context.Context, tid int64, ten *types.Tenant) error
	Delete(ctx context.Context, tid int64) error
	Get(ctx context.Context, tid int64) (*types.Tenant, error)
	List(ctx context.Context) ([]types.Tenant, error)
}

type tenant struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (t *tenant) Create(ctx context.Context, ten *types.Tenant) error {
	_, err := t.factory.Tenant().Create(ctx, &model.Tenant{
		Name:        ten.Name,
		Description: ten.Description,
	})
	if err != nil {
		klog.Errorf("failed to create tenant %s: %v", ten.Name, err)
		return err
	}

	return nil
}

// Update TODO
func (t *tenant) Update(ctx context.Context, tid int64, ten *types.Tenant) error {
	return nil
}

func (t *tenant) Delete(ctx context.Context, tid int64) error {
	_, err := t.factory.Tenant().Delete(ctx, tid)
	if err != nil {
		klog.Errorf("failed to delete %d tenant: %v", tid, err)
		return err
	}

	return nil
}

func (t *tenant) Get(ctx context.Context, tid int64) (*types.Tenant, error) {
	object, err := t.factory.Tenant().Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	return t.model2Type(object), nil
}

func (t *tenant) List(ctx context.Context) ([]types.Tenant, error) {
	return nil, nil
}

func (t *tenant) model2Type(o *model.Tenant) *types.Tenant {
	return &types.Tenant{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		Description: o.Description,
	}
}

func NewTenant(cfg config.Config, f db.ShareDaoFactory) *tenant {
	return &tenant{
		cc:      cfg,
		factory: f,
	}
}
