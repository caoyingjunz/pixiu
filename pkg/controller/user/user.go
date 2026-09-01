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

package user

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	rbacaccess "github.com/caoyingjunz/pixiu/pkg/rbac/access"
	rbacapi "github.com/caoyingjunz/pixiu/pkg/rbac/api"
	menupkg "github.com/caoyingjunz/pixiu/pkg/rbac/menu"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/loginlimit"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

var (
	userIndexer  client.UserCache
	tokenIndexer client.TokenCache
)

func init() {
	userIndexer = *client.NewUserCache()
	tokenIndexer = *client.NewTokenCache()
}

type UserGetter interface {
	User() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateUserRequest) error
	Update(ctx context.Context, userId int64, req *types.UpdateUserRequest) error
	Delete(ctx context.Context, userId int64) error
	Get(ctx context.Context, userId int64) (*types.User, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)

	// UpdatePassword 用户修改密码或者管理员重置密码
	UpdatePassword(ctx context.Context, userId int64, req *types.UpdateUserPasswordRequest) error
	// GetCount 仅获取用户数量
	GetCount(ctx context.Context, opts types.ListOptions) (int64, error)
	// GetStatus 获取用户状态，优先从缓存获取，如果没有则从库里获取，然后同步到缓存
	GetStatus(ctx context.Context, uid int64) (int, error)

	Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error)
	Logout(ctx *gin.Context, userId int64) error

	GetCurrentUserPermissions(ctx context.Context) (*types.CurrentUserPermissionsResponse, error)

	GetLoginToken(ctx context.Context, userId int64) (string, error)
	ValidAccess(ctx *gin.Context, roleId int64) error
	ValidateLoginToken(ctx context.Context, userId int64, token string) (bool, error)

	ListOAuthProviders(ctx context.Context, enabledOnly bool) ([]*types.OAuthProviderSummary, error)
	GetOAuthProviderConfig(ctx context.Context, provider string) (*types.OAuthProviderConfig, error)
	UpdateOAuthProviderConfig(ctx context.Context, provider string, req *types.UpdateOAuthProviderConfigRequest) (*types.OAuthProviderConfig, error)
	GetOAuthProviderLoginURL(ctx context.Context, provider string) (*types.OAuthLoginURLResponse, error)
	LoginWithOAuthProvider(ctx context.Context, provider string, req *types.OAuthLoginRequest) (*types.LoginResponse, error)
}

