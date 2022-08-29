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

package k8s

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type K8sInterface interface {
	ClusterCreate(ctx context.Context, obj *model.K8sCluster) (*model.K8sCluster, error)
	ClusterGetByName(ctx context.Context, name string) (*model.K8sCluster, error)
	ClusterGetAll(ctx context.Context) ([]model.K8sCluster, error)
}

type k8s struct {
	db *gorm.DB
}

func NewK8s(db *gorm.DB) K8sInterface {
	return &k8s{db}
}

func (s *k8s) ClusterCreate(ctx context.Context, obj *model.K8sCluster) (*model.K8sCluster, error) {
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now
	if err := s.db.Create(obj).Error; err != nil {
		return nil, err
	}

	return obj, nil
}

func (s *k8s) ClusterGetByName(ctx context.Context, name string) (*model.K8sCluster, error) {
	var d model.K8sCluster
	if err := s.db.Where("name = ?", name).First(&d).Error; err != nil {
		return nil, err
	}

	return &d, nil
}

func (s *k8s) ClusterGetAll(ctx context.Context) ([]model.K8sCluster, error) {
	var d []model.K8sCluster
	if err := s.db.Find(&d).Error; err != nil {
		return nil, err
	}

	return d, nil
}
