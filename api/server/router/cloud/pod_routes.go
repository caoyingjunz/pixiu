package cloud

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

func (s *cloudRouter) getLog(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		logsOptions types.LogsOptions
		err         error
	)

	err = c.ShouldBindUri(&logsOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err = c.ShouldBindQuery(&logsOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	ws, err := util.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	defer ws.Close()

	err = pixiu.CoreV1.Cloud().Pods(logsOptions.CloudName).Logs(c, ws, &logsOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}

func (s *cloudRouter) webShell(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		WebShellOptions types.WebShellOptions
	)
	if err := c.ShouldBindUri(&WebShellOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().Pods(WebShellOptions.CloudName).NewHandler(&WebShellOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}