type user struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (u *user) Create(ctx context.Context, req *types.CreateUserRequest) error {
	// 仅超级管理员可创建用户
	if err := controllerutil.CheckRoot(ctx); err != nil {
		return err
	}

	encrypt, err := util.EncryptUserPassword(req.Password)
	if err != nil {
		klog.Errorf("failed to encrypt user password: %v", err)
		return errors.ErrServerInternal
	}

	object, err := u.factory.User().GetUserByName(ctx, req.Name)
	if err != nil {
		klog.Errorf("failed to get user %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	if object != nil {
		err = errors.ErrUserExists // 记录错误
		return err
	}

	// 超级管理员全局只允许存在一个
	if req.Role == model.RoleRoot {
		root, err := u.factory.User().GetRoot(ctx)
		if err != nil {
			klog.Errorf("failed to check root user: %v", err)
			return errors.ErrServerInternal
		}
		if root != nil {
			return errors.ErrRootAlreadyExists
		}
	}

	if _, err = u.factory.User().Create(ctx, &model.User{
		Name:        req.Name,
		Password:    encrypt,
		Status:      req.Status,
		Role:        req.Role,
		Email:       req.Email,
		Phone:       req.Phone,
		Description: req.Description,
	}); err != nil {
		klog.Errorf("failed to create user %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

// 更新前置检查：资源存在 + 非超管只能更新自己的用户
func (u *user) preUpdate(ctx context.Context, uid int64) (*model.User, *model.User, error) {
	curUser, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	object, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", uid, err)
		return nil, nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, nil, errors.ErrUserNotFound
	}
	// 非超级管理员只能更新自己的用户（等价于 CheckResourceOwner：Root 放行 / Id 相等放行）
	if curUser.Role != model.RoleRoot && curUser.Id != uid {
		return nil, nil, errors.ErrForbidden
	}
	return curUser, object, nil
}

func (u *user) Update(ctx context.Context, uid int64, req *types.UpdateUserRequest) error {
	curUser, old, err := u.preUpdate(ctx, uid)
	if err != nil {
		klog.Errorf("pre-update check failed for user(%d): %v", uid, err)
		return err
	}

	updates := map[string]interface{}{
		"status":      req.Status,
		"email":       req.Email,
		"phone":       req.Phone,
		"description": req.Description,
	}
	// 非超管不允许修改角色（垂直越权防护）；req.Role 是值类型，前端不传时零值=RoleRoot(0)，故非超管一律强制保持旧角色
	if curUser.Role == model.RoleRoot {
		updates["role"] = req.Role
	} else {
		updates["role"] = old.Role
	}

	if err = u.factory.User().Update(ctx, uid, req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update user(%d): %v", uid, err)
		return errors.ErrServerInternal
	}

	userIndexer.Set(uid, int(req.Status))
	return nil
}

func (u *user) preResetPassword(ctx context.Context, userId int64, operatorId int64, req *types.UpdateUserPasswordRequest) error {
	// 操作人必须具备管理员权限
	operator, err := u.Get(ctx, operatorId)
	if err != nil {
		return err
	}

	if operator.Role != model.RoleRoot {
		return fmt.Errorf("非超级管理员，不允许重置用户密码")
	}
	return nil
}

func (u *user) preChangePassword(ctx context.Context, userId int64, operatorId int64, req *types.UpdateUserPasswordRequest) error {
	if operatorId != userId {
		return fmt.Errorf("用户只能修改自己的密码")
	}
	if len(req.Old) == 0 {
		return fmt.Errorf("当前密码不能为空")
	}
	// 须从 DB 读取 model.User：u.Get 经 model2Type 转换时会省略 Password，导致校验永远失败
	object, err := u.factory.User().Get(ctx, userId)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", userId, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrUserNotFound
	}

	// 校验旧密码是否正确
	if err = util.ValidateUserPassword(object.Password, req.Old); err != nil {
		klog.Errorf("failed to verify user password: %v", err)
		return errors.ErrInvalidPassword
	}
	return nil
}

// UpdatePassword 支持用户修改密码和管理员重置密码
// 修改密码: 用户只能修改自己密码
// 重启密码: 管理员可以重置他人密码
func (u *user) UpdatePassword(ctx context.Context, userId int64, req *types.UpdateUserPasswordRequest) error {
	// 新老密码不允许相同
	if req.New == req.Old {
		return errors.ErrDuplicatedPassword
	}

	operatorId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return err
	}

	if req.Reset {
		// 管理员重置密码前置检查
		if err = u.preResetPassword(ctx, userId, operatorId, req); err != nil {
			return err
		}
	} else {
		// 用户修改密码前置检查
		if err = u.preChangePassword(ctx, userId, operatorId, req); err != nil {
			return err
		}
	}

	newPass, err := util.EncryptUserPassword(req.New)
	if err != nil {
		klog.Errorf("failed to encrypt user password: %v", err)
		return errors.ErrServerInternal
	}
	if err = u.factory.User().Update(ctx, userId, *req.ResourceVersion, map[string]interface{}{
		"password": newPass,
	}); err != nil {
		klog.Errorf("failed to update user(%d) password: %v", userId, err)
		return errors.ErrServerInternal
	}

	tokenIndexer.Delete(userId)
	return nil
}

func (u *user) Delete(ctx context.Context, userId int64) error {
	object, err := u.factory.User().Get(ctx, userId)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", userId, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrUserNotFound
	}
	// 非超级管理员只能删除自己
	if err := controllerutil.CheckResourceOwner(ctx, userId); err != nil {
		return err
	}
	if err := u.factory.User().Delete(ctx, userId); err != nil {
		klog.Errorf("failed to delete user(%d): %v", userId, err)
		return errors.ErrServerInternal
	}

	userIndexer.Delete(userId)
	tokenIndexer.Delete(userId)
	return nil
}

func (u *user) Get(ctx context.Context, userId int64) (*types.User, error) {
	curUser, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// 非管理员仅可读自身，防止水平用户枚举 / 详情泄露
	if curUser.Id != userId {
		if err = controllerutil.CheckAdmin(ctx); err != nil {
			return nil, err
		}
	}

	object, err := u.factory.User().Get(ctx, userId)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", userId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrUserNotFound
	}

	return model2Type(object), nil
}

func (u *user) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	curUser, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithOrderByDesc(),
		db.WithNameLike(listOption.NameSelector),
		db.WithPhoneLike(listOption.UserPhone),
		db.WithEmailLike(listOption.UserEmail),
	}
	if listOption.Status != nil {
		opts = append(opts, db.WithUserStatus(*listOption.Status))
	}
	// 非管理员列表强制收敛到自身，避免枚举其他账号
	if err = controllerutil.CheckAdmin(ctx); err != nil {
		opts = append(opts, db.WithId(curUser.Id))
	}

	pageResult.Total, err = u.factory.User().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get user counts: %v", err)
		return nil, errors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithOffset(offset), db.WithLimit(listOption.Limit))

	objects, err := u.factory.User().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get user list: %v", err)
		return nil, errors.ErrServerInternal
	}

	users := make([]types.User, 0)
	for _, object := range objects {
		users = append(users, *model2Type(&object))
	}
	pageResult.Items = users

	return pageResult, nil
}

