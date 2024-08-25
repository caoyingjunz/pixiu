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

package audit

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AuditGetter interface {
	Audit() Interface
}

type Interface interface {
	List(ctx context.Context) ([]types.Audit, error)
	Get(ctx context.Context, aid int64) (*types.Audit, error)
}

type audit struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (a *audit) Get(ctx context.Context, aid int64) (*types.Audit, error) {
	object, err := a.factory.Audit().Get(ctx, aid)
	if err != nil {
		klog.Errorf("failed to get audit %d: %v", aid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrAuditNotFound
	}
	return a.model2Type(object), nil
}

func (a *audit) List(ctx context.Context) ([]types.Audit, error) {
	objects, err := a.factory.Audit().List(ctx, db.WithOrderByDesc())
	if err != nil {
		klog.Errorf("failed to get tenants: %v", err)
		return nil, errors.ErrServerInternal
	}

	var ts []types.Audit
	for _, object := range objects {
		ts = append(ts, *a.model2Type(&object))
	}
	return ts, nil
}

func (a *audit) model2Type(o *model.Audit) *types.Audit {
	return &types.Audit{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Ip:         o.Ip,
		Action:     o.Action,
		Status:     o.Status,
		Operator:   o.Operator,
		Path:       o.Path,
		ObjectType: o.ObjectType,
	}
}

func NewAudit(cfg config.Config, f db.ShareDaoFactory) *audit {
	return &audit{
		cc:      cfg,
		factory: f,
	}
}
