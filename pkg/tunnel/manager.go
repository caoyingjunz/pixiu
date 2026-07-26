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

package tunnel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/rancher/remotedialer"
	"k8s.io/klog/v2"
)

const (
	// TokenHeader is the primary agent tunnel token header (Rancher / remotedialer compatible).
	TokenHeader = "X-API-Tunnel-Token"
	// TokenHeaderLegacy is kept for backward compatibility with earlier Pixiu agents.
	TokenHeaderLegacy = "X-Pixiu-Tunnel-Token"
	// ConnectPath is the websocket endpoint agents connect to.
	ConnectPath = "/pixiu/connect"
)

var (
	ErrAgentDisconnected = errors.New("cluster agent disconnected")

	defaultManager *Manager
	once           sync.Once
)

// TokenLookup resolves an agent tunnel token to a cluster name (session clientKey).
type TokenLookup interface {
	ClusterName(ctx context.Context, token string) (string, error)
}

// Manager wraps remotedialer.Server for Pixiu reverse tunnels.
type Manager struct {
	server *remotedialer.Server
	lookup TokenLookup

	mu        sync.RWMutex
	connected map[string]bool // last control-plane probe result by cluster name
}

// Init creates the process-wide tunnel manager. Safe to call multiple times;
// only the first successful call wins.
func Init(lookup TokenLookup) *Manager {
	once.Do(func() {
		m := &Manager{lookup: lookup}
		m.server = remotedialer.New(m.authorize, remotedialer.DefaultErrorWriter)
		m.server.ClientConnectAuthorizer = func(proto, address string) bool {
			return proto == "tcp"
		}
		defaultManager = m
		klog.Info("tunnel manager initialized")
	})
	return defaultManager
}

// Default returns the process-wide manager (may be nil before Init).
func Default() *Manager {
	return defaultManager
}

func (m *Manager) authorize(req *http.Request) (string, bool, error) {
	token := tokenFromRequest(req)
	if token == "" {
		klog.Warning("tunnel authorize denied: missing tunnel token header/query")
		return "", false, nil
	}
	if m.lookup == nil {
		return "", false, errors.New("tunnel token lookup is not configured")
	}
	name, err := m.lookup.ClusterName(req.Context(), token)
	if err != nil {
		klog.Errorf("tunnel authorize failed: %v", err)
		return "", false, err
	}
	if name == "" {
		klog.Warningf("tunnel authorize denied: token not found (len=%d)", len(token))
		return "", false, nil
	}
	klog.V(2).Infof("tunnel authorize ok: cluster=%s", name)
	return name, true, nil
}

func tokenFromRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	for _, key := range []string{TokenHeader, TokenHeaderLegacy} {
		if v := strings.TrimSpace(req.Header.Get(key)); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(req.URL.Query().Get("token")); v != "" {
		return v
	}
	return ""
}

// ServeHTTP upgrades agent websocket connections.
func (m *Manager) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	m.server.ServeHTTP(rw, req)
}

// HasSession reports whether the cluster agent tunnel is connected.
func (m *Manager) HasSession(clusterName string) bool {
	if m == nil || m.server == nil {
		return false
	}
	return m.server.HasSession(clusterName)
}

// Dialer returns a net.Dialer-compatible function that dials through the agent tunnel.
func (m *Manager) Dialer(clusterName string) func(context.Context, string, string) (net.Conn, error) {
	return m.server.Dialer(clusterName)
}

// ClusterDialContext returns a DialContext for rest.Config.
// If the agent is offline it returns ErrAgentDisconnected.
func (m *Manager) ClusterDialContext(clusterName string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if m == nil || !m.HasSession(clusterName) {
			return nil, ErrAgentDisconnected
		}
		return m.Dialer(clusterName)(ctx, network, address)
	}
}