func (u *user) GetCount(ctx context.Context, opts types.ListOptions) (int64, error) {
	curUser, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return 0, err
	}
	countOpts := []db.Options{}
	if err = controllerutil.CheckAdmin(ctx); err != nil {
		countOpts = append(countOpts, db.WithId(curUser.Id))
	}
	userCount, err := u.factory.User().Count(ctx, countOpts...)
	if err != nil {
		klog.Errorf("failed to get user counts: %v", err)
		return 0, errors.ErrServerInternal
	}

	return userCount, nil
}

// GetStatus 获取用户状态，优先从缓存获取，如果没有则从库里获取，然后同步到缓存
func (u *user) GetStatus(ctx context.Context, uid int64) (int, error) {
	status, ok := userIndexer.Get(uid)
	if ok {
		return status, nil
	}

	object, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", uid, err)
		return 0, errors.ErrServerInternal
	}
	if object == nil {
		return 0, errors.ErrUserNotFound
	}

	userIndexer.Set(uid, int(object.Status))
	return int(object.Status), nil
}

func (u *user) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	// 用户名锁定后仅允许低频探测，避免多 IP 持续打满 bcrypt
	if !loginlimit.AllowUserAttempt(req.Name) {
		return nil, errors.ErrTooManyLoginAttempts
	}

	object, err := u.factory.User().GetUserByName(ctx, req.Name)
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrUserNotFound
	}

	// 如果用户已被禁用，则不允许登陆
	if object.Status == model.UserStatusForbidden {
		return nil, fmt.Errorf("用户已被禁用")
	}

	// 限制并发 bcrypt，避免刷登录打满 CPU 导致正常用户无法登录
	if !loginlimit.AcquireVerify() {
		return nil, errors.ErrTooManyLoginAttempts
	}
	defer loginlimit.ReleaseVerify()

	if err = util.ValidateUserPassword(object.Password, req.Password); err != nil {
		loginlimit.RecordUserFailure(req.Name)
		klog.Errorf("failed to verify user password: %v", err)
		return nil, errors.ErrInvalidPassword
	}
	loginlimit.ClearUserFailures(req.Name)

	// 生成登陆的 token 信息
	key := u.GetTokenKey()
	token, err := tokenutil.GenerateToken(object.Id, object.Name, object.TenantId, key)
	if err != nil {
		return nil, fmt.Errorf("生成用户 token 失败: %v", err)
	}

	if u.cc.Default.SingleLogin {
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

const feishuProvider = "feishu"
const oauthStateTTL = 5 * time.Minute

type oauthProviderSpec struct {
	Provider   string
	Name       string
	LoginType  string
	ButtonText string
}

var oauthProviderSpecs = map[string]oauthProviderSpec{
	feishuProvider: {
		Provider:   feishuProvider,
		Name:       "飞书",
		LoginType:  "redirect",
		ButtonText: "飞书扫码登录",
	},
	"wechat_work": {
		Provider:   "wechat_work",
		Name:       "企业微信",
		LoginType:  "redirect",
		ButtonText: "企业微信登录",
	},
	"dingtalk": {
		Provider:   "dingtalk",
		Name:       "钉钉",
		LoginType:  "redirect",
		ButtonText: "钉钉登录",
	},
	"ldap": {
		Provider:   "ldap",
		Name:       "LDAP",
		LoginType:  "password",
		ButtonText: "LDAP 登录",
	},
}

var oauthProviderOrder = []string{feishuProvider, "wechat_work", "dingtalk", "ldap"}

type oauthProviderClient interface {
	LoginURL(cfg *model.OAuthProvider, state string) (string, error)
	ExchangeUser(ctx context.Context, cfg *model.OAuthProvider, code string) (*oauthUserProfile, error)
}

var oauthProviderClients = map[string]oauthProviderClient{
	feishuProvider: feishuOAuthClient{},
}

type oauthUserProfile struct {
	Provider        string
	Name            string
	AvatarURL       string
	OpenID          string
	UnionID         string
	Email           string
	EnterpriseEmail string
	UserID          string
	Mobile          string
}

type feishuOAuthClient struct{}

type feishuAppTokenResponse struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	AppAccessToken string `json:"app_access_token"`
	Expire         int    `json:"expire"`
}

type feishuAccessTokenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccessToken     string `json:"access_token"`
		Name            string `json:"name"`
		AvatarURL       string `json:"avatar_url"`
		OpenID          string `json:"open_id"`
		UnionID         string `json:"union_id"`
		Email           string `json:"email"`
		EnterpriseEmail string `json:"enterprise_email"`
		UserID          string `json:"user_id"`
		Mobile          string `json:"mobile"`
	} `json:"data"`
}

type feishuUserInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Name            string `json:"name"`
		AvatarURL       string `json:"avatar_url"`
		OpenID          string `json:"open_id"`
		UnionID         string `json:"union_id"`
		Email           string `json:"email"`
		EnterpriseEmail string `json:"enterprise_email"`
		UserID          string `json:"user_id"`
		Mobile          string `json:"mobile"`
	} `json:"data"`
}

