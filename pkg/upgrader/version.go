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

package upgrader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VersionInfo 是外部版本源解析后的结果。
type VersionInfo struct {
	Version     string
	Image       string
	CommitID    string
	ReleaseTime string
}

// versionPayload 是版本源返回的版本字段集合。
type versionPayload struct {
	Version     string `json:"version"`
	Image       string `json:"image"`
	Commit      string `json:"commit"`
	ReleaseTime string `json:"release_time"`
}

func (p versionPayload) toVersionInfo() VersionInfo {
	return VersionInfo{
		Version:     strings.TrimSpace(p.Version),
		Image:       strings.TrimSpace(p.Image),
		CommitID:    strings.TrimSpace(p.Commit),
		ReleaseTime: strings.TrimSpace(p.ReleaseTime),
	}
}

type versionResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Result  *versionPayload `json:"result"`
}

// fetchLatestVersion 查询 pixiu 新发布版本信息。
func fetchLatestVersion(ctx context.Context, url string, currentVersion, currentCommit string) (*VersionInfo, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	target, err := buildVersionURL(url, currentVersion, currentCommit)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("构造版本查询请求失败: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询版本失败(%s): %v", url, err)
	}
	defer resp.Body.Close()

	// 检查请求是否正常
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询版本失败(%s): HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取版本响应失败: %v", err)
	}

	var vResp versionResponse
	if err = json.Unmarshal(body, &vResp); err != nil {
		return nil, fmt.Errorf("版本源(%s)响应不是合法 JSON: %v", url, err)
	}
	if vResp.Code != 200 {
		return nil, fmt.Errorf("版本源(%s)返回错误(code=%d): %s", url, vResp.Code, vResp.Message)
	}
	if vResp.Result == nil {
		return nil, fmt.Errorf("版本源(%s)响应缺少 result", url)
	}

	info := vResp.Result.toVersionInfo()
	if info.Version == "" {
		return nil, fmt.Errorf("未能从版本源(%s)解析出版本号", url)
	}
	if info.CommitID == "" {
		return nil, fmt.Errorf("未能从版本源(%s)解析出 commit id", url)
	}
	return &info, nil
}

// parseReleaseTime 解析版本源返回的发布时间，无法解析时返回 nil。
func parseReleaseTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}

// buildVersionURL 在版本源地址上附加当前版本与 commit 查询参数。
func buildVersionURL(rawURL, currentVersion, currentCommit string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("解析版本源地址失败(%s): %v", rawURL, err)
	}
	query := parsed.Query()
	if currentVersion != "" {
		query.Set("current_version", currentVersion)
	}
	if currentCommit != "" {
		query.Set("current_commit", currentCommit)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
