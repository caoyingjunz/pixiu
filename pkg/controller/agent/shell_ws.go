/*
Copyright 2026 The Pixiu Authors.

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

package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/agentshell"
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	// maxShellSessionsPerAgent limits concurrent host shell sessions per agent.
	maxShellSessionsPerAgent = 5
	// shellPingInterval keeps the browser websocket alive through nginx.
	shellPingInterval = 25 * time.Second
	shellWriteTimeout = 10 * time.Second
	shellDialTimeout  = 15 * time.Second
)

// shellSessions tracks live shell sessions per agent (in-process only).
var shellSessions = struct {
	sync.Mutex
	counts map[int64]int
}{counts: make(map[int64]int)}

func acquireShellSession(agentId int64) bool {
	shellSessions.Lock()
	defer shellSessions.Unlock()

	if shellSessions.counts[agentId] >= maxShellSessionsPerAgent {
		return false
	}
	shellSessions.counts[agentId]++
	return true
}

func releaseShellSession(agentId int64) {
	shellSessions.Lock()
	defer shellSessions.Unlock()

	if shellSessions.counts[agentId] <= 1 {
		delete(shellSessions.counts, agentId)
		return
	}
	shellSessions.counts[agentId]--
}

// WsShellHandler opens an interactive shell on the host running a deploy agent.
func (a *agentController) WsShellHandler(ctx context.Context, agentId int64, w http.ResponseWriter, r *http.Request) error {
	// 仅 root/admin 可打开 agent 宿主机 Shell
	if err := util.CheckAdmin(ctx); err != nil {
		return err
	}

	object, err := a.factory.Agent().Get(ctx, agentId)
	if err != nil {
		klog.Errorf("failed to get agent(%d): %v", agentId, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrAgentNotFound
	}
	if object.AgentType != model.AgentTypeDeploy {
		return errors.NewError(fmt.Errorf("agent %d is not a deploy agent", agentId), http.StatusBadRequest)
	}
	// 可见性口径：仅 root、owner 或角色 scope 授权的用户可登录该代理宿主 Shell
	if err = util.CheckResourceAccess(ctx, a.factory, object.UserID, types.ResourceTypeAgent, agentId); err != nil {
		return err
	}

	sessionKey := agentshell.SessionKey(agentId)
	tm := tunnel.Default()
	if tm == nil || !tm.HasSessionKey(sessionKey) {
		return errors.NewError(fmt.Errorf("部署代理隧道未连接，请确认代理在线"), http.StatusServiceUnavailable)
	}

	if !acquireShellSession(agentId) {
		return errors.NewError(
			fmt.Errorf("该代理的 Shell 会话数已达上限(%d)，请关闭其他会话后重试", maxShellSessionsPerAgent),
			http.StatusTooManyRequests,
		)
	}
	defer releaseShellSession(agentId)

	upgrader := &websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024 * 10,
		HandshakeTimeout: time.Second * 2,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{r.Header.Get("Sec-WebSocket-Protocol")},
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// 升级失败时尚未 hijack，可交给上层返回 JSON 错误
		return err
	}
	defer wsConn.Close()

	// Upgrade 成功后连接已被 hijack，禁止再写 JSON，后续错误只通过 WebSocket 提示。
	wsConn.SetReadLimit(2 * agentshell.MaxFrameSize)

	userID, userName := int64(0), ""
	if user, uErr := httputils.GetUserFromContext(ctx); uErr == nil {
		userID, userName = user.Id, user.Name
	}
	klog.Infof("agent shell session opened: user=%d(%s) agent=%d(%s) remote=%s", userID, userName, agentId, object.Name, r.RemoteAddr)
	defer klog.Infof("agent shell session closed: user=%d(%s) agent=%d(%s) remote=%s", userID, userName, agentId, object.Name, r.RemoteAddr)

	dialCtx, cancel := context.WithTimeout(ctx, shellDialTimeout)
	defer cancel()

	conn, err := tm.DialerByKey(sessionKey)(dialCtx, "unix", agentshell.SocketPath)
	if err != nil {
		klog.Errorf("agent shell dial failed (agent=%d %s): %v", agentId, object.Name, err)
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n[SSH 连接失败] "+err.Error()+"\r\n"))
		return nil
	}
	defer conn.Close()

	bridgeShell(wsConn, conn)
	return nil
}

// wsWriter serializes websocket writes between the tunnel reader and the heartbeat.
type wsWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsWriter) writeMessage(messageType int, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_ = w.conn.SetWriteDeadline(time.Now().Add(shellWriteTimeout))
	return w.conn.WriteMessage(messageType, payload)
}

// bridgeShell pumps frames between the browser websocket and the tunnel conn.
func bridgeShell(wsConn *websocket.Conn, conn net.Conn) {
	writer := &wsWriter{conn: wsConn}

	var (
		closeOnce sync.Once
		wg        sync.WaitGroup
	)
	closeAll := func() {
		closeOnce.Do(func() {
			_ = conn.Close()
			_ = wsConn.Close()
		})
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer closeAll()
		copyBrowserToTunnel(wsConn, conn)
	}()
	go func() {
		defer wg.Done()
		defer closeAll()
		copyTunnelToBrowser(conn, writer)
	}()

	hbDone := make(chan struct{})
	go func() {
		// 服务端周期下发 ping，避免 nginx 因空闲超时断开
		ticker := time.NewTicker(shellPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbDone:
				return
			case <-ticker.C:
				if err := wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(shellWriteTimeout)); err != nil {
					klog.Errorf("agent shell heartbeat failed: %v", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	close(hbDone)
}

// copyBrowserToTunnel forwards binary frames from the browser to the agent.
// Frame layout from the browser: [1B type][base64(payload)].
func copyBrowserToTunnel(wsConn *websocket.Conn, conn net.Conn) {
	for {
		messageType, data, err := wsConn.ReadMessage()
		if err != nil {
			return
		}
		// 前端心跳等文本消息直接忽略
		if messageType != websocket.BinaryMessage || len(data) < 1 {
			continue
		}

		typ := data[0]
		if typ != agentshell.FrameData && typ != agentshell.FrameResize {
			continue
		}

		payload := make([]byte, base64.StdEncoding.DecodedLen(len(data)-1))
		n, err := base64.StdEncoding.Decode(payload, data[1:])
		if err != nil {
			klog.Warningf("agent shell invalid base64 frame (type=%d): %v", typ, err)
			continue
		}

		if err = agentshell.WriteFrame(conn, typ, payload[:n]); err != nil {
			klog.Errorf("agent shell write tunnel frame failed: %v", err)
			return
		}
	}
}

// copyTunnelToBrowser forwards raw output from the agent to the browser.
// 与既有 Node SSH 终端（pkg/types Turn.Write）一致：浏览器直接接收原始字节，
// 不带类型字节、不做 base64 编码。
func copyTunnelToBrowser(conn net.Conn, writer *wsWriter) {
	for {
		_, payload, err := agentshell.ReadFrame(conn)
		if err != nil {
			return
		}

		if err = writer.writeMessage(websocket.BinaryMessage, payload); err != nil {
			klog.Errorf("agent shell write browser frame failed: %v", err)
			return
		}
	}
}