func (u *user) ListOAuthProviders(ctx context.Context, enabledOnly bool) ([]*types.OAuthProviderSummary, error) {
	if !enabledOnly {
		if err := controllerutil.CheckRoot(ctx); err != nil {
			return nil, err
		}
	}
	objects, err := u.factory.OAuthProvider().List(ctx)
	if err != nil {
		klog.Errorf("failed to list oauth providers: %v", err)
		return nil, errors.ErrServerInternal
	}
	saved := make(map[string]*model.OAuthProvider, len(objects))
	for _, object := range objects {
		saved[object.Provider] = object
	}

	providers := make([]*types.OAuthProviderSummary, 0, len(oauthProviderOrder))
	for _, provider := range oauthProviderOrder {
		spec := oauthProviderSpecs[provider]
		object := saved[spec.Provider]
		enabled := object != nil && object.Enabled
		if enabledOnly && !enabled {
			continue
		}
		providers = append(providers, &types.OAuthProviderSummary{
			Provider:   spec.Provider,
			Name:       firstNonEmpty(providerName(object), spec.Name),
			LoginType:  firstNonEmpty(providerLoginType(object), spec.LoginType),
			ButtonText: providerButtonText(spec, object),
			Enabled:    enabled,
		})
	}
	return providers, nil
}

func (u *user) GetOAuthProviderConfig(ctx context.Context, provider string) (*types.OAuthProviderConfig, error) {
	if err := controllerutil.CheckRoot(ctx); err != nil {
		return nil, err
	}
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	cfg, err := u.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, errors.ErrServerInternal
	}
	return oauthProvider2Type(spec, cfg), nil
}

func (u *user) UpdateOAuthProviderConfig(ctx context.Context, provider string, req *types.UpdateOAuthProviderConfigRequest) (*types.OAuthProviderConfig, error) {
	if err := controllerutil.CheckRoot(ctx); err != nil {
		return nil, err
	}
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	old, err := u.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, errors.ErrServerInternal
	}
	appSecret := strings.TrimSpace(req.AppSecret)
	if appSecret == "" && old != nil {
		appSecret = old.AppSecret
	}
	defaultRole := req.DefaultRole
	if defaultRole != model.RoleAdmin && defaultRole != model.RoleUser {
		defaultRole = model.RoleUser
	}
	if req.Enabled && (strings.TrimSpace(req.AppID) == "" || appSecret == "" || strings.TrimSpace(req.RedirectURI) == "") {
		return nil, fmt.Errorf("启用%s登录时 App ID、App Secret、Redirect URL 不能为空", spec.Name)
	}

	saved, err := u.factory.OAuthProvider().Save(ctx, &model.OAuthProvider{
		Provider:       spec.Provider,
		Name:           firstNonEmpty(strings.TrimSpace(req.Name), spec.Name),
		LoginType:      firstNonEmpty(strings.TrimSpace(req.LoginType), spec.LoginType),
		Enabled:        req.Enabled,
		AppID:          strings.TrimSpace(req.AppID),
		AppSecret:      appSecret,
		RedirectURI:    strings.TrimSpace(req.RedirectURI),
		Scopes:         strings.TrimSpace(req.Scopes),
		ConfigJSON:     strings.TrimSpace(req.ConfigJSON),
		AutoCreateUser: req.AutoCreateUser,
		DefaultRole:    defaultRole,
		MatchEmail:     req.MatchEmail,
		Description:    strings.TrimSpace(req.Description),
	})
	if err != nil {
		klog.Errorf("failed to save oauth provider(%s) config: %v", spec.Provider, err)
		return nil, errors.ErrServerInternal
	}
	return oauthProvider2Type(spec, saved), nil
}

func (u *user) GetOAuthProviderLoginURL(ctx context.Context, provider string) (*types.OAuthLoginURLResponse, error) {
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	cfg, err := u.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, errors.ErrServerInternal
	}
	if cfg == nil || !cfg.Enabled {
		return &types.OAuthLoginURLResponse{Provider: spec.Provider, Enabled: false}, nil
	}
	if cfg.AppID == "" || cfg.RedirectURI == "" {
		return nil, fmt.Errorf("%s登录未完成配置", spec.Name)
	}
	providerClient, ok := oauthProviderClients[spec.Provider]
	if !ok {
		return nil, fmt.Errorf("%s登录暂未实现", spec.Name)
	}
	state := u.oauthState(spec.Provider)
	loginURL, err := providerClient.LoginURL(cfg, state)
	if err != nil {
		return nil, err
	}
	return &types.OAuthLoginURLResponse{
		Provider: spec.Provider,
		Enabled:  true,
		URL:      loginURL,
		State:    state,
	}, nil
}

