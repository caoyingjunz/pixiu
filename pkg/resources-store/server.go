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

package resourcesstore

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// 此代码单元提供 http 访问端口
// 使用 gin 框架
// 后续看看是否将路由加入主支
const (
	BaseUrl string = "/resourcestore"
	Port    int    = 9000
)

type App struct {
	engine *gin.Engine
	rg     *resourceGetter
}

// TODO: 后续 config 从 cloud 中获取
func NewApp() *App {
	config, err := NewConfig()
	if err != nil {
		klog.Fatalf("can't get config, err: %v\n", err)
		return nil
	}
	ctx := context.Background()
	rg := NewResourceGetter(ctx, config)

	app := &App{
		engine: gin.Default(),
		rg:     rg,
	}

	go Process(app.rg)

	routeGroup := app.engine.Group(BaseUrl)

	routeGroup.GET("/:kind/:namespace", app.GetGVRWithNS)
	routeGroup.GET("/:kind", app.GetGVR)
	routeGroup.GET("/:kind/:namespace/:name", app.GetGVRWithNSAndName)

	return app
}

func (app *App) Run() error {
	err := app.engine.Run(fmt.Sprintf(":%d", Port))
	return err
}

// 获取某个 namespace 下的某种资源
// eg: kubectl get pod -n default
func (app *App) GetGVRWithNS(c *gin.Context) {
	r := httputils.NewResponse()
	kind := c.Param("kind")
	ns := c.Param("namespace")

	gvr, err := app.rg.ParseKind(kind)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	r.Result = app.rg.store.ListByNamespace(gvr, ns)

	httputils.SetSuccess(c, r)
}

// 获取某种资源
// eg: kubectl get pod -A
func (app *App) GetGVR(c *gin.Context) {
	r := httputils.NewResponse()
	kind := c.Param("kind")

	gvr, err := app.rg.ParseKind(kind)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	r.Result = app.rg.store.ListAll(gvr)

	httputils.SetSuccess(c, r)
}

// 获取某个 namespace 下的某个资源
// eg: kubectl get pod -n default xxx
func (app *App) GetGVRWithNSAndName(c *gin.Context) {
	r := httputils.NewResponse()
	kind := c.Param("kind")
	ns := c.Param("namespace")
	name := c.Param("name")

	gvr, err := app.rg.ParseKind(kind)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	result, exist := app.rg.store.GetByNamespaceAndName(gvr, ns, name)
	if !exist {
		result = "resource not found"
	}

	r.Result = result

	httputils.SetSuccess(c, r)
}
