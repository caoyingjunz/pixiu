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
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

func (c *cluster) WsHandler(ctx context.Context, opt *types.WebShellOptions, w http.ResponseWriter, r *http.Request) error {
	cs, err := c.GetClusterSetByName(ctx, opt.Cluster)
	if err != nil {
		klog.Errorf("failed to get cluster(%s) client set: %v", opt.Cluster, err)
		return err
	}

	session, err := types.NewTerminalSession(w, r)
	if err != nil {
		return err
	}
	// 处理关闭
	defer func() {
		_ = session.Close()
	}()
	klog.Infof("connecting to %s/%s,", opt.Namespace, opt.Pod)

	cmd := opt.Command
	if len(cmd) == 0 {
		cmd = "/bin/bash"
	}

	// 组装 POST 请求
	req := cs.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opt.Pod).
		Namespace(opt.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: opt.Container,
			Command:   []string{cmd},
			Stderr:    true,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	// remotecommand 主要实现了http 转 SPDY 添加X-Stream-Protocol-Version相关header 并发送请求
	executor, err := remotecommand.NewSPDYExecutor(cs.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	// 与 kubelet 建立 stream 连接
	if err = executor.Stream(remotecommand.StreamOptions{
		Stdout:            session,
		Stdin:             session,
		Stderr:            session,
		TerminalSizeQueue: session,
		Tty:               true,
	}); err != nil {
		_, _ = session.Write([]byte("exec pod command failed," + err.Error()))
		// 标记关闭terminal
		session.Done()
	}

	return nil
}

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

	// 处理连接
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
