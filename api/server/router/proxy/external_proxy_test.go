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

package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestRequireExternalDatasourceContext(t *testing.T) {
	tests := []struct {
		name       string
		datasource *types.Datasource
		wantErr    bool
		wantCode   int
	}{
		{
			name:       "无数据源上下文拒绝",
			datasource: nil,
			wantErr:    true,
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "有数据源上下文放行",
			datasource: &types.Datasource{PixiuMeta: types.PixiuMeta{Id: 1}},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireExternalDatasourceContext(tt.datasource)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("期望放行，实际报错: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("期望拒绝，实际放行")
			}
			var apiErr apierrors.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("期望 apierrors.Error，实际 %T: %v", err, err)
			}
			if apiErr.Code != tt.wantCode {
				t.Fatalf("期望状态码 %d，实际 %d", tt.wantCode, apiErr.Code)
			}
			if !strings.Contains(apiErr.Error(), "datasource context") {
				t.Fatalf("错误信息须点明缺少数据源上下文，实际: %v", apiErr)
			}
		})
	}
}

// TestExternalProxyHandlerRejectsWithoutDatasource 验证处理器接线：
// 未携带 X-Pixiu-Datasource-Id 的 /pixiu/external 请求必须被拒绝，且不得触达上游目标。
func TestExternalProxyHandlerRejectsWithoutDatasource(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := gin.New()
	// 无数据源请求不应访问 p.c（控制器），因此可直接使用空 proxyRouter
	p := &proxyRouter{}
	router.Any("/pixiu/external/*act", p.externalProxyHandler)

	request := httptest.NewRequest(
		http.MethodGet,
		"/pixiu/external/_cluster/health?url="+url.QueryEscape(upstream.URL),
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	// 安全断言优先：被拒绝的请求不得触达上游（SSRF 回归探测点）
	if hits := upstreamHits.Load(); hits != 0 {
		t.Fatalf("被拒绝的请求不得触达上游，实际命中 %d 次", hits)
	}

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是 JSON: %v, body=%s", err, recorder.Body.String())
	}
	if body.Code != http.StatusForbidden {
		t.Fatalf("期望业务码 %d，实际 %d (body=%s)", http.StatusForbidden, body.Code, recorder.Body.String())
	}
	if !strings.Contains(body.Message, "datasource context") {
		t.Fatalf("错误信息须点明缺少数据源上下文，实际: %s", body.Message)
	}
}
