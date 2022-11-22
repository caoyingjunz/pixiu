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
	"github.com/gin-gonic/gin"
	"k8s.io/api/core/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err     error
		opts    meta.CreateOptions
		service v1.Service
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&service); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	service.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().Services(opts.Cloud).Create(c, &service); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err     error
		opts    meta.UpdateOptions
		service v1.Service
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&service); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	service.Name = opts.ObjectName
	service.Namespace = opts.Namespace
	err = pixiu.CoreV1.Cloud().Services(opts.Cloud).Update(c, &service)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.DeleteOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err = pixiu.CoreV1.Cloud().Services(opts.Cloud).Delete(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.GetOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Services(opts.Cloud).Get(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listServices
func (s *cloudRouter) listServices(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.ListOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Services(opts.Cloud).List(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
