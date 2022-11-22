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

func (s *cloudRouter) createDeployment(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.CreateOptions
		d    v1.Deployment
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&d); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	d.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().Deployments(opts.Cloud).Create(c, &d); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateDeployment(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.DeleteOptions
		d    v1.Deployment
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&d); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	d.Name = opts.ObjectName
	d.Namespace = opts.Namespace
	err = pixiu.CoreV1.Cloud().Deployments(opts.Cloud).Update(c, &d)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteDeployment(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.UpdateOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Deployments(opts.Cloud).Delete(c, opts.Namespace, opts.ObjectName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listDeployments API: clouds/<cloud_name>/namespaces/<ns>/deployments
func (s *cloudRouter) listDeployments(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.ListOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().Deployments(opts.Cloud).List(c, opts.Namespace); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
