package cloud

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) listEventsDeploymentByName(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err     error
		options types.GetOrDeleteOptions
	)
	if err = c.ShouldBindUri(&options); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Events(options.CloudName).ListEventsOfDeploymentByName(context.TODO(), options)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
