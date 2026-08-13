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
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/httpstream"
	spdystream "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/third_party/forked/golang/netutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

const spdyPingPeriod = 5 * time.Second

// NewSPDYExecutor builds a remotecommand.Executor that honors rest.Config.Dial.
// client-go's remotecommand.NewSPDYExecutor ignores Dial (see kubernetes#112847),
// which breaks pod exec / file browse for Agent reverse-tunnel clusters.
func NewSPDYExecutor(config *rest.Config, method string, u *url.URL) (remotecommand.Executor, error) {
	wrapper, upgrader, err := RoundTripperFor(config)
	if err != nil {
		return nil, err
	}
	return remotecommand.NewSPDYExecutorForTransports(wrapper, upgrader, method, u)
}

// RoundTripperFor returns a SPDY upgrade round tripper that uses config.Dial when set.
func RoundTripperFor(config *rest.Config) (http.RoundTripper, spdy.Upgrader, error) {
	if config.Dial == nil {
		return spdy.RoundTripperFor(config)
	}

	tlsConfig, err := rest.TLSConfigFor(config)
	if err != nil {
		return nil, nil, err
	}
	upgradeRT := &dialAwareSpdyRoundTripper{
		dial:       config.Dial,
		tlsConfig:  tlsConfig,
		pingPeriod: spdyPingPeriod,
	}
	wrapper, err := rest.HTTPWrappersForConfig(config, upgradeRT)
	if err != nil {
		return nil, nil, err
	}
	return wrapper, upgradeRT, nil
}

// dialAwareSpdyRoundTripper upgrades an HTTP connection to SPDY using a custom DialContext
// (e.g. Agent remotedialer), instead of dialing the apiserver host from the Pixiu process.
type dialAwareSpdyRoundTripper struct {
	dial       DialContextFunc
	tlsConfig  *tls.Config
	pingPeriod time.Duration
	conn       net.Conn
}

var _ utilnet.TLSClientConfigHolder = &dialAwareSpdyRoundTripper{}
var _ httpstream.UpgradeRoundTripper = &dialAwareSpdyRoundTripper{}

func (s *dialAwareSpdyRoundTripper) TLSClientConfig() *tls.Config {
	return s.tlsConfig
}

func (s *dialAwareSpdyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = utilnet.CloneRequest(req)
	req.Header.Add(httpstream.HeaderConnection, httpstream.HeaderUpgrade)
	req.Header.Add(httpstream.HeaderUpgrade, spdystream.HeaderSpdy31)

	conn, err := s.dialTLS(req.Context(), req.URL)
	if err != nil {
		return nil, err
	}
	if err = req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	s.conn = conn
	return resp, nil
}

func (s *dialAwareSpdyRoundTripper) dialTLS(ctx context.Context, u *url.URL) (net.Conn, error) {
	dialAddr := netutil.CanonicalAddr(u)
	conn, err := s.dial(ctx, "tcp", dialAddr)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "http" {
		return conn, nil
	}

	host, _, err := net.SplitHostPort(dialAddr)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	tlsConfig := s.tlsConfig
	switch {
	case tlsConfig == nil:
		tlsConfig = &tls.Config{ServerName: host} //nolint:gosec
	case tlsConfig.ServerName == "":
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = host
	}
	tlsConn := tls.Client(conn, tlsConfig)
	if err = tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (s *dialAwareSpdyRoundTripper) NewConnection(resp *http.Response) (httpstream.Connection, error) {
	connectionHeader := strings.ToLower(resp.Header.Get(httpstream.HeaderConnection))
	upgradeHeader := strings.ToLower(resp.Header.Get(httpstream.HeaderUpgrade))
	if resp.StatusCode != http.StatusSwitchingProtocols ||
		!strings.Contains(connectionHeader, strings.ToLower(httpstream.HeaderUpgrade)) ||
		!strings.Contains(upgradeHeader, strings.ToLower(spdystream.HeaderSpdy31)) {
		defer resp.Body.Close()
		responseError := "unable to read error from server response"
		if responseErrorBytes, err := io.ReadAll(resp.Body); err == nil {
			responseError = strings.TrimSpace(string(responseErrorBytes))
		}
		return nil, fmt.Errorf("unable to upgrade connection: %s", responseError)
	}
	return spdystream.NewClientConnectionWithPings(s.conn, s.pingPeriod)
}
