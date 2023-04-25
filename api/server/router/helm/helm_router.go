package helm

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/pixiu"
)

func (h helmRouter) ListReleasesHandler(c *gin.Context) {
	resp := httputils.NewResponse()
	var helm Helm
	if err := c.ShouldBindUri(&helm); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if releases, err := pixiu.CoreV1.Helm().ListDeployedReleases(helm.CloudName, helm.Namespace); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	} else {
		resp.Result = releases
	}
	httputils.SetSuccess(c, resp)
}
