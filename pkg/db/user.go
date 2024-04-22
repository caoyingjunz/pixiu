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

package db

import (
	"context"
	"github.com/caoyingjunz/pixiu/pkg/db/dbconn"
	"github.com/caoyingjunz/pixiu/pkg/db/mysql"
	"github.com/caoyingjunz/pixiu/pkg/db/sqlite"
	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type UserInterface interface {
	Create(ctx context.Context, object *model.User) (*model.User, error)
	Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context) ([]model.User, error)

	Count(ctx context.Context) (int64, error)

	GetUserByName(ctx context.Context, userName string) (*model.User, error)
}

func newUser(db *dbconn.DbConn) UserInterface {
	switch db.Type {
	case "mysql":
		return mysql.NewUser(db.Conn.(*gorm.DB))
	case "sqlite":
		return sqlite.NewUser(db.Conn.(*gorm.DB))
	}
	return nil
}
