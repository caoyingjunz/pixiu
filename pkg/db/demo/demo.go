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

package demo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type DemoInterface interface {
	Get(ctx context.Context, did int64) (*model.Demo, error)
	Create(ctx context.Context, obj *model.Demo) (*model.Demo, error)
}

type demo struct {
	db *gorm.DB
}

func NewDemo(db *gorm.DB) DemoInterface {
	return &demo{db}
}

func (s *demo) Get(ctx context.Context, did int64) (*model.Demo, error) {
	var d model.Demo
	if err := s.db.Where("id = ?", did).First(&d).Error; err != nil {
		return nil, err
	}

	return &d, nil
}

func (s *demo) Create(ctx context.Context, obj *model.Demo) (*model.Demo, error) {
	// 系统维护自动
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now
	if err := s.db.Create(obj).Error; err != nil {
		return nil, err
	}

	return obj, nil
}
