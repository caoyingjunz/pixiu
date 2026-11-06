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

package system

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type systemRouter struct {
	c       controller.PixiuInterface
	factory db.ShareDaoFactory
}

// NewRouter 注册系统升级相关路由（状态由 pixiu-updater 持久化到数据库）。
func NewRouter(o *options.Options) {
	r := &systemRouter{c: o.Controller, factory: o.Factory}

	group := &apiregistry.Group{
		Name:    "系统升级",
		BaseURL: "/pixiu/system",
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/upgrade/status", Handler: r.getUpgradeStatus, Description: "查询升级状态"},
			{Method: "POST", RelativePath: "/upgrade", Handler: r.triggerUpgrade, Description: "触发升级"},
		},
	}
	group.Register(o.HttpEngine.Group("/pixiu/system"), o.Controller.APIResource())
}

func (r *systemRouter) getUpgradeStatus(c *gin.Context) {
	resp := httputils.NewResponse()

	record, err := r.factory.Upgrade().Get(c)
	if err != nil {
		httputils.SetFailed(c, resp, errors.ErrServerInternal)
		return
	}
	resp.Result = modelUpgrade2Type(record)
	httputils.SetSuccess(c, resp)
}

func (r *systemRouter) triggerUpgrade(c *gin.Context) {
	resp := httputils.NewResponse()

	if err := controllerutil.CheckAdmin(c); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}

	record, err := r.factory.Upgrade().Get(c)
	if err != nil {
		httputils.SetFailed(c, resp, errors.ErrServerInternal)
		return
	}
	switch {
	case record == nil || record.Image == "" || record.LatestVersion == "":
		httputils.SetFailed(c, resp, errors.NewError(fmt.Errorf("当前已是最新版本"), http.StatusConflict))
		return
	case record.Status == model.UpgradeStatusPending || record.Status == model.UpgradeStatusUpgrading:
		httputils.SetFailed(c, resp, errors.NewError(fmt.Errorf("升级已在进行中"), http.StatusConflict))
		return
	case !record.UpgradeAvailable &&
		record.Status != model.UpgradeStatusNotUpgraded && record.Status != model.UpgradeStatusFailed:
		httputils.SetFailed(c, resp, errors.NewError(fmt.Errorf("当前已是最新版本"), http.StatusConflict))
		return
	}

	// 0/4 -> 1：条件 UPDATE 原子迁移，抢不到说明状态已被其他请求改变
	marked, err := r.factory.Upgrade().MarkPending(c)
	if err != nil {
		httputils.SetFailed(c, resp, errors.ErrServerInternal)
		return
	}
	if !marked {
		httputils.SetFailed(c, resp, errors.NewError(fmt.Errorf("升级状态已变化，请刷新后重试"), http.StatusConflict))
		return
	}
	httputils.SetSuccess(c, resp)
}

func modelUpgrade2Type(record *model.UpgradeRecord) *types.UpgradeStatus {
	if record == nil {
		return &types.UpgradeStatus{}
	}
	return &types.UpgradeStatus{
		CurrentVersion:   record.CurrentVersion,
		LatestVersion:    record.LatestVersion,
		Image:            record.Image,
		UpgradeAvailable: record.UpgradeAvailable,
		Status:           record.Status,
		Upgrading:        record.Status == model.UpgradeStatusPending || record.Status == model.UpgradeStatusUpgrading,
		LastError:        record.LastError,
		LastCheckedAt:    record.LastCheckedAt,
	}
}
