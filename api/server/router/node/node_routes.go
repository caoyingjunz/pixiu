/*
Copyright 2024 The Pixiu Authors.

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

package node

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type nodeMeta struct {
	NodeId int64 `uri:"nodeId" binding:"required"`
}

func (n *nodeRouter) createNode(c *gin.Context) {
	r := httputils.NewResponse()

	var req types.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	if err = n.c.Node().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (n *nodeRouter) updateNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt nodeMeta
		req types.UpdateNodeRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = n.c.Node().Update(c, opt.NodeId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (n *nodeRouter) deleteNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt nodeMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = n.c.Node().Delete(c, opt.NodeId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (n *nodeRouter) getNode(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt nodeMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = n.c.Node().Get(c, opt.NodeId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (n *nodeRouter) listNodes(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt types.ListOptions
		err error
	)
	if err = c.ShouldBindQuery(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = n.c.Node().List(c, opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
