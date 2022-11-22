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
	v1 "k8s.io/api/apps/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.CreateOptions
		sts  v1.StatefulSet
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&sts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	sts.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().StatefulSets(opts.Cloud).Create(c, &sts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.UpdateOptions
		sts  v1.StatefulSet
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&sts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	sts.Name = opts.ObjectName
	sts.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().StatefulSets(opts.Cloud).Update(c, &sts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.DeleteOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().StatefulSets(opts.Cloud).Delete(c, opts.Namespace, opts.ObjectName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.GetOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().StatefulSets(opts.Cloud).Get(c, opts.Namespace, opts.ObjectName)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listStatefulSets(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.ListOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().StatefulSets(opts.Cloud).List(c, opts.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
