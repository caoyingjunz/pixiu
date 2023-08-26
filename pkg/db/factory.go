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
<<<<<<< HEAD
	Cloud() cloud.Interface
	KubeConfig() cloud.Interface
	User() user.Interface
=======
	Cloud() cloud.CloudInterface
	KubeConfig() cloud.KubeConfigInterface
>>>>>>> 9b81bdc42738ee1721f1f0336bbe081fc9ad7414
}

type shareDaoFactory struct {
	db *gorm.DB
}

<<<<<<< HEAD
func (f *shareDaoFactory) Cloud() cloud.Interface      { return cloud.NewCloud(f.db) }
func (f *shareDaoFactory) KubeConfig() cloud.Interface { return cloud.NewKubeConfig(f.db) }
func (f *shareDaoFactory) User() user.Interface        { return user.NewUser(f.db) }
=======
func (f *shareDaoFactory) Cloud() cloud.CloudInterface           { return cloud.NewCloud(f.db) }
func (f *shareDaoFactory) KubeConfig() cloud.KubeConfigInterface { return cloud.NewKubeConfig(f.db) }
>>>>>>> 9b81bdc42738ee1721f1f0336bbe081fc9ad7414

func NewDaoFactory(db *gorm.DB) ShareDaoFactory {
	return &shareDaoFactory{
		db: db,
	}
}
