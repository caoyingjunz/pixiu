package cluster

import (
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/gin-gonic/gin"
)

func (cr *clusterRouter) listPod(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opts struct {
			Cluster   string `uri:"cluster" binding:"required"`
			Namespace string `uri:"namespace" binding:"required"`
		}
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &opts, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Pod(opts.Cluster, opts.Namespace).List(c.Request.Context()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
