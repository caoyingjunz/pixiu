package cloud

import (
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"

	"github.com/gin-gonic/gin"
	"github.com/igm/sockjs-go/v3/sockjs"
	"k8s.io/client-go/tools/remotecommand"
)

func (s *cloudRouter) webShell(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		WebShellOptions types.WebShellOptions
	)
	if err := c.ShouldBindUri(&WebShellOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	sockjs.NewHandler("/webshell/ws", sockjs.DefaultOptions, func(session sockjs.Session) {
		if err := pixiu.CoreV1.Cloud().WebShell(WebShellOptions.CloudName).WebShellHandler(&types.WebShell{
			Conn:      session,
			SizeChan:  make(chan *remotecommand.TerminalSize),
			Namespace: WebShellOptions.Namespace,
			Pod:       WebShellOptions.Pod,
			Container: WebShellOptions.Container,
		}, "/bin/bash", WebShellOptions); err != nil {

		}

	}).ServeHTTP(c.Writer, c.Request)
}
