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

	pixiumeta "github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

type AuditGetter interface {
	Audit() AuditInterface
}

type AuditInterface interface {
	Create(ctx context.Context, event *types.Event) error
	Delete(ctx context.Context) error
	List(c context.Context, selector *pixiumeta.ListSelector) ([]types.Event, error)
}

type audit struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newAudit(c *pixiu) AuditInterface {
	return &audit{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
	}
}

func (ad *audit) Create(c context.Context, event *types.Event) error {
	return nil
}

func (ad *audit) Delete(ctx context.Context) error {
	return nil
}

func (ad *audit) List(c context.Context, selector *pixiumeta.ListSelector) ([]types.Event, error) {
	return nil, nil
}
