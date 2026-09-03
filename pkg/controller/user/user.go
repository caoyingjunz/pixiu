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
	"context"
	"fmt"

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

// Login 校验用户名密码并签发 token。
// TODO: 后续迁入 pkg/controller/auth，与注册、验证码统一由 Auth 模块承载。
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
		Name:        o.Name,
		Description: o.Description,
		Status:      o.Status,
		Role:        o.Role,
		Email:       o.Email,
		Phone:       o.Phone,
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
