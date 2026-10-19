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

package storage

import (
	"net/http"
	"strings"
	"testing"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func requireAPIError(t *testing.T, err error, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %d, got nil", wantCode)
	}
	apiErr, ok := err.(apierrors.Error)
	if !ok {
		t.Fatalf("expected apierrors.Error, got %T: %v", err, err)
	}
	if apiErr.Code != wantCode {
		t.Fatalf("expected error code %d, got %d (%v)", wantCode, apiErr.Code, err)
	}
	if wantMessage != "" && !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("expected error message to contain %q, got %q", wantMessage, err.Error())
	}
}

func TestValidateStorageDatasource(t *testing.T) {
	tests := []struct {
		name        string
		object      *model.Datasource
		wantMessage string
	}{
		{
			name: "middleware storage without external flag",
			// storage 保持既有口径：不校验 External，External=false 也应通过。
			object: &model.Datasource{
				Type:     model.DatasourceTypeMiddleware,
				SubType:  model.DatasourceSubTypeStorage,
				External: false,
			},
		},
		{
			name: "middleware storage external",
			object: &model.Datasource{
				Type:     model.DatasourceTypeMiddleware,
				SubType:  model.DatasourceSubTypeStorage,
				External: true,
			},
		},
		{
			name: "wrong datasource type",
			object: &model.Datasource{
				Type:    model.DatasourceTypeLog,
				SubType: model.DatasourceSubTypeStorage,
			},
			wantMessage: "is not a storage datasource",
		},
		{
			name: "wrong datasource subtype",
			object: &model.Datasource{
				Type:    model.DatasourceTypeMiddleware,
				SubType: model.DatasourceSubTypeMySQL,
			},
			wantMessage: "is not a storage datasource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStorageDatasource(tt.object)
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			requireAPIError(t, err, http.StatusBadRequest, tt.wantMessage)
		})
	}
}