func (u *user) LoginWithOAuthProvider(ctx context.Context, provider string, req *types.OAuthLoginRequest) (*types.LoginResponse, error) {
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	cfg, err := u.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, errors.ErrServerInternal
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("%s登录未启用", spec.Name)
	}
	if !u.validateOAuthState(spec.Provider, req.State) {
		return nil, fmt.Errorf("第三方登录状态校验失败，请重新登录")
	}
	providerClient, ok := oauthProviderClients[spec.Provider]
	if !ok {
		return nil, fmt.Errorf("%s登录暂未实现", spec.Name)
	}
	profile, err := providerClient.ExchangeUser(ctx, cfg, req.Code)
	if err != nil {
		klog.Errorf("failed to login with oauth provider(%s): %v", spec.Provider, err)
		return nil, err
	}
	object, err := u.findOrCreateOAuthUser(ctx, cfg, profile)
	if err != nil {
		return nil, err
	}
	if object.Status == model.UserStatusForbidden {
		return nil, fmt.Errorf("用户已被禁用")
	}
	return u.loginResponseForUser(object)
}

func (u *user) getOAuthProvider(ctx context.Context, provider string) (*model.OAuthProvider, error) {
	return u.factory.OAuthProvider().GetByProvider(ctx, provider)
}

func getOAuthProviderSpec(provider string) (oauthProviderSpec, error) {
	spec, ok := oauthProviderSpecs[strings.TrimSpace(provider)]
	if !ok {
		return oauthProviderSpec{}, fmt.Errorf("不支持的登录源: %s", provider)
	}
	return spec, nil
}

func oauthProvider2Type(spec oauthProviderSpec, o *model.OAuthProvider) *types.OAuthProviderConfig {
	if o == nil {
		return &types.OAuthProviderConfig{
			Provider:       spec.Provider,
			Name:           spec.Name,
			LoginType:      spec.LoginType,
			ButtonText:     spec.ButtonText,
			AutoCreateUser: true,
			DefaultRole:    model.RoleUser,
			MatchEmail:     true,
		}
	}
	return &types.OAuthProviderConfig{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Provider:       spec.Provider,
		Name:           firstNonEmpty(providerName(o), spec.Name),
		LoginType:      firstNonEmpty(providerLoginType(o), spec.LoginType),
		ButtonText:     providerButtonText(spec, o),
		Enabled:        o.Enabled,
		AppID:          o.AppID,
		AppSecretSet:   o.AppSecret != "",
		RedirectURI:    o.RedirectURI,
		Scopes:         o.Scopes,
		ConfigJSON:     o.ConfigJSON,
		AutoCreateUser: o.AutoCreateUser,
		DefaultRole:    o.DefaultRole,
		MatchEmail:     o.MatchEmail,
		Description:    o.Description,
	}
}

func providerName(o *model.OAuthProvider) string {
	if o == nil {
		return ""
	}
	return o.Name
}

func providerLoginType(o *model.OAuthProvider) string {
	if o == nil {
		return ""
	}
	return o.LoginType
}

func providerButtonText(spec oauthProviderSpec, o *model.OAuthProvider) string {
	if o == nil || o.Name == "" {
		return spec.ButtonText
	}
	if o.Name == spec.Name {
		return spec.ButtonText
	}
	if o.LoginType == "redirect" {
		return o.Name + "登录"
	}
	return o.Name + " 登录"
}

func (feishuOAuthClient) LoginURL(cfg *model.OAuthProvider, state string) (string, error) {
	values := url.Values{}
	values.Set("app_id", cfg.AppID)
	values.Set("redirect_uri", cfg.RedirectURI)
	values.Set("state", state)
	return "https://open.feishu.cn/open-apis/authen/v1/index?" + values.Encode(), nil
}

func (feishuOAuthClient) ExchangeUser(ctx context.Context, cfg *model.OAuthProvider, code string) (*oauthUserProfile, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	appToken, err := requestFeishuAppAccessToken(ctx, client, cfg)
	if err != nil {
		return nil, err
	}
	tokenResp, err := requestFeishuUserAccessToken(ctx, client, appToken, code)
	if err != nil {
		return nil, err
	}
	info, err := requestFeishuUserInfo(ctx, client, tokenResp.Data.AccessToken)
	if err != nil {
		return nil, err
	}
	profile := &oauthUserProfile{
		Provider:        feishuProvider,
		Name:            firstNonEmpty(info.Data.Name, tokenResp.Data.Name),
		AvatarURL:       firstNonEmpty(info.Data.AvatarURL, tokenResp.Data.AvatarURL),
		OpenID:          firstNonEmpty(info.Data.OpenID, tokenResp.Data.OpenID),
		UnionID:         firstNonEmpty(info.Data.UnionID, tokenResp.Data.UnionID),
		Email:           firstNonEmpty(info.Data.Email, tokenResp.Data.Email),
		EnterpriseEmail: firstNonEmpty(info.Data.EnterpriseEmail, tokenResp.Data.EnterpriseEmail),
		UserID:          firstNonEmpty(info.Data.UserID, tokenResp.Data.UserID),
		Mobile:          firstNonEmpty(info.Data.Mobile, tokenResp.Data.Mobile),
	}
	if profile.OpenID == "" && profile.UnionID == "" {
		return nil, fmt.Errorf("第三方登录未返回可绑定的用户标识")
	}
	return profile, nil
}

