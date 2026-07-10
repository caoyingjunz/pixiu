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

package ai

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const aiBaseURL = "/pixiu/ai"

type aiRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	r := &aiRouter{c: o.Controller}
	r.initRoutes(o.HttpEngine)
}

func (r *aiRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "AI",
		BaseURL: aiBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "/respond/stream", Handler: r.respondStream, Description: "Stream ai response with configured ai account"},
		},
	}
	group.Register(ginEngine.Group(aiBaseURL), r.c.APIResource())
}
