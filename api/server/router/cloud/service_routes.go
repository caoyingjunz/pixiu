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
	"context"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

// listServices
func (s *cloudRouter) listServices(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		listOptions types.ListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Services(listOptions.CloudName).List(context.TODO(), listOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) createService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions types.GetOrCreateOptions
		service    corev1.Service
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&service); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	service.Name = getOptions.ObjectName
	service.Namespace = getOptions.Namespace
	if err = pixiu.CoreV1.Cloud().Services(getOptions.CloudName).Create(context.TODO(), &service); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		createOptions types.GetOrCreateOptions
		service       corev1.Service
	)
	if err = c.ShouldBindUri(&createOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// oldService, err := pixiu.CoreV1.Cloud().Services(createOptions.CloudName).Get(context.TODO(), createOptions)
	if err = c.ShouldBindJSON(&service); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	service.Name = createOptions.ObjectName
	service.Namespace = createOptions.Namespace
	err = pixiu.CoreV1.Cloud().Services(createOptions.CloudName).Update(context.TODO(), &service)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getService(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions types.GetOrDeleteOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Services(getOptions.CloudName).Get(context.TODO(), getOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
