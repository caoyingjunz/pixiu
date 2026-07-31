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

package clusteragent

import (
	"context"
	"crypto/tls"
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

// Agent 通过 WebSocket 反向隧道连接到 Pixiu 服务端。
type Agent struct {
	Server   string
	Token    string
	Insecure bool
}

// Run 建立并维持与服务端的隧道连接，断开时自动重连。
func (a *Agent) Run(ctx context.Context) error {
	wsURL, err := buildConnectURL(a.Server, a.Token)
	if err != nil {
		return fmt.Errorf("invalid PIXIU_SERVER: %w", err)
	}

	headers := http.Header{}
	headers.Set(tunnel.TokenHeader, a.Token)

	dialer := websocket.DefaultDialer
	if a.Insecure {
		dialer = &websocket.Dialer{
			Proxy:            http.ProxyFromEnvironment,
			HandshakeTimeout: 45 * time.Second,
			TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	klog.Infof("pixiu-cluster-agent connecting to %s (token_len=%d)", redactTokenQuery(wsURL), len(a.Token))
	for {
		if ctx.Err() != nil {
			return nil
		}
		err := remotedialer.ClientConnect(ctx, wsURL, headers, dialer, func(proto, address string) bool {
			return proto == "tcp"
		}, func(ctx context.Context, _ *remotedialer.Session) error {
			klog.Infof("tunnel connected, waiting for dial requests")
			<-ctx.Done()
			return nil
		})
		if ctx.Err() != nil {
			return nil
		}
		klog.Errorf("tunnel disconnected: %v; retrying in 5s", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func buildConnectURL(server, token string) (string, error) {
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
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func redactTokenQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("token") {
		q.Set("token", "***")
		u.RawQuery = q.Encode()
	}
	return u.String()
}
