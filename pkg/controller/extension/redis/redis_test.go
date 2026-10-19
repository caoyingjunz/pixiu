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

package redis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
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

func userContext(role model.UserLevel) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	httputils.SetUserToContext(c, &model.User{Name: "tester", Role: role})
	return c
}

func newTestDatasource(t *testing.T, cfg types.DatasourceConfig, external bool) *model.Datasource {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal datasource config: %v", err)
	}
	object := &model.Datasource{
		Type:     model.DatasourceTypeMiddleware,
		SubType:  model.DatasourceSubTypeRedis,
		Config:   string(data),
		External: external,
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

func TestPingAdhocNilConfig(t *testing.T) {
	c := New(config.Config{}, nil)

	_, err := c.PingAdhoc(userContext(model.RoleRoot), 0, nil)
	requireAPIError(t, err, http.StatusBadRequest, "redis config is required")
}

func TestResolveAdhocConfigBackfillsPasswords(t *testing.T) {
	object := newTestDatasource(t, types.DatasourceConfig{
		Redis: &types.RedisSourceConfig{
			Mode:             types.RedisModeSentinel,
			Addresses:        []string{"10.0.0.1:26379"},
			MasterName:       "mymaster",
			Password:         "saved-data-pass",
			SentinelPassword: "saved-sentinel-pass",
		},
	}, true)
	factory := &fakeFactory{ds: &fakeDatasourceDAO{object: object}}
	c := New(config.Config{}, factory).(*controller)
	ctx := userContext(model.RoleRoot)

	t.Run("empty passwords are backfilled", func(t *testing.T) {
		got, err := c.resolveAdhocConfig(ctx, 1, &types.RedisSourceConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Password != "saved-data-pass" {
			t.Fatalf("expected saved data password to be backfilled, got %q", got.Password)
		}
		if got.SentinelPassword != "saved-sentinel-pass" {
			t.Fatalf("expected saved sentinel password to be backfilled, got %q", got.SentinelPassword)
		}
	})

	t.Run("nil config is initialized and backfilled", func(t *testing.T) {
		got, err := c.resolveAdhocConfig(ctx, 1, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil config")
		}
		if got.Password != "saved-data-pass" || got.SentinelPassword != "saved-sentinel-pass" {
			t.Fatalf("expected both passwords to be backfilled, got data=%q sentinel=%q",
				got.Password, got.SentinelPassword)
		}
	})

	t.Run("explicit passwords are preserved", func(t *testing.T) {
		got, err := c.resolveAdhocConfig(ctx, 1, &types.RedisSourceConfig{
			Password:         "client-data-pass",
			SentinelPassword: "client-sentinel-pass",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Password != "client-data-pass" || got.SentinelPassword != "client-sentinel-pass" {
			t.Fatalf("expected client passwords to be preserved, got data=%q sentinel=%q",
				got.Password, got.SentinelPassword)
		}
	})
}

func TestPingAdhocRejectsNonExternalDatasource(t *testing.T) {
	object := newTestDatasource(t, types.DatasourceConfig{
		Redis: &types.RedisSourceConfig{Address: "10.0.0.1:6379"},
	}, false)
	factory := &fakeFactory{ds: &fakeDatasourceDAO{object: object}}
	c := New(config.Config{}, factory)

	_, err := c.PingAdhoc(userContext(model.RoleRoot), 1, &types.RedisSourceConfig{})
	requireAPIError(t, err, http.StatusBadRequest, "redis datasource only supports external direct connection")
}