func requestFeishuAppAccessToken(ctx context.Context, client *http.Client, cfg *model.OAuthProvider) (string, error) {
	body := map[string]string{"app_id": cfg.AppID, "app_secret": cfg.AppSecret}
	var out feishuAppTokenResponse
	if err := postFeishuJSON(ctx, client, "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal", "", body, &out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.AppAccessToken == "" {
		return "", fmt.Errorf("获取飞书 app_access_token 失败: %s", firstNonEmpty(out.Msg, fmt.Sprintf("code=%d", out.Code)))
	}
	return out.AppAccessToken, nil
}

func requestFeishuUserAccessToken(ctx context.Context, client *http.Client, appToken, code string) (*feishuAccessTokenResponse, error) {
	body := map[string]string{"grant_type": "authorization_code", "code": code}
	var out feishuAccessTokenResponse
	if err := postFeishuJSON(ctx, client, "https://open.feishu.cn/open-apis/authen/v1/access_token", appToken, body, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 || out.Data.AccessToken == "" {
		return nil, fmt.Errorf("获取飞书 user_access_token 失败: %s", firstNonEmpty(out.Msg, fmt.Sprintf("code=%d", out.Code)))
	}
	return &out, nil
}

func requestFeishuUserInfo(ctx context.Context, client *http.Client, userToken string) (*feishuUserInfoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://open.feishu.cn/open-apis/authen/v1/user_info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("获取飞书用户信息失败: status=%d", resp.StatusCode)
	}
	var out feishuUserInfoResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("获取飞书用户信息失败: %s", firstNonEmpty(out.Msg, fmt.Sprintf("code=%d", out.Code)))
	}
	return &out, nil
}

