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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"1.2.3", "v1.2.3", 0},          // v 前缀可选
		{"v1.2.3", "v1.2.4", -1},        // 补丁位
		{"v1.2.10", "v1.2.9", 1},        // 数值比较而非字典序
		{"v1.10.0", "v1.9.0", 1},        // 多位数
		{"v1.2", "v1.2.0", 0},           // 缺段补 0
		{"v2.0.0", "v1.99.99", 1},       // 高位优先
		{"v1.2.3-beta.1", "v1.2.3", -1}, // 预发布低于正式版
		{"v1.2.3", "v1.2.3-beta.1", 1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVersionFromImageTag(t *testing.T) {
	cases := []struct {
		ref string
		ok  bool
	}{
		{"crpi-xxx.cn-hangzhou.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1", true},
		{"pixiu:v1.2.3", true},
		{"pixiu:latest", false},
		{"crpi-xxx:5000/pixiu", false}, // 无 tag
		{"pixiu:stable", false},        // 非版本 tag
	}
	for _, c := range cases {
		_, ok := versionFromImageTag(c.ref)
		if ok != c.ok {
			t.Errorf("versionFromImageTag(%q) ok = %v, want %v", c.ref, ok, c.ok)
		}
	}
}

func TestFetchExternalVersion(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{"纯文本", "v2.1.0\n", "v2.1.0", false},
		{"JSON", `{"version": "v2.1.0"}`, "v2.1.0", false},
		{"空响应", "  ", "", false},
		{"非法 JSON", "{version: 1}", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()

			got, err := fetchExternalVersion(context.Background(), newHTTPTimeoutClient(), srv.URL)
			if (err != nil) != c.wantErr {
				t.Fatalf("fetchExternalVersion() err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("fetchExternalVersion() = %q, want %q", got, c.want)
			}
		})
	}
}
