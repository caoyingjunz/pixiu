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

package deployagent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/remotedialer"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/tunnel"
)

// RunTunnel 建立并维持与服务端的反向隧道，仅放行本机 Shell 的 Unix socket。
// 断开时每 5s 重连，ctx 取消后退出。
func RunTunnel(ctx context.Context, server, token, shellSocket string) error {
	wsURL, err := buildTunnelURL(server)
	if err != nil {
		return fmt.Errorf("invalid PIXIU_SERVER: %w", err)
	}

	headers := http.Header{}
	headers.Set(tunnel.TokenHeader, token)

	klog.Infof("deploy-agent shell tunnel connecting to %s (shell_socket=%s)", wsURL, shellSocket)
	for {
		if ctx.Err() != nil {
			return nil
		}
		err := remotedialer.ClientConnect(ctx, wsURL, headers, websocket.DefaultDialer,
			func(proto, address string) bool {
				// 仅放行本次 Shell 的 Unix socket，不开放任意地址。
				return proto == "unix" && address == shellSocket
			},
			func(ctx context.Context, _ *remotedialer.Session) error {
				klog.Infof("deploy-agent shell tunnel connected, waiting for dial requests")
				<-ctx.Done()
				return nil
			})
		if ctx.Err() != nil {
			return nil
		}
		klog.Errorf("deploy-agent shell tunnel disconnected: %v; retrying in 5s", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func buildTunnelURL(server string) (string, error) {
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = tunnel.ConnectPath
	// 不使用 query 携带 token，避免进入日志；服务端优先读取 header。
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
