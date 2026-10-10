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

package datasourceauth

import (
	"net/url"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func logDatasource(datasourceURL string) *types.Datasource {
	return &types.Datasource{
		PixiuMeta: types.PixiuMeta{Id: 1},
		Type:      model.DatasourceTypeLog,
		Config: types.DatasourceConfig{
			Log: &types.LogSourceConfig{URL: datasourceURL},
		},
	}
}

func TestValidateExternalTarget(t *testing.T) {
	tests := []struct {
		name        string
		datasource  *types.Datasource
		targetRaw   string
		targetIsNil bool
		wantErr     bool
	}{
		{
			name:       "host/scheme/port 与数据源一致放行",
			datasource: logDatasource("http://loki.example.com:3100"),
			targetRaw:  "http://loki.example.com:3100/loki/api/v1/labels",
		},
		{
			name:       "http 默认端口 80 等价放行",
			datasource: logDatasource("http://loki.example.com"),
			targetRaw:  "http://loki.example.com:80/loki/api/v1/labels",
		},
		{
			name:       "https 默认端口 443 等价放行",
			datasource: logDatasource("https://es.example.com"),
			targetRaw:  "https://es.example.com:443/_cluster/health",
		},
		{
			name:       "host 大小写不敏感放行",
			datasource: logDatasource("http://Nacos.Example.com:8848"),
			targetRaw:  "http://nacos.example.com:8848/nacos/v1/console/health",
		},
		{
			name:       "host 不一致拒绝",
			datasource: logDatasource("http://loki.example.com:3100"),
			targetRaw:  "http://169.254.169.254:3100/loki/api/v1/labels",
			wantErr:    true,
		},
		{
			name:       "scheme 不一致拒绝",
			datasource: logDatasource("http://loki.example.com:3100"),
			targetRaw:  "https://loki.example.com:3100/loki/api/v1/labels",
			wantErr:    true,
		},
		{
			name:       "port 不一致拒绝",
			datasource: logDatasource("http://loki.example.com:3100"),
			targetRaw:  "http://loki.example.com:3101/loki/api/v1/labels",
			wantErr:    true,
		},
		{
			name: "nacos 允许端口不同",
			datasource: func() *types.Datasource {
				ds := logDatasource("http://nacos.example.com:8848")
				ds.SubType = model.DatasourceSubTypeNacos
				return ds
			}(),
			targetRaw: "http://nacos.example.com:8850/nacos/v1/console/server/state",
		},
		{
			name: "nacos 仍校验 host 一致",
			datasource: func() *types.Datasource {
				ds := logDatasource("http://nacos.example.com:8848")
				ds.SubType = model.DatasourceSubTypeNacos
				return ds
			}(),
			targetRaw: "http://other.example.com:8850/nacos/v1/console/server/state",
			wantErr:   true,
		},
		{
			name:        "target 为空拒绝",
			datasource:  logDatasource("http://loki.example.com:3100"),
			targetIsNil: true,
			wantErr:     true,
		},
		{
			name:       "数据源无代理 URL 拒绝",
			datasource: &types.Datasource{PixiuMeta: types.PixiuMeta{Id: 2}, Type: model.DatasourceTypeLog},
			targetRaw:  "http://loki.example.com:3100/loki/api/v1/labels",
			wantErr:    true,
		},
		{
			name: "alert 数据源按 alert.url 绑定放行",
			datasource: &types.Datasource{
				PixiuMeta: types.PixiuMeta{Id: 3},
				Type:      model.DatasourceTypeAlert,
				Config: types.DatasourceConfig{
					Alert: &types.AlertSourceConfig{URL: "http://alert.example.com:9093/api/v2"},
				},
			},
			targetRaw: "http://alert.example.com:9093/api/v2/alerts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target *url.URL
			if !tt.targetIsNil {
				parsed, err := url.Parse(tt.targetRaw)
				if err != nil {
					t.Fatalf("测试用例 target 非法: %v", err)
				}
				target = parsed
			}
			err := ValidateExternalTarget(tt.datasource, target)
			if tt.wantErr && err == nil {
				t.Fatal("期望拒绝，实际放行")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("期望放行，实际报错: %v", err)
			}
		})
	}
}
