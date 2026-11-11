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

package upgrade

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
)

type upgradeRouter struct {
	c       controller.PixiuInterface
	enabled bool
}

// NewRouter 注册系统升级相关路由（状态由升级控制器维护）。
func NewRouter(o *options.Options) {
	r := &upgradeRouter{c: o.Controller, enabled: o.ComponentConfig.Upgrade.IsEnabled()}

	group := &apiregistry.Group{
		Name:    "自动升级",
		BaseURL: "/pixiu/upgrade",
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/latest-version", Handler: r.getLatestVersion, Description: "获取最新版本"},
			{Method: "POST", RelativePath: "", Handler: r.triggerUpgrade, Description: "触发升级"},
		},
	}
	group.Register(o.HttpEngine.Group("/pixiu/upgrade"), o.Controller.APIResource())

	versionGroup := &apiregistry.Group{
		Name:    "版本信息",
		BaseURL: "/pixiu",
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/version", Handler: r.getCurrentVersion, Description: "获取当前版本"},
		},
	}
	versionGroup.Register(o.HttpEngine.Group("/pixiu"), o.Controller.APIResource())
}

func (r *upgradeRouter) getLatestVersion(c *gin.Context) {
	resp := httputils.NewResponse()

	if err := controllerutil.CheckAdmin(c); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	status, err := r.c.Upgrade().Latest(c)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	status.Enabled = r.enabled
	resp.Result = status

	httputils.SetSuccess(c, resp)
}

func (r *upgradeRouter) getCurrentVersion(c *gin.Context) {
	resp := httputils.NewResponse()

	version, err := r.c.Upgrade().CurrentVersion(c)
	if err != nil {
		httputils.SetFailed(c, resp, apierrors.ErrServerInternal)
		return
	}
	resp.Result = gin.H{"version": version}
	httputils.SetSuccess(c, resp)
}

func (r *upgradeRouter) triggerUpgrade(c *gin.Context) {
	resp := httputils.NewResponse()

	if err := controllerutil.CheckAdmin(c); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if !r.enabled {
		httputils.SetFailed(c, resp, apierrors.NewError(fmt.Errorf("升级功能未开启"), http.StatusConflict))
		return
	}
	if err := r.c.Upgrade().TriggerUpgrade(c); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}
