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
	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/cloud"
)

type ShareDaoFactory interface {
	Cluster() ClusterInterface
	Cloud() cloud.CloudInterface
	KubeConfig() cloud.KubeConfigInterface
}

type shareDaoFactory struct {
	db *gorm.DB
}

// TODO： 即将废弃，代码逻辑重新实现
func (f *shareDaoFactory) Cloud() cloud.CloudInterface           { return cloud.NewCloud(f.db) }
func (f *shareDaoFactory) KubeConfig() cloud.KubeConfigInterface { return cloud.NewKubeConfig(f.db) }

func (f *shareDaoFactory) Cluster() ClusterInterface {
	return newCluster(f.db)
}

func NewDaoFactory(db *gorm.DB) ShareDaoFactory {
	return &shareDaoFactory{
		db: db,
	}
}
