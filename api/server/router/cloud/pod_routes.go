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
	"github.com/gorilla/websocket"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) getLog(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		logsOptions types.LogsOptions
	)
	if err = c.ShouldBindUri(&logsOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindQuery(&logsOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// TODO: 优化
	upgrader := &websocket.Upgrader{}
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
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
		// TODO: 优化
		webshellOps types.WebShellOptions
	)
	if err := c.ShouldBindQuery(&webshellOps); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cloud().Pods(webshellOps.CloudName).NewWebShellHandler(&webshellOps, c.Writer, c.Request); err != nil {
		log.Logger.Error("webshell error", err)
		httputils.SetFailed(c, r, err)
		return
	}
}
