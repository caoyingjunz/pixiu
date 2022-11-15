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
	Create(ctx context.Context, obj *model.Cloud) (*model.Cloud, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cid int64) (*model.Cloud, error)
	Get(ctx context.Context, cid int64) (*model.Cloud, error)
	List(ctx context.Context) ([]model.Cloud, error)

	SetStatus(ctx context.Context, name string, status int) error
	GetByName(ctx context.Context, name string) (*model.Cloud, error)

	PageList(ctx context.Context, page int, pageSize int) ([]model.Cloud, int64, error)
	Count(ctx context.Context) (int64, error)

	CreateCluster(ctx context.Context, cluObj *model.Cluster) error
	UpdateCluster(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	DeleteCluster(ctx context.Context, cid int64) error
	GetCluster(ctx context.Context, cid int64) (*model.Cluster, error)

	// CreateNodes 批量创建 nodes
	CreateNodes(ctx context.Context, nodesObj []model.Node) error
	DeleteNodes(ctx context.Context, cid int64) error
	GetNodes(ctx context.Context, cid int64) ([]model.Node, error)
}

type cloud struct {
	db *gorm.DB
}

func NewCloud(db *gorm.DB) CloudInterface {
	return &cloud{db}
}

func (s *cloud) Create(ctx context.Context, obj *model.Cloud) (*model.Cloud, error) {
	// TODO: gorm 的 webhook
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now

	if err := s.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *cloud) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := s.db.Model(&model.Cloud{}).
		Where("id = ? and resource_version = ?", uid, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	return nil
}

func (s *cloud) Delete(ctx context.Context, cid int64) (*model.Cloud, error) {
	// mysql 不支持在删除的时候直接返回删除数据，新增一次查询规避
	obj, err := s.Get(ctx, cid)
	if err != nil {
		return nil, err
	}
	if err = s.db.Where("id = ?", cid).Delete(&model.Cloud{}).Error; err != nil {
		return nil, err
	}

	return obj, nil
}

func (s *cloud) Get(ctx context.Context, cid int64) (*model.Cloud, error) {
	var c model.Cloud
	if err := s.db.Where("id = ?", cid).First(&c).Error; err != nil {
		return nil, err
	}

	return &c, nil
}

func (s *cloud) List(ctx context.Context) ([]model.Cloud, error) {
	var cs []model.Cloud
	if err := s.db.Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}

func (s *cloud) PageList(ctx context.Context, page int, pageSize int) ([]model.Cloud, int64, error) {
	var cs []model.Cloud
	if err := s.db.Limit(pageSize).Offset((page - 1) * pageSize).Find(&cs).Error; err != nil {
		return nil, 0, err
	}
	total, err := s.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	return cs, total, nil
}

func (s *cloud) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.Model(&model.Cloud{}).Count(&count).Error; err != nil {
		return count, err
	}

	return count, nil
}

func (s *cloud) SetStatus(ctx context.Context, name string, status int) error {
	updates := make(map[string]interface{})
	updates["gmt_modified"] = time.Now() // 系统维护字段

	updates["status"] = status
	f := s.db.Model(&model.Cloud{}).Where("name = ?", name).Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	return nil
}

func (s *cloud) GetByName(ctx context.Context, name string) (*model.Cloud, error) {
	var c model.Cloud
	if err := s.db.Where("name = ?", name).First(&c).Error; err != nil {
		return nil, err
	}

	return &c, nil
}

func (s *cloud) CreateCluster(ctx context.Context, cluObj *model.Cluster) error {
	now := time.Now()
	cluObj.GmtCreate = now
	cluObj.GmtModified = now

	return s.db.Create(cluObj).Error
}

func (s *cloud) UpdateCluster(ctx context.Context, clusterId int64, resourceVersion int64, updates map[string]interface{}) error {
	return nil
}

func (s *cloud) DeleteCluster(ctx context.Context, cid int64) error {
	return s.db.
		Where("cloud_id = ?", cid).
		Delete(&model.Cluster{}).
		Error
}

func (s *cloud) GetCluster(ctx context.Context, clusterId int64) (*model.Cluster, error) {
	return nil, nil
}

func (s *cloud) CreateNodes(ctx context.Context, nodesObj []model.Node) error {
	now := time.Now()
	for index := range nodesObj {
		nodesObj[index].GmtCreate = now
		nodesObj[index].GmtModified = now
	}

	return s.db.Create(nodesObj).Error
}

func (s *cloud) DeleteNodes(ctx context.Context, cid int64) error {
	return s.db.
		Where("cloud_id = ?", cid).
		Delete(&model.Node{}).
		Error
}

func (s *cloud) GetNodes(ctx context.Context, cid int64) ([]model.Node, error) {
	var nodes []model.Node
	if err := s.db.Where("cloud_id = ?", cid).Find(&nodes).Error; err != nil {
		return nil, err
	}

	return nodes, nil
}
