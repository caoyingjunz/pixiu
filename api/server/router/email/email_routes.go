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

package email

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const emailBaseURL = "/pixiu/emails"

type emailMeta struct {
	EmailId int64 `uri:"id"`
}

// emailRouter is a router to talk with the email controller.
type emailRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new email router.
func NewRouter(o *options.Options) {
	r := &emailRouter{
		c: o.Controller,
	}
	r.initRoutes(o.HttpEngine)
}

func (r *emailRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "系统邮件",
		BaseURL: emailBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createEmail, Description: "创建邮件"},
			{Method: "PUT", RelativePath: "/:id", Handler: r.updateEmail, Description: "更新邮件"},
			{Method: "DELETE", RelativePath: "/:id", Handler: r.deleteEmail, Description: "删除邮件"},
			{Method: "GET", RelativePath: "/:id", Handler: r.getEmail, Description: "查看邮件详情"},
			{Method: "GET", RelativePath: "", Handler: r.listEmails, Description: "查看邮件列表"},
			{Method: "POST", RelativePath: "/:id/test", Handler: r.testSendEmail, Description: "测试邮件发送"},
		},
	}
	group.Register(ginEngine.Group(emailBaseURL), r.c.APIResource())
}

func (r *emailRouter) createEmail(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.CreateEmailRequest
	if err := httputils.BindCreateRequest(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Email().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *emailRouter) updateEmail(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta emailMeta
		req  types.UpdateEmailRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	req.Id = meta.EmailId
	if err := r.c.Email().Update(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *emailRouter) deleteEmail(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta emailMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Email().Delete(c, meta.EmailId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *emailRouter) getEmail(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta emailMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Email().Get(c, meta.EmailId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *emailRouter) listEmails(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.BindListOptionsWithUser(c, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Email().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *emailRouter) testSendEmail(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta emailMeta
		req  types.TestSendEmailRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Email().TestSend(c, meta.EmailId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}
