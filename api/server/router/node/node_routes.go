package node

import (
	"bytes"
	"context"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/node"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	klog "github.com/sirupsen/logrus"
	"net/http"
	"sync"
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
		w      types.WebSSHConfig
		config *model.SSHClientConfig
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

	switch w.AuthModel {
	case model.PASSWORD:
		config = node.SSHClientConfigPassword(&w)
	case model.PUBLICKEY:
		config = node.SSHClientConfigPulicKey(&w)
	}

	client, err := node.NewSSHClient(config)
	if err != nil {
		if err := wsConn.WriteControl(websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second)); err != nil {
			klog.Printf("Failed to write close message: %v", err)
		}
		return
	}
	defer client.Close()

	turn, err := node.NewTurn(wsConn, client)
	if err != nil {
		if err := wsConn.WriteControl(websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second)); err != nil {
			klog.Printf("Failed to write close message: %v", err)
		}
		return
	}
	defer turn.Close()

	runSSH(turn)
}

func runSSH(turn *node.Turn) {
	logBuff := node.BufPool.Get().(*bytes.Buffer)
	logBuff.Reset()
	defer node.BufPool.Put(logBuff)

	wg := &sync.WaitGroup{}
	wg.Add(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go turn.StartLoopRead(ctx, wg, logBuff)
	go turn.StartSessionWait(wg)

	wg.Wait()
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
