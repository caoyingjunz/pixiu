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

package cluster

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ensureHTTPSGatewayBase 将网关 base URL 规范为 HTTPS。
// kubectl/client-go 在 http:// 上不会发送 Authorization Bearer，必须使用 https。
// 若原 URL 为 http，则改写为 https，并把端口替换为 httpsPort（Pixiu HTTPS 监听端口）。
// 若原 URL 已是 https（例如前置反代），则保持不变。
func ensureHTTPSGatewayBase(base string, httpsPort int) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("empty gateway base url")
	}
	if httpsPort <= 0 {
		httpsPort = 8443
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid gateway base url: %w", err)
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid gateway base url: missing host")
	}
	if strings.EqualFold(u.Scheme, "https") {
		return strings.TrimRight(u.String(), "/"), nil
	}
	if !strings.EqualFold(u.Scheme, "http") {
		return "", fmt.Errorf("unsupported gateway url scheme %q", u.Scheme)
	}
	u.Scheme = "https"
	host := u.Hostname()
	u.Host = net.JoinHostPort(host, strconv.Itoa(httpsPort))
	return strings.TrimRight(u.String(), "/"), nil
}
