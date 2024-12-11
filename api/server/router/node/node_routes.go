package node

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"
)

type WebSSHConfig struct {
	Host      string    `form:"host" json:"host" binding:"required"`
	Port      int       `form:"port" json:"port"`
	User      string    `form:"user" json:"user" binding:"required"`
	Password  string    `form:"password" json:"password"`
	AuthModel AuthModel `form:"auth_model" json:"auth_model"`
	PkPath    string    `form:"pk_path" json:"pk_path"`
	Protocol  string    `form:"protocol" json:"protocol"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 10,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (nr *nodeRouter) ServeConn(c *gin.Context) {
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		klog.Error("failed to upgrade websocket, ", err)
		wsConn.Close()
		return
	}
	defer wsConn.Close()

	var w WebSSHConfig
	if err := c.ShouldBindQuery(&w); err != nil {
		klog.Error("failed to parese webssh config, ", err)
		wsConn.Close()
		return
	}

	if w.Port == 0 {
		w.Port = 22
	}

	if w.Protocol == "" {
		w.Protocol = "tcp"
	}

	if w.AuthModel == 0 {
		w.AuthModel = PASSWORD
	}

	var config *SSHClientConfig

	switch w.AuthModel {
	case PASSWORD:
		config = SSHClientConfigPassword(&w)
	case PUBLICKEY:
		config = SSHClientConfigPulicKey(&w)
	}

	client, err := NewSSHClient(config)
	if err != nil {
		wsConn.WriteControl(websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	defer client.Close()

	turn, err := NewTurn(wsConn, client)
	if err != nil {
		wsConn.WriteControl(websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	defer turn.Close()

	logBuff := bufPool.Get().(*bytes.Buffer)
	logBuff.Reset()
	defer bufPool.Put(logBuff)

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := turn.LoopRead(logBuff, ctx)
		if err != nil {
			klog.Error("failed to read", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := turn.SessionWait()
		if err != nil {
			klog.Error("failed to wait session, ", err)
		}
		cancel()
	}()
	wg.Wait()
}
