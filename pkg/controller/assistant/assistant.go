/*
Copyright 2026 The Pixiu Authors.

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

package assistant

import (
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/ai"
	"github.com/caoyingjunz/pixiu/pkg/controller/provider"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type Getter interface {
	Assistant() Interface
}

type Interface interface {
	Provider() provider.Interface
	AI() ai.Interface
}

type controller struct {
	provider provider.Interface
	ai       ai.Interface
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		provider: provider.New(cfg, f),
		ai:       ai.New(cfg, f),
	}
}

func (c *controller) Provider() provider.Interface {
	return c.provider
}

func (c *controller) AI() ai.Interface {
	return c.ai
}
