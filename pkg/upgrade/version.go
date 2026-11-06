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
	"regexp"
	"strconv"
	"strings"
	"time"
)

var versionRe = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.\-+]+)?`)

// VersionInfo 是外部版本源解析后的结果。
type VersionInfo struct {
	Version string
	Image   string
}

// fetchLatestVersion 查询外部版本源。
// 兼容两种响应：JSON（version/latest_version/tag + image）与纯文本/HTML（正则提取首个版本号）。
func fetchLatestVersion(ctx context.Context, url string) (*VersionInfo, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造版本查询请求失败: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询版本失败(%s): %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询版本失败(%s): HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取版本响应失败: %v", err)
	}

	info := &VersionInfo{}
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) == nil {
		info.Version = pickString(payload, "version", "latest_version", "latestVersion", "tag")
		info.Image = pickString(payload, "image", "image_url", "imageUrl")
		if data, ok := payload["data"].(map[string]interface{}); ok {
			if info.Version == "" {
				info.Version = pickString(data, "version", "latest_version", "latestVersion", "tag")
			}
			if info.Image == "" {
				info.Image = pickString(data, "image", "image_url", "imageUrl")
			}
		}
	}
	if info.Version == "" {
		info.Version = versionRe.FindString(string(body))
	}
	info.Version = strings.TrimSpace(info.Version)
	if info.Version == "" {
		return nil, fmt.Errorf("未能从版本源(%s)解析出版本号", url)
	}
	return info, nil
}

func pickString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// compareVersion 比较语义化版本：a>b 返回 1，a<b 返回 -1，相等返回 0。
// 解析失败时按字符串比较，调用方仅在结果 > 0 时升级。
func compareVersion(a, b string) int {
	aMain, aPre := splitVersion(a)
	bMain, bPre := splitVersion(b)
	if aMain == nil || bMain == nil {
		return strings.Compare(a, b)
	}
	for i := 0; i < 3; i++ {
		if aMain[i] != bMain[i] {
			if aMain[i] > bMain[i] {
				return 1
			}
			return -1
		}
	}
	// 无预发布 > 有预发布
	switch {
	case aPre == "" && bPre != "":
		return 1
	case aPre != "" && bPre == "":
		return -1
	case aPre == bPre:
		return 0
	case aPre > bPre:
		return 1
	default:
		return -1
	}
}

func splitVersion(version string) ([]int, string) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	mainPart := version
	pre := ""
	if idx := strings.IndexAny(version, "-+"); idx >= 0 {
		mainPart, pre = version[:idx], version[idx+1:]
	}
	segments := strings.Split(mainPart, ".")
	if len(segments) != 3 {
		return nil, ""
	}
	numbers := make([]int, 3)
	for i, segment := range segments {
		number, err := strconv.Atoi(segment)
		if err != nil {
			return nil, ""
		}
		numbers[i] = number
	}
	return numbers, pre
}

// imageTag 返回镜像引用的 tag（去掉仓库与 digest）。
func imageTag(image string) string {
	image = strings.TrimSpace(image)
	if idx := strings.Index(image, "@"); idx >= 0 {
		image = image[:idx]
	}
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		image = image[idx+1:]
	}
	if idx := strings.LastIndex(image, ":"); idx >= 0 {
		return image[idx+1:]
	}
	return image
}
