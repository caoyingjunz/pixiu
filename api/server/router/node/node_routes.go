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
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/node"
	"github.com/caoyingjunz/pixiu/pkg/types"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

func (n *nodeRouter) serveConn(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		SSHConfig types.WebSSHRequest
		err       error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &SSHConfig); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	upgrader := &websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024 * 10,
		HandshakeTimeout: time.Second * 2,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{c.Request.Header.Get("Sec-WebSocket-Protocol")},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	defer conn.Close()

	sshClient, err := sshutil.NewSSHClient(&SSHConfig)
	if err != nil {
		WriteCloseMessage(conn, err)
		return
	}
	defer sshClient.Close()

	turn, err := types.NewTurn(conn, sshClient)
	if err != nil {
		WriteCloseMessage(conn, err)
		return
	}
	defer turn.Close()

	node.WaitFor(turn)
}

func WriteCloseMessage(conn *websocket.Conn, err error) {
	_ = conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(time.Second))
}
