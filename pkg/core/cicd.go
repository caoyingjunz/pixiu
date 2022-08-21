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
	"fmt"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
)

type CicdGetter interface {
	Cicd() CicdInterface
}

type CicdInterface interface {
	CreateJob(ctx context.Context) error
}

type cicd struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newCicd(c *pixiu) CicdInterface {
	return &cicd{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
	}
}

func (c *cicd) CreateJob(ctx context.Context) error {
	fmt.Println("do test cicd")
	return nil
}
