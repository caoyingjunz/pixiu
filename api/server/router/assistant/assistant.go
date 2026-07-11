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

package assistant

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	assistantBaseURL    = "/pixiu/assistant"
	providerBaseURL     = assistantBaseURL + "/providers"
	conversationBaseURL = assistantBaseURL + "/conversations"
	messageBaseURL      = assistantBaseURL + "/messages"
)

type router struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	r := &router{
		c: o.Controller,
	}
	r.initRoutes(o.HttpEngine)
}

func (r *router) initRoutes(ginEngine *gin.Engine) {
	providerGroup := &apiregistry.Group{
		Name:    "智能助手",
		BaseURL: providerBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createProvider, Description: "Create assistant provider"},
			{Method: "PUT", RelativePath: "/:providerId", Handler: r.updateProvider, Description: "Update assistant provider"},
			{Method: "DELETE", RelativePath: "/:providerId", Handler: r.deleteProvider, Description: "Delete assistant provider"},
			{Method: "GET", RelativePath: "", Handler: r.listProviders, Description: "List assistant providers"},
			{Method: "GET", RelativePath: "/:providerId", Handler: r.getProvider, Description: "Get assistant provider"},
		},
	}
	providerGroup.Register(ginEngine.Group(providerBaseURL), r.c.APIResource())

	conversationGroup := &apiregistry.Group{
		Name:    "智能助手",
		BaseURL: conversationBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "DELETE", RelativePath: "/:conversationId", Handler: r.deleteConversation, Description: "Delete conversation"},
			{Method: "GET", RelativePath: "", Handler: r.listConversations, Description: "List conversations"},
			{Method: "GET", RelativePath: "/:conversationId", Handler: r.getConversation, Description: "Get conversation"},
		},
	}
	conversationGroup.Register(ginEngine.Group(conversationBaseURL), r.c.APIResource())

	messageGroup := &apiregistry.Group{
		Name:    "智能助手",
		BaseURL: messageBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "DELETE", RelativePath: "/:messageId", Handler: r.deleteMessage, Description: "Delete message"},
			{Method: "GET", RelativePath: "", Handler: r.listMessages, Description: "List messages"},
			{Method: "GET", RelativePath: "/:messageId", Handler: r.getMessage, Description: "Get message"},
		},
	}
	messageGroup.Register(ginEngine.Group(messageBaseURL), r.c.APIResource())

	respondGroup := &apiregistry.Group{
		Name:    "智能助手",
		BaseURL: assistantBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "/respond/stream", Handler: r.stream, Description: "Stream assistant response"},
		},
	}
	respondGroup.Register(ginEngine.Group(assistantBaseURL), r.c.APIResource())
}
