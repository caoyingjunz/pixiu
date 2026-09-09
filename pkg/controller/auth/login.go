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

package auth

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/loginlimit"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

// tokenIndexer 登录状态缓存
var tokenIndexer client.TokenCache

func init() {
	tokenIndexer = *client.NewTokenCache()
}

// Login 校验用户名密码并签发 token。
func (c *controller) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	// 用户名锁定后仅允许低频探测，避免多 IP 持续打满 bcrypt
	if !loginlimit.AllowUserAttempt(req.Name) {
		return nil, apierrors.ErrTooManyLoginAttempts
	}

	object, err := c.factory.User().GetUserByName(ctx, req.Name)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.ErrUserNotFound
	}

	// 如果用户已被禁用，则不允许登陆
	if object.Status == model.UserStatusForbidden {
		return nil, fmt.Errorf("用户已被禁用")
	}

	// 限制并发 bcrypt，避免刷登录打满 CPU 导致正常用户无法登录
	if !loginlimit.AcquireVerify() {
		return nil, apierrors.ErrTooManyLoginAttempts
	}
	defer loginlimit.ReleaseVerify()

	if err = util.ValidateUserPassword(object.Password, req.Password); err != nil {
		loginlimit.RecordUserFailure(req.Name)
		klog.Errorf("failed to verify user password: %v", err)
		return nil, apierrors.ErrInvalidPassword
	}
	loginlimit.ClearUserFailures(req.Name)

	return c.loginResponseForUser(object)
}

func (c *controller) loginResponseForUser(object *model.User) (*types.LoginResponse, error) {
	key := c.GetTokenKey()
	token, err := tokenutil.GenerateToken(object.Id, object.Name, object.TenantId, key)
	if err != nil {
		return nil, fmt.Errorf("生成用户 token 失败: %v", err)
	}

	if c.cc.Default.SingleLogin {
		tokenIndexer.Set(object.Id, token)
	} else {
		tokenIndexer.Add(object.Id, token)
	}
	return &types.LoginResponse{
		UserId:   object.Id,
		UserName: object.Name,
		Token:    token,
		Role:     object.Role,
	}, nil
}

// Logout 允许用户登出登陆状态。
// 无 userId 参数：当前用户由上下文取得，handler 内不再按路径参数校验归属，天然无越权面。
func (c *controller) Logout(ctx *gin.Context) error {
	curUser, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	if c.cc.Default.SingleLogin {
		tokenIndexer.Delete(curUser.Id)
		return nil
	}

	token, err := tokenutil.ExtractToken(ctx, false)
	if err != nil {
		return err
	}
	tokenIndexer.DeleteToken(curUser.Id, token)
	return nil
}

// Refresh 刷新登录 token（滑动续期）。
// TODO: 暂未实现续期逻辑。实现思路：校验当前 token 仍有效且在 tokenIndexer 中 →
// tokenutil.GenerateToken 签发新 token → 按登录模式 tokenIndexer.Set/Add 替换旧 token 并返回新 token。
func (c *controller) Refresh(ctx context.Context) error {
	return fmt.Errorf("token 刷新功能暂未实现")
}

func (c *controller) ValidateLoginToken(ctx context.Context, userId int64, token string) (bool, error) {
	if c.cc.Default.SingleLogin {
		existToken, err := c.GetLoginToken(ctx, userId)
		if err != nil {
			return false, err
		}
		return token == existToken, nil
	}

	if !tokenIndexer.Exists(userId, token) {
		return false, fmt.Errorf("invalid empty token")
	}
	return true, nil
}

func (c *controller) GetLoginToken(ctx context.Context, userId int64) (string, error) {
	t, exists := tokenIndexer.Get(userId)
	if !exists {
		return "", fmt.Errorf("invalid empty token")
	}

	return t, nil
}

func (c *controller) GetTokenKey() []byte {
	k := c.cc.Default.JWTKey
	return []byte(k)
}

// RevokeUserTokens 撤销指定用户的全部登录会话，供用户删除/重置密码后清理。
func RevokeUserTokens(userId int64) {
	tokenIndexer.Delete(userId)
}
