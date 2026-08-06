/*
Copyright 2024 The Pixiu Authors.

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

package user

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// fakeUser 通过嵌入 db.UserInterface 的方式 mock 用户 DAO，仅覆盖测试用到的方法
type fakeUser struct {
	db.UserInterface
	getFn    func(ctx context.Context, uid int64) (*model.User, error)
	updateFn func(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error
}

func (f *fakeUser) Get(ctx context.Context, uid int64) (*model.User, error) {
	if f.getFn != nil {
		return f.getFn(ctx, uid)
	}
	return nil, nil
}

func (f *fakeUser) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, uid, resourceVersion, updates)
	}
	return nil
}

// fakeFactory 通过嵌入 db.ShareDaoFactory 的方式 mock DAO 工厂
type fakeFactory struct {
	db.ShareDaoFactory
	user db.UserInterface
}

func (f *fakeFactory) User() db.UserInterface { return f.user }

// ctxWithUser 构造携带当前登录用户信息的 context
func ctxWithUser(u *model.User) context.Context {
	c := &gin.Context{}
	httputils.SetUserToContext(c, u)
	return c
}

func TestUserUpdatePrivilege(t *testing.T) {
	rv := int64(1)
	newUser := func(id int64, role model.UserLevel) *model.User {
		return &model.User{Model: pixiu.Model{Id: id}, Name: "u", Role: role}
	}

	cases := []struct {
		name      string
		curUser   *model.User
		targetUID int64
		oldUser   *model.User
		reqRole   model.UserLevel
		wantErr   error
		// wantRole 期望写入 DB 的 role（=wantUpdatesRole，nil 表示 Update 不应被调用）
		wantRole     *model.UserLevel
		wantUpCalled bool
	}{
		{
			name:      "非超管修改他人用户 → ErrForbidden",
			curUser:   newUser(1, model.RoleUser),
			targetUID: 2,
			oldUser:   newUser(2, model.RoleUser),
			reqRole:   model.RoleRoot,
			wantErr:   errors.ErrForbidden,
		},
		{
			name:      "用户不存在 → ErrUserNotFound",
			curUser:   newUser(1, model.RoleUser),
			targetUID: 99,
			oldUser:   nil,
			reqRole:   model.RoleUser,
			wantErr:   errors.ErrUserNotFound,
		},
		{
			name:         "非超管改自己且 req.Role=0(RoleRoot) → role 保持旧值",
			curUser:      newUser(1, model.RoleUser),
			targetUID:    1,
			oldUser:      newUser(1, model.RoleUser),
			reqRole:      model.RoleRoot,
			wantRole:     rolePtr(model.RoleUser),
			wantUpCalled: true,
		},
		{
			name:         "非超管改自己且 req.Role=RoleAdmin → 提权被阻止，role 保持旧值",
			curUser:      newUser(1, model.RoleUser),
			targetUID:    1,
			oldUser:      newUser(1, model.RoleUser),
			reqRole:      model.RoleAdmin,
			wantRole:     rolePtr(model.RoleUser),
			wantUpCalled: true,
		},
		{
			name:         "超管修改他人 → 可改 role",
			curUser:      newUser(1, model.RoleRoot),
			targetUID:    2,
			oldUser:      newUser(2, model.RoleUser),
			reqRole:      model.RoleAdmin,
			wantRole:     rolePtr(model.RoleAdmin),
			wantUpCalled: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upCalled := false
			var writtenRole model.UserLevel = -1
			fu := &fakeUser{
				getFn: func(_ context.Context, uid int64) (*model.User, error) {
					return tc.oldUser, nil
				},
				updateFn: func(_ context.Context, uid int64, _ int64, updates map[string]interface{}) error {
					upCalled = true
					if v, ok := updates["role"]; ok {
						writtenRole = v.(model.UserLevel)
					}
					return nil
				},
			}
			u := &user{factory: &fakeFactory{user: fu}}

			err := u.Update(ctxWithUser(tc.curUser), tc.targetUID, &types.UpdateUserRequest{
				Role:            tc.reqRole,
				Status:          model.UserStatusNormal,
				ResourceVersion: &rv,
			})

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Fatalf("期望错误 %v, 实际 %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应返回错误, 实际 %v", err)
			}
			if tc.wantUpCalled != upCalled {
				t.Fatalf("Update 调用期望 %v, 实际 %v", tc.wantUpCalled, upCalled)
			}
			if tc.wantRole != nil && writtenRole != *tc.wantRole {
				t.Fatalf("写入 role 期望 %v, 实际 %v", *tc.wantRole, writtenRole)
			}
		})
	}
}

func rolePtr(r model.UserLevel) *model.UserLevel { return &r }
