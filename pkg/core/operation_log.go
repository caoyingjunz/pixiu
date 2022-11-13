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
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type OperationLogGetter interface {
	OperationLog() OperationLogInterface
}

type OperationLogInterface interface {
	Create(ctx context.Context, obj *model.OperationLog) error
	Delete(c context.Context, ids []int64) error
	List(c context.Context, page, limit int) (res *model.PageOperationLog, err error)
}

type operationLog struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newOperationLog(c *pixiu) OperationLogInterface {
	return &operationLog{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
	}
}

func (ol operationLog) Create(c context.Context, obj *model.OperationLog) error {
	if _, err := ol.factory.OperationLog().Create(c, obj); err != nil {
		log.Logger.Errorf("failed to save operation log %s: %v", obj.UserID, err)
		return err
	}
	return nil
}

func (ol operationLog) Delete(c context.Context, ids []int64) error {
	if err := ol.factory.OperationLog().Delete(c, ids); err != nil {
		log.Logger.Errorf("batch delete %s operationLog error: %v", ids, err)
		return err
	}
	return nil
}

func (ol operationLog) List(c context.Context, page, limit int) (res *model.PageOperationLog, err error) {
	operationLogs, err := ol.factory.OperationLog().List(c, page, limit)
	if err != nil {
		log.Logger.Errorf("list operationLog error: %v", err)
		return nil, err
	}
	return operationLogs, nil

}
