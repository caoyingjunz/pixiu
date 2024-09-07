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

package cluster

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/caoyingjunz/pixiu/pkg/types"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

var BufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

func (c *cluster) WsNodeHandler(ctx context.Context, sshConfig *types.WebSSHRequest, w http.ResponseWriter, r *http.Request) error {
	upgrader := &websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024 * 10,
		HandshakeTimeout: time.Second * 2,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{r.Header.Get("Sec-WebSocket-Protocol")},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	sshClient, err := sshutil.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	turn, err := types.NewTurn(conn, sshClient)
	if err != nil {
		return err
	}
	defer turn.Close()

	handler(turn)
	return nil
}

func handler(turn *types.Turn) {
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
