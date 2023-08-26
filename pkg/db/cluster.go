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

package db

import (
	"context"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type ClusterInterface interface {
	Create(ctx context.Context, object *model.Cluster) error
}

type cluster struct {
	db *gorm.DB
}

func (c *cluster) Create(ctx context.Context, object *model.Cluster) error {
	return nil
}

func newCluster(db *gorm.DB) ClusterInterface {
	return &cluster{db}
}
