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

package postgres

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
		SubType:  model.DatasourceSubTypePostgres,
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
	requireAPIError(t, err, http.StatusBadRequest, "postgres config is required")
}

func TestResolveAdhocConfigBackfillsPassword(t *testing.T) {
	object := newTestDatasource(t, types.DatasourceConfig{
		Postgres: &types.PostgresSourceConfig{
			Host:     "db.internal",
			Port:     5432,
			UserName: "app",
			Password: "saved-pg-pass",
		},
	}, true)
	factory := &fakeFactory{ds: &fakeDatasourceDAO{object: object}}
	c := New(config.Config{}, factory).(*controller)
	ctx := userContext(model.RoleRoot)

	t.Run("empty password is backfilled", func(t *testing.T) {
		got, err := c.resolveAdhocConfig(ctx, 1, &types.PostgresSourceConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Password != "saved-pg-pass" {
			t.Fatalf("expected saved password to be backfilled, got %q", got.Password)
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
		if got.Password != "saved-pg-pass" {
			t.Fatalf("expected saved password to be backfilled, got %q", got.Password)
		}
	})

	t.Run("explicit password is preserved", func(t *testing.T) {
		got, err := c.resolveAdhocConfig(ctx, 1, &types.PostgresSourceConfig{Password: "client-pass"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Password != "client-pass" {
			t.Fatalf("expected client password to be preserved, got %q", got.Password)
		}
	})
}

func TestPingAdhocRejectsNonExternalDatasource(t *testing.T) {
	object := newTestDatasource(t, types.DatasourceConfig{
		Postgres: &types.PostgresSourceConfig{Host: "db.internal", Port: 5432, UserName: "app"},
	}, false)
	factory := &fakeFactory{ds: &fakeDatasourceDAO{object: object}}
	c := New(config.Config{}, factory)

	_, err := c.PingAdhoc(userContext(model.RoleRoot), 1, &types.PostgresSourceConfig{})
	requireAPIError(t, err, http.StatusBadRequest, "postgres datasource only supports external direct connection")
}
