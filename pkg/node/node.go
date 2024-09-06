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
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	klog "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func RunSSH(turn *Turn) {
	logBuff := BufPool.Get().(*bytes.Buffer)
	logBuff.Reset()
	defer BufPool.Put(logBuff)

	wg := &sync.WaitGroup{}
	wg.Add(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go turn.StartLoopRead(ctx, wg, logBuff)
	go turn.StartSessionWait(wg)

	wg.Wait()
}

func SelectAuthModel(conf types.WebSSHConfig) *model.SSHClientConfig {
	var config *model.SSHClientConfig
	switch conf.AuthModel {
	case model.PASSWORD:
		config = sshClientConfigPassword(&conf)
	case model.PUBLICKEY:
		config = sshClientConfigPulicKey(&conf)
	}

	return config
}

func NewClient(wsConn *websocket.Conn, config *model.SSHClientConfig) *ssh.Client {
	client, err := newSSHClient(config)
	if err != nil {
		if err := wsConn.WriteControl(websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second)); err != nil {
			klog.Printf("Failed to write close message: %v", err)
		}
	}

	return client
}

func NewTurn(wsConn *websocket.Conn, client *ssh.Client) *Turn {
	turn, err := newTurn(wsConn, client)
	if err != nil {
		if err := wsConn.WriteControl(websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second)); err != nil {
			klog.Printf("Failed to write close message: %v", err)
		}
	}

	return turn
}
