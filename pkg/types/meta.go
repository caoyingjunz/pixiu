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

package types

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	timeLayout = "2006-01-02 15:04:05.999999999"

	MsgData   = '1'
	MsgResize = '2'
)

func (c *Cluster) SetId(i int64) {
	c.Id = i
}

func (o *KubeObject) SetReplicaSets(replicaSets []appv1.ReplicaSet) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.ReplicaSets = replicaSets
}

func (o *KubeObject) GetReplicaSets() []appv1.ReplicaSet {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.ReplicaSets
}

func (o *KubeObject) SetPods(pods []v1.Pod) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Pods = pods
}

func (o *KubeObject) GetPods() []v1.Pod {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.Pods
}

func FormatTime(GmtCreate time.Time, GmtModified time.Time) TimeSpec {
	return TimeSpec{
		GmtCreate:   GmtCreate.Format(timeLayout),
		GmtModified: GmtModified.Format(timeLayout),
	}
}

// NewTerminalSession 该方法用于升级 http 协议至 websocket，并new一个 TerminalSession 类型的对象返回
func NewTerminalSession(w http.ResponseWriter, r *http.Request) (*TerminalSession, error) {
	// 初始化 Upgrader 类型的对象，用于http协议升级为 websocket 协议
	upgrader := &websocket.Upgrader{
		HandshakeTimeout: time.Second * 2,
		// 检测请求来源
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{r.Header.Get("Sec-WebSocket-Protocol")},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	session := &TerminalSession{
		wsConn:   conn,
		sizeChan: make(chan remotecommand.TerminalSize),
		doneChan: make(chan struct{}),
	}

	return session, nil
}

// 用于读取web端的输入，接收web端输入的指令内容
func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		return copy(p, "\u0004"), err
	}
	// 反序列化
	var msg TerminalMessage
	if err = json.Unmarshal(message, &msg); err != nil {
		return copy(p, "\u0004"), err
	}
	// 逻辑判断
	switch msg.Operation {
	// 如果是标准输入
	case "stdin":
		return copy(p, msg.Data), nil
	// 窗口调整大小
	case "resize":
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	// ping	无内容交互
	case "ping":
		return 0, nil
	default:
		return copy(p, "\u0004"), fmt.Errorf("unknown message type")
	}
}

// 写数据的方法，拿到 api-server 的返回内容，向web端输出
func (t *TerminalSession) Write(p []byte) (int, error) {
	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      string(p),
	})
	if err != nil {
		return 0, err
	}
	if err = t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Done 标记关闭doneChan,关闭后触发退出终端
func (t *TerminalSession) Done() {
	close(t.doneChan)
}

// Close 用于关闭websocket连接
func (t *TerminalSession) Close() error {
	return t.wsConn.Close()
}

// Next 获取web端是否resize,以及是否退出终端
func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

