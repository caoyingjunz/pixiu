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
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/node"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	klog "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type WebSSHConfig struct {
	Host      string          `form:"host" json:"host" binding:"required"`
	Port      int             `form:"port" json:"port"`
	User      string          `form:"user" json:"user" binding:"required"`
	Password  string          `form:"password" json:"password"`
	AuthModel model.AuthModel `form:"auth_model" json:"auth_model"`
	PkPath    string          `form:"pk_path" json:"pk_path"`
	Protocol  string          `form:"protocol" json:"protocol"`
}

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 10,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (n *nodeRouter) serveConn(c *gin.Context) {
	var (
		w types.WebSSHConfig
	)

	wsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		writeCloseMessage(wsConn, err)
		return
	}
	defer wsConn.Close()

	if err = c.ShouldBindQuery(&w); err != nil {
		writeCloseMessage(wsConn, err)
		return
	}
	preCheckSSH(&w)

	config := node.SelectAuthModel(w)
	client := node.NewClient(wsConn, config)
	defer client.Close()

	turn := node.NewTurn(wsConn, client)
	defer turn.Close()

	node.RunSSH(turn)
}

func writeCloseMessage(wsConn *websocket.Conn, err error) {
	if err := wsConn.WriteControl(websocket.CloseMessage,
		[]byte(err.Error()), time.Now().Add(time.Second)); err != nil {
		klog.Printf("Failed to write close message: %v", err)
	}
}

func preCheckSSH(config *types.WebSSHConfig) {
	if config.AuthModel == 0 {
		config.AuthModel = model.PASSWORD
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}
	if config.Port == 0 {
		config.Port = 22
	}
}
