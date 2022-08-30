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

package cloud

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type CloudInterface interface {
	ClusterCreate(ctx context.Context, obj *model.CloudCluster) (*model.CloudCluster, error)
	ClusterGet(ctx context.Context, name string) (*model.CloudCluster, error)
	ClusterGetAll(ctx context.Context) ([]model.CloudCluster, error)
	ClusterDelete(ctx context.Context, name string) (*model.CloudCluster, error)
}

type cloud struct {
	db *gorm.DB
}

func NewCloud(db *gorm.DB) CloudInterface {
	return &cloud{db}
}

func (s *cloud) ClusterCreate(ctx context.Context, obj *model.CloudCluster) (*model.CloudCluster, error) {
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now
	if err := s.db.Create(obj).Error; err != nil {
		return nil, err
	}

	return obj, nil
}

func (s *cloud) ClusterGet(ctx context.Context, name string) (*model.CloudCluster, error) {
	var d model.CloudCluster
	if err := s.db.Where("name = ?", name).First(&d).Error; err != nil {
		return nil, err
	}

	return &d, nil
}

func (s *cloud) ClusterGetAll(ctx context.Context) ([]model.CloudCluster, error) {
	var d []model.CloudCluster
	if err := s.db.Find(&d).Error; err != nil {
		return nil, err
	}

	return d, nil
}

func (s *cloud) ClusterDelete(ctx context.Context, name string) (*model.CloudCluster, error) {
	var d model.CloudCluster
	if err := s.db.Where("name = ?", name).Delete(&d).Error; err != nil {
		return nil, err
	}

	return &d, nil
}