func NewTurn(wsConn *websocket.Conn, sshClient *ssh.Client) (*Turn, error) {
	session, err := sshClient.NewSession()
	if err != nil {
		return nil, err
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	turn := &Turn{StdinPipe: stdinPipe, Session: session, WsConn: wsConn}
	session.Stdout = turn
	session.Stderr = turn

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echo
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	if err = session.RequestPty("xterm", 150, 30, modes); err != nil {
		return nil, err
	}
	if err = session.Shell(); err != nil {
		return nil, err
	}

	return turn, nil
}

func (t *Turn) Write(p []byte) (n int, err error) {
	writer, err := t.WsConn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

	return writer.Write(p)
}

func (t *Turn) Close() error {
	if t.Session != nil {
		t.Session.Close()
	}
	return t.WsConn.Close()
}

func (t *Turn) Read(p []byte) (n int, err error) {
	for {
		msgType, reader, err := t.WsConn.NextReader()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		return reader.Read(p)
	}
}

func (t *Turn) StartLoopRead(ctx context.Context, wg *sync.WaitGroup, logBuff *bytes.Buffer) {
	defer wg.Done()
	err := t.loopRead(logBuff, ctx)
	if err != nil {
		klog.Errorf("LoopRead exit, err:%s", err)
	}
}

func (t *Turn) loopRead(logBuff *bytes.Buffer, context context.Context) error {
	for {
		select {
		case <-context.Done():
			return fmt.Errorf("LoopRead exit")
		default:
			_, wsData, err := t.WsConn.ReadMessage()
			if err != nil {
				return fmt.Errorf("reading webSocket message err:%s", err)
			}
			body := decode(wsData[1:])

			switch wsData[0] {
			case MsgResize:
				if err := t.resizeDo(body); err != nil {
					return err
				}
			case MsgData:
				if err := t.dataDo(body, logBuff); err != nil {
					return err
				}
			}
		}
	}
}

func (t *Turn) dataDo(body []byte, logBuff *bytes.Buffer) error {
	if _, err := t.StdinPipe.Write(body); err != nil {
		return fmt.Errorf("StdinPipe write err:%s", err)
	}

	if _, err := logBuff.Write(body); err != nil {
		return fmt.Errorf("logBuff write err:%s", err)
	}
	return nil
}

type Resize struct {
	Columns int
	Rows    int
}

func (t *Turn) resizeDo(body []byte) error {
	var args Resize
	err := json.Unmarshal(body, &args)
	if err != nil {
		return fmt.Errorf("ssh pty resize windows err:%s", err)
	}

	if args.Columns > 0 && args.Rows > 0 {
		if err := t.Session.WindowChange(args.Rows, args.Columns); err != nil {
			return fmt.Errorf("ssh pty resize windows err:%s", err)
		}
	}
	return nil
}

func (t *Turn) sessionWait() error {
	if err := t.Session.Wait(); err != nil {
		return err
	}
	return nil
}

func (t *Turn) StartSessionWait(wg *sync.WaitGroup) {
	defer wg.Done()
	err := t.sessionWait()
	if err != nil {
		klog.Errorf("SessionWait exit, err:%s", err)
	}
}

func decode(p []byte) []byte {
	decodeString, _ := base64.StdEncoding.DecodeString(string(p))
	return decodeString
}

func (a *PlanNodeAuth) Marshal() (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *PlanNodeAuth) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), a); err != nil {
		return err
	}
	return nil
}

func (ks *KubernetesSpec) Marshal() (string, error) {
	data, err := json.Marshal(ks)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ks *KubernetesSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), ks); err != nil {
		return err
	}
	return nil
}

func (ns *NetworkSpec) Marshal() (string, error) {
	data, err := json.Marshal(ns)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ns *NetworkSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), ns); err != nil {
		return err
	}
	return nil
}

func (rs *RuntimeSpec) Marshal() (string, error) {
	data, err := json.Marshal(rs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (rs *RuntimeSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), rs); err != nil {
		return err
	}
	return nil
}

func (cs ComponentSpec) Marshal() (string, error) {
	data, err := json.Marshal(cs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (cs *ComponentSpec) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), cs); err != nil {
		return err
	}
	return nil
}

func (rs *RuntimeSpec) IsDocker() bool {
	return rs.Runtime == string(model.DockerCRI)
}

func (rs *RuntimeSpec) IsContainerd() bool {
	return rs.Runtime == string(model.ContainerdCRI)
}

func (p PageRequest) IsPaged() bool {
	return p.Page != 0 && p.Limit != 0
}

func (p PageRequest) Offset(total int) (int, int, error) {
	offset := (p.Page - 1) * p.Limit
	if offset > total {
		return 0, 0, fmt.Errorf("invaild offset")
	}

	end := offset + p.Limit
	if end > total {
		end = total
	}

	return offset, end, nil
}

func (node *KubeNode) Marshal() (string, error) {
	data, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (node *KubeNode) Unmarshal(s string) error {
	if err := json.Unmarshal([]byte(s), node); err != nil {
		return err
	}
	return nil
}
