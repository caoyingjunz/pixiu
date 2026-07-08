/*
Copyright 2021 The Pixiu Authors.

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
	"encoding/base64"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestBasicAuthFromDatasource(t *testing.T) {
	tests := []struct {
		name string
		ds   *types.Datasource
		want string
	}{
		{
			name: "log datasource",
			ds: &types.Datasource{
				Type: model.DatasourceTypeLog,
				Config: types.DatasourceConfig{
					Log: &types.LogSourceConfig{
						UserName: "log-user",
						Password: "log-pass",
					},
				},
			},
			want: "Basic " + base64.StdEncoding.EncodeToString([]byte("log-user:log-pass")),
		},
		{
			name: "alert datasource",
			ds: &types.Datasource{
				Type: model.DatasourceTypeAlert,
				Config: types.DatasourceConfig{
					Alert: &types.AlertSourceConfig{
						UserName: "alert-user",
						Password: "alert-pass",
					},
				},
			},
			want: "Basic " + base64.StdEncoding.EncodeToString([]byte("alert-user:alert-pass")),
		},
		{
			name: "alert datasource without credentials",
			ds: &types.Datasource{
				Type: model.DatasourceTypeAlert,
				Config: types.DatasourceConfig{
					Alert: &types.AlertSourceConfig{},
				},
			},
			want: "",
		},
		{
			name: "log datasource missing config",
			ds: &types.Datasource{
				Type:   model.DatasourceTypeLog,
				Config: types.DatasourceConfig{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := basicAuthFromDatasource(tt.ds); got != tt.want {
				t.Fatalf("basicAuthFromDatasource() = %q, want %q", got, tt.want)
			}
		})
	}
}
