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
	corev1 "k8s.io/api/core/v1"

	pixiumeta "github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createNamespace(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts pixiumeta.CloudMeta
		ns   corev1.Namespace
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&ns); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Namespaces(opts.Cloud).Create(c, ns); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateNamespace(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts pixiumeta.NamespaceMeta
		ns   corev1.Namespace
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&ns); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	ns.Name = opts.ObjectName
	r.Result, err = pixiu.CoreV1.Cloud().Namespaces(opts.Cloud).Update(c, ns)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteNamespace(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts pixiumeta.NamespaceMeta
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Namespaces(opts.Cloud).Delete(c, opts.ObjectName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getNamespace(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts pixiumeta.NamespaceMeta
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
	}
	r.Result, err = pixiu.CoreV1.Cloud().Namespaces(opts.Cloud).Get(c, opts.ObjectName)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listNamespaces(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts pixiumeta.CloudMeta
	)

	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().Namespaces(opts.Cloud).List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
