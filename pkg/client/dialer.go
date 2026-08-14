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

package client

import (
	"context"
	"net"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
)

const (
	// defaultKubeClientTimeout 单个 kube API 请求的总超时（含重定向/重试）。
	defaultKubeClientTimeout = 60 * time.Second
	// defaultDialTimeout 直连拨号超时。
	defaultDialTimeout = 10 * time.Second
	// defaultTLSHandshakeTimeout TLS 握手超时。
	defaultTLSHandshakeTimeout = 10 * time.Second
	// defaultResponseHeaderTimeout 等待服务端响应头超时。
	defaultResponseHeaderTimeout = 30 * time.Second
	// defaultIdleConnTimeout 空闲连接保活超时。
	defaultIdleConnTimeout = 90 * time.Second
)

// DialContextFunc matches rest.Config.Dial.
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ClusterSetOptions controls how ClusterSet is built.
type ClusterSetOptions struct {
	ClusterName string
	ConnectMode model.ConnectMode
	Dial        DialContextFunc
}

// NewClusterSetWithOptions builds a ClusterSet and optionally injects a custom Dialer
// (used for Agent reverse-tunnel clusters).
func NewClusterSetWithOptions(cfg string, opts ClusterSetOptions) (*ClusterSet, error) {
	kubeConfigBytes, err := ParseKubeConfigBytes(cfg)
	if err != nil {
		return nil, err
	}

	cs := &ClusterSet{}
	if err = cs.CompleteWithOptions(kubeConfigBytes, opts); err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *ClusterSet) Complete(cfg []byte) error {
	return cs.CompleteWithOptions(cfg, ClusterSetOptions{})
}

func (cs *ClusterSet) CompleteWithOptions(cfg []byte, opts ClusterSetOptions) error {
	var err error
	if cs.Config, err = clientcmd.RESTConfigFromKubeConfig(cfg); err != nil {
		return err
	}

	dial := opts.Dial
	if dial == nil && opts.ConnectMode == model.ConnectModeTunnel && opts.ClusterName != "" {
		if tm := tunnel.Default(); tm != nil {
			dial = tm.ClusterDialContext(opts.ClusterName)
		}
	}
	if dial != nil {
		cs.Config.Dial = dial
	}

	// 显式设置总请求超时与传输层超时，避免 apiserver 无响应时请求/拨号长时间挂起。
	// 注意不能直接设置 cs.Config.Transport：client-go transport.New 中自定义 Transport
	// 与 kubeconfig 的 CA/TLS 配置互斥会报错，因此通过 WrapTransport 在 kubeconfig 生成的
	// 传输之上补齐超时参数。WrapTransport 需在 Config.Dial 设置之后配置，以便引用 dial。
	cs.Config.Timeout = defaultKubeClientTimeout
	cs.Config.WrapTransport = wrapTransport(dial)

	if cs.Client, err = kubernetes.NewForConfig(cs.Config); err != nil {
		return err
	}
	return nil
}

// wrapTransport 为 client-go 传输层补齐显式超时。
// rt 断言为 *http.Transport 并 Clone 后修改，避免污染 client-go tlsCache 共享的实例。
// 隧道模式（dial != nil）由 remotedialer 注入 DialContext，这里只对直连设置拨号超时。
func wrapTransport(dial DialContextFunc) func(rt http.RoundTripper) http.RoundTripper {
	return func(rt http.RoundTripper) http.RoundTripper {
		t, ok := rt.(*http.Transport)
		if !ok {
			return rt
		}
		t = t.Clone()
		if t.TLSHandshakeTimeout == 0 {
			t.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
		}
		if t.ResponseHeaderTimeout == 0 {
			t.ResponseHeaderTimeout = defaultResponseHeaderTimeout
		}
		if t.IdleConnTimeout == 0 {
			t.IdleConnTimeout = defaultIdleConnTimeout
		}
		if dial == nil {
			t.DialContext = (&net.Dialer{
				Timeout:   defaultDialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext
		}
		return t
	}
}
