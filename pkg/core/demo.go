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

package core

import (
	"context"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
)

type DemoGetter interface {
	Demo() DemoInterface
}

type DemoInterface interface {
	Create(ctx context.Context, obj *types.Demo) error
	Get(ctx context.Context, did int64) (*types.Demo, error)
}

type demo struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newDemo(c *pixiu) DemoInterface {
	return &demo{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
	}
}

func (c *demo) Create(ctx context.Context, obj *types.Demo) error {
	// do something
	_, err := c.factory.Demo().Create(ctx, &model.Demo{Name: obj.Name})
	if err != nil {
		log.Logger.Errorf("failed to create %s demo: %v", obj.Name, err)
		return err
	}

	return nil
}

func (c *demo) Get(ctx context.Context, did int64) (*types.Demo, error) {
	obj, err := c.factory.Demo().Get(ctx, did)
	if err != nil {
		log.Logger.Errorf("failed to get % demo: %v", did, err)
		return nil, err
	}
	return &types.Demo{
		Id:              obj.Id,
		ResourceVersion: obj.ResourceVersion,
		Name:            obj.Name,
	}, nil
}
