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

package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// versionFetchTimeout 拉取外部版本号的超时时间
	versionFetchTimeout = 15 * time.Second
	// versionBodyLimit 版本号响应体大小上限：版本号本身极短，超限视为非法响应
	versionBodyLimit = 4 << 10
)

// httpTimeoutClient 带统一超时的 HTTP 客户端（外部版本号地址通常是低速静态服务，超时从严）
type httpTimeoutClient struct {
	client *http.Client
}

func newHTTPTimeoutClient() *httpTimeoutClient {
	return &httpTimeoutClient{
		client: &http.Client{Timeout: versionFetchTimeout},
	}
}

// fetchExternalVersion 从 version_url 获取最新版本号。
// 同时兼容两种响应格式：
//   - JSON：{"version": "v1.2.3"}（优先识别）
//   - 纯文本：v1.2.3（去除首尾空白）
//
// 返回空串表示响应为空（由调用方决定是否跳过本轮）。
func fetchExternalVersion(ctx context.Context, hc *httpTimeoutClient, versionURL string) (string, error) {
	if versionURL == "" {
		return "", fmt.Errorf("未配置外部版本号地址(upgrade.version_url)")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := hc.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("响应状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, versionBodyLimit))
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			Version string `json:"version"`
		}
		if err = json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return "", fmt.Errorf("JSON 响应解析失败: %v", err)
		}
		return strings.TrimSpace(payload.Version), nil
	}
	return trimmed, nil
}

// versionFromImageTag 从镜像引用的 tag 中解析版本号（如 repo/pixiu:v1.2.3 -> v1.2.3）。
// 仅当 tag 形如版本号（vX.Y.Z[-pre]）时返回 ok=true；latest 等非版本 tag 返回 false。
func versionFromImageTag(ref string) (string, bool) {
	// tag 在最后一段路径的最后一个冒号之后；registry 端口冒号（如 1.2.3.4:5000/repo）
	// 不在最后一段内，直接取最后一段处理即可
	last := ref
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		last = ref[idx+1:]
	}
	idx := strings.LastIndex(last, ":")
	if idx < 0 {
		return "", false
	}
	tag := last[idx+1:]
	if tag == "" || tag == "latest" {
		return "", false
	}
	if _, _, ok := parseVersion(tag); !ok {
		return "", false
	}
	return tag, true
}

// CompareVersions 比较两个版本号（v 前缀可选），返回 -1/0/1。
// 支持数字段版本（段数不限，缺段补 0）与可选的预发布后缀（如 -beta.1）：
// 预发布版本低于同版本正式版；两个预发布后缀仅按字典序粗略比较。
func CompareVersions(a, b string) int {
	aNums, aPre, aOK := parseVersion(a)
	bNums, bPre, bOK := parseVersion(b)
	if !aOK || !bOK {
		// 任一不是合法版本号时退化为字符串比较，保证比较结果有确定的全序
		return strings.Compare(a, b)
	}

	n := len(aNums)
	if len(bNums) > n {
		n = len(bNums)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(aNums) {
			av = aNums[i]
		}
		if i < len(bNums) {
			bv = bNums[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}

	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "":
		return 1 // 正式版高于预发布
	case bPre == "":
		return -1
	default:
		return strings.Compare(aPre, bPre)
	}
}

// parseVersion 解析 vX.Y.Z[-pre] 形式的版本号。
func parseVersion(v string) (nums []int, pre string, ok bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nil, "", false
	}
	// 去除构建元数据（+xxx），不参与比较
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	if idx := strings.Index(v, "-"); idx >= 0 {
		pre = v[idx+1:]
		v = v[:idx]
	}
	if pre == "" && strings.Contains(v, ".") == false && !isAllDigits(v) {
		return nil, "", false
	}

	for _, seg := range strings.Split(v, ".") {
		if !isAllDigits(seg) {
			return nil, "", false
		}
		n, err := strconv.Atoi(seg)
		if err != nil {
			return nil, "", false
		}
		nums = append(nums, n)
	}
	if len(nums) == 0 {
		return nil, "", false
	}
	return nums, pre, true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
