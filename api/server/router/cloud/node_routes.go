package cloud

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) listNodes(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		listOptions types.NodeListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Nodes(listOptions.CloudName).List(context.TODO())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getNode(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions types.GetNodeOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Nodes(getOptions.CloudName).Get(context.TODO(), getOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