func postFeishuJSON(ctx context.Context, client *http.Client, endpoint, bearer string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("飞书接口请求失败: status=%d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

func (u *user) findOrCreateOAuthUser(ctx context.Context, cfg *model.OAuthProvider, profile *oauthUserProfile) (*model.User, error) {
	provider := firstNonEmpty(profile.Provider, cfg.Provider)
	var object *model.User
	var err error
	if profile.UnionID != "" {
		object, err = u.factory.User().GetBy(ctx, db.WithOAuthUnionID(provider, profile.UnionID))
		if err != nil {
			return nil, errors.ErrServerInternal
		}
	}
	if object == nil && profile.OpenID != "" {
		object, err = u.factory.User().GetBy(ctx, db.WithOAuthOpenID(provider, profile.OpenID))
		if err != nil {
			return nil, errors.ErrServerInternal
		}
	}
	email := firstNonEmpty(profile.EnterpriseEmail, profile.Email)
	if object == nil && cfg.MatchEmail && email != "" {
		object, err = u.factory.User().GetBy(ctx, db.WithEmail(email))
		if err != nil {
			return nil, errors.ErrServerInternal
		}
	}
	if object != nil {
		return u.bindOAuthProfile(ctx, object, provider, profile)
	}
	if !cfg.AutoCreateUser {
		return nil, fmt.Errorf("第三方账号未绑定 Pixiu 用户")
	}
	return u.createOAuthUser(ctx, cfg, provider, profile, email)
}

func (u *user) bindOAuthProfile(ctx context.Context, object *model.User, provider string, profile *oauthUserProfile) (*model.User, error) {
	if object.OAuthProvider != "" && object.OAuthProvider != provider {
		return nil, fmt.Errorf("该 Pixiu 用户已绑定其他登录源")
	}
	if object.OAuthOpenID != "" && profile.OpenID != "" && object.OAuthOpenID != profile.OpenID {
		return nil, fmt.Errorf("该 Pixiu 用户已绑定其他第三方账号")
	}
	if object.OAuthUnionID != "" && profile.UnionID != "" && object.OAuthUnionID != profile.UnionID {
		return nil, fmt.Errorf("该 Pixiu 用户已绑定其他第三方账号")
	}
	updates := map[string]interface{}{}
	if object.OAuthProvider == "" {
		updates["oauth_provider"] = provider
	}
	if object.OAuthOpenID == "" && profile.OpenID != "" {
		updates["oauth_open_id"] = profile.OpenID
	}
	if object.OAuthUnionID == "" && profile.UnionID != "" {
		updates["oauth_union_id"] = profile.UnionID
	}
	if object.OAuthUserID == "" && profile.UserID != "" {
		updates["oauth_user_id"] = profile.UserID
	}
	if profile.AvatarURL != "" && object.AvatarURL != profile.AvatarURL {
		updates["avatar_url"] = profile.AvatarURL
	}
	if len(updates) == 0 {
		return object, nil
	}
	if err := u.factory.User().Update(ctx, object.Id, object.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to bind oauth user(%d, %s): %v", object.Id, provider, err)
		return nil, errors.ErrServerInternal
	}
	return u.factory.User().Get(ctx, object.Id)
}

func (u *user) createOAuthUser(ctx context.Context, cfg *model.OAuthProvider, provider string, profile *oauthUserProfile, email string) (*model.User, error) {
	password, err := util.EncryptUserPassword(randomHex(24))
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	name := u.uniqueOAuthUserName(ctx, firstNonEmpty(profile.Name, email, profile.UserID, profile.OpenID, provider+"-user"))
	object, err := u.factory.User().Create(ctx, &model.User{
		Name:          name,
		Password:      password,
		Status:        model.UserStatusNormal,
		Role:          cfg.DefaultRole,
		Email:         email,
		Phone:         profile.Mobile,
		OAuthProvider: provider,
		OAuthOpenID:   profile.OpenID,
		OAuthUnionID:  profile.UnionID,
		OAuthUserID:   profile.UserID,
		AvatarURL:     profile.AvatarURL,
		Description:   "第三方登录自动创建",
	})
	if err != nil {
		klog.Errorf("failed to create oauth user(%s): %v", provider, err)
		return nil, errors.ErrServerInternal
	}
	return object, nil
}

func (u *user) uniqueOAuthUserName(ctx context.Context, base string) string {
	base = strings.TrimSpace(base)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		base = "oauth-user"
	}
	base = truncateRunes(base, 48)
	if base == "" {
		base = "oauth-user"
	}
	name := base
	for i := 0; i < 20; i++ {
		existing, err := u.factory.User().GetUserByName(ctx, name)
		if err != nil || existing == nil {
			return name
		}
		name = fmt.Sprintf("%s-%02d", base, i+1)
	}
	return fmt.Sprintf("%s-%s", base, randomHex(4))
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

func (u *user) loginResponseForUser(object *model.User) (*types.LoginResponse, error) {
	key := u.GetTokenKey()
	token, err := tokenutil.GenerateToken(object.Id, object.Name, object.TenantId, key)
	if err != nil {
		return nil, fmt.Errorf("生成用户 token 失败: %v", err)
	}
	if u.cc.Default.SingleLogin {
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

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func (u *user) oauthState(provider string) string {
	issuedAt := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randomHex(16)
	payload := strings.Join([]string{provider, issuedAt, nonce}, ":")
	signature := u.signOAuthState(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + signature))
}

func (u *user) validateOAuthState(provider, state string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 4 || parts[0] != provider {
		return false
	}
	issuedAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}
	issuedTime := time.Unix(issuedAt, 0)
	if time.Since(issuedTime) < 0 || time.Since(issuedTime) > oauthStateTTL {
		return false
	}
	payload := strings.Join(parts[:3], ":")
	expected := u.signOAuthState(payload)
	return hmac.Equal([]byte(parts[3]), []byte(expected))
}

func (u *user) signOAuthState(payload string) string {
	mac := hmac.New(sha256.New, u.GetTokenKey())
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Logout
// 允许用户登出登陆状态
func (u *user) Logout(ctx *gin.Context, userId int64) error {
	if err := controllerutil.CheckResourceOwner(ctx, userId); err != nil {
		return err
	}
	if u.cc.Default.SingleLogin {
		tokenIndexer.Delete(userId)
		return nil
	}

	token, err := tokenutil.ExtractToken(ctx, false)
	if err != nil {
		return err
	}
	tokenIndexer.DeleteToken(userId, token)
	return nil
}

func (u *user) ValidateLoginToken(ctx context.Context, userId int64, token string) (bool, error) {
	if u.cc.Default.SingleLogin {
		existToken, err := u.GetLoginToken(ctx, userId)
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

func (u *user) GetLoginToken(ctx context.Context, userId int64) (string, error) {
	t, exists := tokenIndexer.Get(userId)
	if !exists {
		return "", fmt.Errorf("invalid empty token")
	}

	return t, nil
}

func (u *user) GetCurrentUserPermissions(ctx context.Context) (*types.CurrentUserPermissionsResponse, error) {
	curUser, err := httputils.GetUserFromContext(ctx)
	if err != nil || curUser == nil {
		return nil, errors.ErrUnauthorized
	}

	resp := &types.CurrentUserPermissionsResponse{
		Role:    curUser.Role,
		IsRoot:  curUser.Role == model.RoleRoot,
		APIs:    make([]types.APIResource, 0),
		Scopes:  make([]types.RoleAPIScope, 0),
		Buttons: make([]string, 0),
		Menus:   make([]string, 0),
	}

	// 超级管理员角色
	if curUser.Role == model.RoleRoot {
		apis, apiErr := u.factory.API().List(ctx) // 全部APIs
		if apiErr != nil {
			klog.Errorf("failed to list apis for root permissions: %v", err)
			return nil, errors.ErrServerInternal
		}
		for i := range apis {
			api := toAPIResource(&apis[i])
			resp.APIs = append(resp.APIs, *api)
			resp.Buttons = append(resp.Buttons, rbacapi.Button(apis[i].Method, apis[i].Path))
		}
		resp.Menus = menupkg.Resolve(menupkg.ResolveOptions{IsRoot: true})
		return resp, nil
	}

	roleId := int64(curUser.Role)
	apis, err := u.factory.API().GetByRoleId(ctx, roleId) // 关联 role 的 APIs
	if err != nil {
		klog.Errorf("failed to list APIs for role %d: %v", roleId, err)
		return nil, errors.ErrServerInternal
	}
	for i := range apis {
		api := toAPIResource(&apis[i])
		resp.APIs = append(resp.APIs, *api)
		resp.Buttons = append(resp.Buttons, rbacapi.Button(apis[i].Method, apis[i].Path))
	}

	scopes, err := u.factory.Role().Scope().ListScopes(ctx, roleId)
	if err != nil {
		klog.Errorf("failed to list api scopes for role %d: %v", roleId, err)
		return nil, errors.ErrServerInternal
	}
	for i := range scopes {
		resp.Scopes = append(resp.Scopes, types.RoleAPIScope{
			APIId:        scopes[i].APIId,
			ResourceType: scopes[i].ResourceType,
			ResourceId:   scopes[i].ResourceId,
		})
	}

	explicitMenus, menuErr := u.factory.Role().Menu().ListMenuCodesByRoleId(ctx, roleId)
	if menuErr != nil {
		klog.Errorf("failed to list role menus for role %d: %v", roleId, menuErr)
		return nil, errors.ErrServerInternal
	}
	resp.Menus = menupkg.Resolve(menupkg.ResolveOptions{
		IsAdmin:       curUser.Role == model.RoleAdmin,
		Buttons:       resp.Buttons,
		ExplicitMenus: explicitMenus,
	})
	return resp, nil
}

func toAPIResource(o *model.API) *types.APIResource {
	return &types.APIResource{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Method:      o.Method,
		Path:        o.Path,
		Group:       o.Group,
		Description: o.Description,
	}
}

func (u *user) ValidProxy(ctx *gin.Context, roleId int64) error {
	// 超管已在 ValidAccess 放行；此处防御性跳过
	if roleId == 0 {
		return nil
	}
	// k8s 资源授权由 Permission（scoped kubeconfig）/ 集群 Authorize 兜底，proxy 请求不再按 scope 校验
	return nil
}

func (u *user) ValidAccess(ctx *gin.Context, roleId int64) error {
	method := ctx.Request.Method
	path := ctx.FullPath() // 如 /pixiu/users/:id

	switch rbacaccess.Classify(roleId, method, path) {
	case rbacaccess.Allow:
		if roleId == 0 {
			klog.V(1).Infof("super admin, skipping permission check")
		}
		return nil
	case rbacaccess.Proxy:
		return u.ValidProxy(ctx, roleId)
	case rbacaccess.Check:
		// TODO 通过缓存提示性能
		apisMap, err := u.FormatAPIsForRole(ctx, roleId)
		if err != nil {
			return err
		}
		if !rbacaccess.AllowedBySet(apisMap, method, path) {
			return fmt.Errorf("无访问权限")
		}
		return nil
	default:
		return fmt.Errorf("无访问权限")
	}
}

func (u *user) FormatAPIsForRole(ctx context.Context, roleId int64) (map[string]bool, error) {
	apis, err := u.factory.API().GetByRoleId(ctx, roleId)
	if err != nil {
		return nil, err
	}

	eps := make([]rbacapi.Endpoint, 0, len(apis))
	for i := range apis {
		eps = append(eps, rbacapi.Endpoint{Method: apis[i].Method, Path: apis[i].Path})
	}
	return rbacapi.BuildSet(eps), nil
}

func (u *user) GetTokenKey() []byte {
	k := u.cc.Default.JWTKey
	return []byte(k)
}

// 将 model user 转换成 types（不含 Password，避免通过 API 泄露哈希）
func model2Type(o *model.User) *types.User {
	return &types.User{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		Name:          o.Name,
		Description:   o.Description,
		Status:        o.Status,
		Role:          o.Role,
		Email:         o.Email,
		Phone:         o.Phone,
		OAuthProvider: o.OAuthProvider,
		OAuthOpenID:   o.OAuthOpenID,
		OAuthUnionID:  o.OAuthUnionID,
		OAuthUserID:   o.OAuthUserID,
		AvatarURL:     o.AvatarURL,
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
	}
}

func NewUser(cfg config.Config, f db.ShareDaoFactory) *user {
	return &user{
		cc:      cfg,
		factory: f,
	}
}
