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

package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type fakeDatasourceDAO struct {
	object *model.Datasource
	err    error
}

func (f *fakeDatasourceDAO) Create(context.Context, *model.Datasource) (*model.Datasource, error) {
	return nil, nil
}

func (f *fakeDatasourceDAO) Update(context.Context, int64, int64, map[string]interface{}) error {
	return nil
}

func (f *fakeDatasourceDAO) Delete(context.Context, int64) error { return nil }

func (f *fakeDatasourceDAO) Get(context.Context, int64) (*model.Datasource, error) {
	return f.object, f.err
}

func (f *fakeDatasourceDAO) List(context.Context, ...db.Options) ([]model.Datasource, error) {
	return nil, nil
}

func (f *fakeDatasourceDAO) Count(context.Context, ...db.Options) (int64, error) { return 0, nil }

type fakeFactory struct {
	db.ShareDaoFactory
	ds db.DatasourceInterface
}

func (f *fakeFactory) Datasource() db.DatasourceInterface { return f.ds }

// rootContext 构造带 root 用户的 gin 上下文：root 会跳过 Role/Scope 校验，
// 测试因此不需要伪造 Role DAO，也不会触达真实数据库。
func rootContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	httputils.SetUserToContext(c, &model.User{Name: "root", Role: model.RoleRoot})
	return c
}

func newMySQLDatasource(t *testing.T, cfg types.DatasourceConfig) *model.Datasource {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal datasource config: %v", err)
	}
	object := &model.Datasource{
		Type:     model.DatasourceTypeMiddleware,
		SubType:  model.DatasourceSubTypeMySQL,
		Config:   string(data),
		External: true,
		UserId:   1,
	}
	object.Id = 1
	return object
}

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

func TestLoadDatasourceConfig(t *testing.T) {
	validObject := newMySQLDatasource(t, types.DatasourceConfig{
		Mysql: &types.MySQLSourceConfig{Host: "127.0.0.1", Port: 3306, UserName: "root"},
	})
	invalidObject := newMySQLDatasource(t, types.DatasourceConfig{})
	invalidObject.Config = "{not-json"
	validateErr := apierrors.NewError(fmt.Errorf("custom validate failed"), http.StatusBadRequest)
	daoErr := fmt.Errorf("connection reset")

	tests := []struct {
		name        string
		object      *model.Datasource
		getErr      error
		validate    func(*model.Datasource) error
		wantCode    int
		wantMessage string
		wantConfig  bool
	}{
		{
			name:       "success without validate",
			object:     validObject,
			wantConfig: true,
		},
		{
			name:       "success with validate",
			object:     validObject,
			validate:   func(*model.Datasource) error { return nil },
			wantConfig: true,
		},
		{
			name:     "datasource not found",
			object:   nil,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "dao error",
			object:   validObject,
			getErr:   daoErr,
			wantCode: http.StatusInternalServerError,
		},
		{
			name:        "validate error is returned unchanged",
			object:      validObject,
			validate:    func(*model.Datasource) error { return validateErr },
			wantCode:    http.StatusBadRequest,
			wantMessage: "custom validate failed",
		},
		{
			name:     "invalid config json returns 500",
			object:   invalidObject,
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &fakeFactory{ds: &fakeDatasourceDAO{object: tt.object, err: tt.getErr}}
			object, cfg, err := LoadDatasourceConfig(rootContext(), factory, 1, tt.validate)

			if tt.wantCode != 0 {
				requireAPIError(t, err, tt.wantCode, tt.wantMessage)
				if object != nil || cfg != nil {
					t.Fatalf("expected nil object and config on error, got object=%v config=%v", object, cfg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if object != tt.object {
				t.Fatalf("expected returned object %p, got %p", tt.object, object)
			}
			if cfg == nil {
				t.Fatal("expected non-nil datasource config")
			}
			if tt.wantConfig && cfg.Mysql == nil {
				t.Fatal("expected mysql config to be decoded")
			}
		})
	}
}

func TestLoadDatasourceConfigForAdhoc(t *testing.T) {
	validObject := newMySQLDatasource(t, types.DatasourceConfig{
		Mysql: &types.MySQLSourceConfig{Host: "127.0.0.1", Port: 3306, UserName: "root"},
	})
	invalidObject := newMySQLDatasource(t, types.DatasourceConfig{})
	invalidObject.Config = "{not-json"

	tests := []struct {
		name        string
		object      *model.Datasource
		kind        string
		wantCode    int
		wantMessage string
		wantConfig  bool
	}{
		{
			name:       "success",
			object:     validObject,
			kind:       "MySQL",
			wantConfig: true,
		},
		{
			name:        "invalid config json returns 400 with kind",
			object:      invalidObject,
			kind:        "MySQL",
			wantCode:    http.StatusBadRequest,
			wantMessage: "invalid MySQL datasource config",
		},
		{
			name:        "invalid config json uses caller kind",
			object:      invalidObject,
			kind:        "PostgreSQL",
			wantCode:    http.StatusBadRequest,
			wantMessage: "invalid PostgreSQL datasource config",
		},
		{
			name:     "datasource not found",
			object:   nil,
			kind:     "MySQL",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &fakeFactory{ds: &fakeDatasourceDAO{object: tt.object}}
			object, cfg, err := LoadDatasourceConfigForAdhoc(rootContext(), factory, 1, nil, tt.kind)

			if tt.wantCode != 0 {
				requireAPIError(t, err, tt.wantCode, tt.wantMessage)
				if object != nil || cfg != nil {
					t.Fatalf("expected nil object and config on error, got object=%v config=%v", object, cfg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg == nil || cfg.Mysql == nil {
				t.Fatal("expected decoded mysql config")
			}
		})
	}
}
