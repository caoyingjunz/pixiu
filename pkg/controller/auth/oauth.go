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
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const (
	oauthStateTTL          = 5 * time.Minute
	OAuthSessionCookieName = "pixiu_oauth_session"
	OAuthStateCookieMaxAge = int(oauthStateTTL / time.Second)

	// oauth_identities 的 subject_type 取值：union_id / open_id。
	oauthSubjectTypeUnion = "union"
	oauthSubjectTypeOpen  = "open"
)

// 第三方登录绑定失败统一对外文案：不区分「账号已存在 / 已绑定其他登录源 / 权限不足」等原因，
// 避免通过错误信息枚举本地账号绑定状态；具体原因仅经 klog 落服务端日志。
var errOAuthBindingNotAllowed = fmt.Errorf("该账号不符合第三方登录绑定条件，请联系管理员")

// errOAuthRoleConflict：内置「普通角色」主键 id 与历史枚举语义（0 root / 1 admin）冲突时 fail-closed。
var errOAuthRoleConflict = fmt.Errorf("系统默认角色配置异常，请联系管理员")

type oauthSessionIDContextKey struct{}

func WithOAuthSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, oauthSessionIDContextKey{}, strings.TrimSpace(sessionID))
}

func NewOAuthSessionID() string {
	return util.RandomHex(24)
}

// oauthProviderSpec 描述一个第三方登录源的静态元数据与客户端实现。
// 采用单一有序切片：既定义了列表/展示顺序，也绑定 provider 与客户端，避免多份 map 之间失配。
type oauthProviderSpec struct {
	Provider   string
	Name       string
	LoginType  string
	ButtonText string
	Client     oauthProviderClient
}

var oauthProviderSpecs = []oauthProviderSpec{
	{
		Provider:   feishuProvider,
		Name:       "飞书",
		LoginType:  "redirect",
		ButtonText: "飞书扫码登录",
		Client:     feishuOAuthClient{},
	},
	{
		Provider:   "wechat_work",
		Name:       "企业微信",
		LoginType:  "redirect",
		ButtonText: "企业微信登录",
	},
	{
		Provider:   "dingtalk",
		Name:       "钉钉",
		LoginType:  "redirect",
		ButtonText: "钉钉登录",
	},
	{
		Provider:   "ldap",
		Name:       "LDAP",
		LoginType:  "password",
		ButtonText: "LDAP 登录",
	},
}

type oauthProviderClient interface {
	LoginURL(cfg *model.OAuthProvider, state string) (string, error)
	ExchangeUser(ctx context.Context, cfg *model.OAuthProvider, code string) (*oauthUserProfile, error)
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

type oauthProviderExtraConfig struct {
	EmailDomains []string `json:"email_domains"`
}

func (c *controller) ListOAuthProviders(ctx context.Context, enabledOnly bool) ([]*types.OAuthProviderSummary, error) {
	if !enabledOnly {
		if err := controllerutil.CheckRoot(ctx); err != nil {
			return nil, err
		}
	}
	var opts []db.Options
	if enabledOnly {
		opts = append(opts, db.WithEnabled(true))
	}
	objects, err := c.factory.OAuthProvider().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list oauth providers: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	saved := make(map[string]*model.OAuthProvider, len(objects))
	for _, object := range objects {
		saved[object.Provider] = object
	}

	providers := make([]*types.OAuthProviderSummary, 0, len(oauthProviderSpecs))
	for _, spec := range oauthProviderSpecs {
		object := saved[spec.Provider]
		enabled := object != nil && object.Enabled
		if enabledOnly && !enabled {
			continue
		}
		providers = append(providers, &types.OAuthProviderSummary{
			Provider:   spec.Provider,
			Name:       util.FirstNonEmpty(providerName(object), spec.Name),
			LoginType:  util.FirstNonEmpty(providerLoginType(object), spec.LoginType),
			ButtonText: providerButtonText(spec, object),
			Enabled:    enabled,
		})
	}
	return providers, nil
}

func (c *controller) GetOAuthProviderConfig(ctx context.Context, provider string) (*types.OAuthProviderConfig, error) {
	if err := controllerutil.CheckRoot(ctx); err != nil {
		return nil, err
	}
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	cfg, err := c.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, apierrors.ErrServerInternal
	}
	return oauthProvider2Type(spec, cfg), nil
}

func (c *controller) UpdateOAuthProviderConfig(ctx context.Context, provider string, req *types.UpdateOAuthProviderConfigRequest) (*types.OAuthProviderConfig, error) {
	if err := controllerutil.CheckRoot(ctx); err != nil {
		return nil, err
	}
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	existing, err := c.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, apierrors.ErrServerInternal
	}

	// PATCH 部分更新：以 existing 为基线（create 时用 spec 默认 + 零值），
	// 仅覆盖请求中显式传入（非 nil 指针）的字段，其余保持原值。
	name, loginType := spec.Name, spec.LoginType
	enabled, autoCreateUser := false, true
	matchEmail := false
	var appID, appSecret, redirectURI, scopes, configJSON, description string
	if existing != nil {
		name = existing.Name
		loginType = existing.LoginType
		enabled = existing.Enabled
		appID = existing.AppID
		appSecret = existing.AppSecret
		redirectURI = existing.RedirectURI
		scopes = existing.Scopes
		configJSON = existing.ConfigJSON
		autoCreateUser = existing.AutoCreateUser
		matchEmail = existing.MatchEmail
		description = existing.Description
	}
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	if req.LoginType != nil {
		loginType = strings.TrimSpace(*req.LoginType)
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.AppID != nil {
		appID = strings.TrimSpace(*req.AppID)
	}
	if req.AppSecret != nil {
		// app_secret 维持既有语义：传入空串视为保留原值，避免误清空密钥
		if secret := strings.TrimSpace(*req.AppSecret); secret != "" {
			appSecret = secret
		}
	}
	if req.RedirectURI != nil {
		redirectURI = strings.TrimSpace(*req.RedirectURI)
	}
	if req.Scopes != nil {
		scopes = strings.TrimSpace(*req.Scopes)
	}
	if req.ConfigJSON != nil {
		configJSON = strings.TrimSpace(*req.ConfigJSON)
	}
	if req.AutoCreateUser != nil {
		autoCreateUser = *req.AutoCreateUser
	}
	if req.MatchEmail != nil {
		matchEmail = *req.MatchEmail
	}
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}

	if enabled && (appID == "" || appSecret == "" || redirectURI == "") {
		return nil, fmt.Errorf("启用%s登录时 App ID、App Secret、Redirect URL 不能为空", spec.Name)
	}
	if enabled && !validOAuthRedirectURI(redirectURI) {
		return nil, fmt.Errorf("%s登录 Redirect URL 不合法", spec.Name)
	}

	updates := map[string]interface{}{
		"name":             name,
		"login_type":       loginType,
		"enabled":          enabled,
		"app_id":           appID,
		"app_secret":       appSecret,
		"redirect_uri":     redirectURI,
		"scopes":           scopes,
		"config_json":      configJSON,
		"auto_create_user": autoCreateUser,
		"match_email":      matchEmail,
		"description":      description,
	}

	var saved *model.OAuthProvider
	if existing == nil {
		saved, err = c.factory.OAuthProvider().Create(ctx, &model.OAuthProvider{
			Provider:       spec.Provider,
			Name:           name,
			LoginType:      loginType,
			Enabled:        enabled,
			AppID:          appID,
			AppSecret:      appSecret,
			RedirectURI:    redirectURI,
			Scopes:         scopes,
			ConfigJSON:     configJSON,
			AutoCreateUser: autoCreateUser,
			MatchEmail:     matchEmail,
			Description:    description,
		})
	} else {
		if err = c.factory.OAuthProvider().Update(ctx, existing.Id, existing.ResourceVersion, updates); err != nil {
			klog.Errorf("failed to update oauth provider(%s) config: %v", spec.Provider, err)
			return nil, apierrors.ErrServerInternal
		}
		saved, err = c.getOAuthProvider(ctx, spec.Provider)
	}
	if err != nil {
		klog.Errorf("failed to save oauth provider(%s) config: %v", spec.Provider, err)
		return nil, apierrors.ErrServerInternal
	}
	return oauthProvider2Type(spec, saved), nil
}

func (c *controller) GetOAuthProviderLoginURL(ctx context.Context, provider string) (*types.OAuthLoginURLResponse, error) {
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	cfg, err := c.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, apierrors.ErrServerInternal
	}
	if cfg == nil || !cfg.Enabled {
		return &types.OAuthLoginURLResponse{Provider: spec.Provider, Enabled: false}, nil
	}
	if cfg.AppID == "" || cfg.RedirectURI == "" {
		return nil, fmt.Errorf("%s登录未完成配置", spec.Name)
	}
	if spec.Client == nil {
		return nil, fmt.Errorf("%s登录暂未实现", spec.Name)
	}
	state, err := c.oauthState(ctx, spec.Provider)
	if err != nil {
		return nil, err
	}
	loginURL, err := spec.Client.LoginURL(cfg, state)
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

func (c *controller) LoginWithOAuthProvider(ctx context.Context, provider string, req *types.OAuthLoginRequest) (*types.LoginResponse, error) {
	spec, err := getOAuthProviderSpec(provider)
	if err != nil {
		return nil, err
	}
	cfg, err := c.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, apierrors.ErrServerInternal
	}
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("%s登录未启用", spec.Name)
	}
	if !c.validateOAuthState(ctx, spec.Provider, req.State) {
		return nil, fmt.Errorf("第三方登录状态校验失败，请重新登录")
	}
	if spec.Client == nil {
		return nil, fmt.Errorf("%s登录暂未实现", spec.Name)
	}
	profile, err := spec.Client.ExchangeUser(ctx, cfg, req.Code)
	if err != nil {
		klog.Errorf("failed to login with oauth provider(%s): %v", spec.Provider, err)
		return nil, err
	}
	object, err := c.findOrCreateOAuthUser(ctx, cfg, profile)
	if err != nil {
		return nil, err
	}
	if object.Status == model.UserStatusForbidden {
		return nil, fmt.Errorf("用户已被禁用")
	}
	return c.loginResponseForUser(object)
}

func (c *controller) getOAuthProvider(ctx context.Context, provider string) (*model.OAuthProvider, error) {
	return c.factory.OAuthProvider().GetByProvider(ctx, provider)
}

func getOAuthProviderSpec(provider string) (oauthProviderSpec, error) {
	name := strings.TrimSpace(provider)
	for _, spec := range oauthProviderSpecs {
		if spec.Provider == name {
			return spec, nil
		}
	}
	return oauthProviderSpec{}, fmt.Errorf("不支持的登录源: %s", provider)
}

func oauthProvider2Type(spec oauthProviderSpec, o *model.OAuthProvider) *types.OAuthProviderConfig {
	if o == nil {
		return &types.OAuthProviderConfig{
			Provider:       spec.Provider,
			Name:           spec.Name,
			LoginType:      spec.LoginType,
			ButtonText:     spec.ButtonText,
			AutoCreateUser: true,
			MatchEmail:     false,
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
		Provider:       o.Provider,
		Name:           util.FirstNonEmpty(o.Name, spec.Name),
		LoginType:      util.FirstNonEmpty(o.LoginType, spec.LoginType),
		ButtonText:     providerButtonText(spec, o),
		Enabled:        o.Enabled,
		AppID:          o.AppID,
		AppSecretSet:   o.AppSecret != "",
		RedirectURI:    o.RedirectURI,
		Scopes:         o.Scopes,
		ConfigJSON:     o.ConfigJSON,
		AutoCreateUser: o.AutoCreateUser,
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
	if o == nil {
		return spec.ButtonText
	}
	if strings.TrimSpace(o.Name) == "" {
		return spec.ButtonText
	}
	if o.LoginType == "redirect" {
		return o.Name + "登录"
	}
	return o.Name + " 登录"
}

func (c *controller) findOrCreateOAuthUser(ctx context.Context, cfg *model.OAuthProvider, profile *oauthUserProfile) (*model.User, error) {
	provider := util.FirstNonEmpty(profile.Provider, cfg.Provider)
	email := util.FirstNonEmpty(profile.EnterpriseEmail, profile.Email)
	lockKey := oauthIdentityLockKey(cfg, provider, profile, email)
	if lockKey != "" {
		lock := getOAuthIdentityLock(lockKey)
		lock.Lock()
		defer lock.Unlock()
	}
	return c.findOrCreateOAuthUserLocked(ctx, cfg, provider, profile)
}

func (c *controller) findOrCreateOAuthUserLocked(ctx context.Context, cfg *model.OAuthProvider, provider string, profile *oauthUserProfile) (*model.User, error) {
	// 优先按第三方标识（union → open）查 oauth_identities 表。命中后按 identity.UserId 加载用户；
	// identity 命中但用户已被删除时忽略该行（返回 nil 继续后续匹配）。
	object, err := c.findExistingOAuthUser(ctx, provider, profile)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	email := util.FirstNonEmpty(profile.EnterpriseEmail, profile.Email)
	matchedByEmail := false
	if object == nil && cfg.MatchEmail && email != "" && emailAllowedByOAuthConfig(cfg, email) {
		object, err = c.factory.User().GetBy(ctx, db.WithEmail(normalizeEmail(email)))
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		matchedByEmail = object != nil
	}
	if object != nil {
		if matchedByEmail {
			// 邮箱自动绑定仅允许内置「普通角色」用户：管理员或非普通角色账号不允许通过邮箱自动绑定，
			// 避免仅凭第三方邮箱即可接管高权限本地账号。
			role, roleErr := resolveRegistrationRole(ctx, c.factory)
			if roleErr != nil {
				klog.Errorf("failed to resolve default role for oauth email binding: %v", roleErr)
				return nil, apierrors.ErrServerInternal
			}
			if object.Role == model.RoleRoot || int64(object.Role) != role.Id {
				klog.Errorf("refuse oauth email binding: user(%d) role=%d is not the builtin normal role", object.Id, object.Role)
				return nil, errOAuthBindingNotAllowed
			}
		}
		// 幂等补齐 identity 行；users.oauth_* 展示字段仍由 bindOAuthProfile 回填。
		if err = c.saveOAuthIdentity(ctx, c.factory, object.Id, provider, profile, true); err != nil {
			klog.Errorf("failed to ensure oauth identity for user(%d, %s): %v", object.Id, provider, err)
			return nil, apierrors.ErrServerInternal
		}
		return c.bindOAuthProfile(ctx, object, provider, profile)
	}
	if !cfg.AutoCreateUser {
		return nil, fmt.Errorf("第三方账号未绑定 Pixiu 用户")
	}
	return c.createOAuthUser(ctx, cfg, provider, profile, email)
}

// findExistingOAuthUser 按 union → open 顺序查 oauth_identities 并加载对应用户；
// identity 命中但用户不存在（已删除）时忽略该行返回 nil。
func (c *controller) findExistingOAuthUser(ctx context.Context, provider string, profile *oauthUserProfile) (*model.User, error) {
	object, err := c.findUserByOAuthIdentity(ctx, provider, oauthSubjectTypeUnion, profile.UnionID)
	if err != nil || object != nil {
		return object, err
	}
	return c.findUserByOAuthIdentity(ctx, provider, oauthSubjectTypeOpen, profile.OpenID)
}

func (c *controller) findUserByOAuthIdentity(ctx context.Context, provider, subjectType, subject string) (*model.User, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, nil
	}
	identity, err := c.factory.OAuthIdentity().GetBySubject(ctx, provider, subjectType, util.TruncateRunes(subject, 128))
	if err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, nil
	}
	return c.factory.User().Get(ctx, identity.UserId)
}

// oauthIdentitySubjects 返回第三方标识中非空且顺序稳定的 (subject_type, subject) 列表。
func oauthIdentitySubjects(profile *oauthUserProfile) [][2]string {
	pairs := make([][2]string, 0, 2)
	if s := strings.TrimSpace(profile.UnionID); s != "" {
		pairs = append(pairs, [2]string{oauthSubjectTypeUnion, s})
	}
	if s := strings.TrimSpace(profile.OpenID); s != "" {
		pairs = append(pairs, [2]string{oauthSubjectTypeOpen, s})
	}
	return pairs
}

// saveOAuthIdentity 写入 (provider, subject_type, subject) → userId 绑定行。
// ignoreConflict=true 时唯一冲突（已存在）按幂等忽略；否则透传冲突错误（供事务回滚）。
func (c *controller) saveOAuthIdentity(ctx context.Context, factory db.ShareDaoFactory, userId int64, provider string, profile *oauthUserProfile, ignoreConflict bool) error {
	for _, pair := range oauthIdentitySubjects(profile) {
		object := &model.OAuthIdentity{
			UserId:      userId,
			Provider:    provider,
			SubjectType: pair[0],
			Subject:     util.TruncateRunes(pair[1], 128),
		}
		if err := factory.OAuthIdentity().Create(ctx, object); err != nil {
			if ignoreConflict && utilerrors.IsUniqueConstraintError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (c *controller) bindOAuthProfile(ctx context.Context, object *model.User, provider string, profile *oauthUserProfile) (*model.User, error) {
	if object.OAuthProvider != "" && object.OAuthProvider != provider {
		klog.Errorf("refuse oauth binding: user(%d) already bound to another provider(%s)", object.Id, object.OAuthProvider)
		return nil, errOAuthBindingNotAllowed
	}
	if object.OAuthOpenID != "" && profile.OpenID != "" && object.OAuthOpenID != profile.OpenID {
		klog.Errorf("refuse oauth binding: user(%d) already bound to another open_id", object.Id)
		return nil, errOAuthBindingNotAllowed
	}
	if object.OAuthUnionID != "" && profile.UnionID != "" && object.OAuthUnionID != profile.UnionID {
		klog.Errorf("refuse oauth binding: user(%d) already bound to another union_id", object.Id)
		return nil, errOAuthBindingNotAllowed
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
	if err := c.factory.User().Update(ctx, object.Id, object.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to bind oauth user(%d, %s): %v", object.Id, provider, err)
		return nil, apierrors.ErrServerInternal
	}
	return c.factory.User().Get(ctx, object.Id)
}

func (c *controller) createOAuthUser(ctx context.Context, cfg *model.OAuthProvider, provider string, profile *oauthUserProfile, email string) (*model.User, error) {
	// 自动建号统一使用内置「普通角色」与角色所属租户，绝不授予管理员权限。
	role, err := resolveRegistrationRole(ctx, c.factory)
	if err != nil {
		klog.Errorf("failed to resolve default role for oauth user: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	// fail-closed：内置「普通角色」主键 id 与历史枚举语义（0 root / 1 admin）冲突时，
	// model.UserLevel(role.Id) 会把新建用户误判为 root/admin，拒绝自动建号。
	if role.Id == int64(model.RoleRoot) || role.Id == int64(model.RoleAdmin) {
		klog.Errorf("refuse to auto create oauth user: default role id(%d) collides with builtin root/admin role semantics", role.Id)
		return nil, errOAuthRoleConflict
	}
	password, err := util.EncryptUserPassword(util.RandomHex(24))
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	name, err := c.uniqueOAuthUserName(ctx, util.FirstNonEmpty(profile.Name, email, profile.UserID, profile.OpenID, provider+"-user"))
	if err != nil {
		klog.Errorf("failed to resolve unique oauth user name(%s): %v", provider, err)
		return nil, apierrors.ErrServerInternal
	}

	// 建用户与写 identity 行同一事务：identity 唯一冲突时整体回滚，避免产生无绑定行的孤儿用户。
	var object *model.User
	err = c.factory.Transaction(ctx, func(factory db.ShareDaoFactory) error {
		created, createErr := factory.User().Create(ctx, &model.User{
			Name:          util.TruncateRunes(name, 48),
			TenantId:      role.TenantId,
			Password:      password,
			Status:        model.UserStatusNormal,
			Role:          model.UserLevel(role.Id),
			Email:         util.TruncateRunes(normalizeEmail(email), 128),
			Phone:         util.TruncateRunes(profile.Mobile, 32),
			OAuthProvider: provider,
			OAuthOpenID:   util.TruncateRunes(profile.OpenID, 128),
			OAuthUnionID:  util.TruncateRunes(profile.UnionID, 128),
			OAuthUserID:   util.TruncateRunes(profile.UserID, 128),
			AvatarURL:     util.TruncateRunes(profile.AvatarURL, 512),
			Description:   "第三方登录自动创建",
		})
		if createErr != nil {
			return createErr
		}
		if createErr = c.saveOAuthIdentity(ctx, factory, created.Id, provider, profile, false); createErr != nil {
			return createErr
		}
		object = created
		return nil
	})
	if err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			// 跨实例并发：另一实例已抢先建号并写入 identity（事务已回滚，本实例无孤儿用户）。
			// 重查一次 identity，命中即返回既有用户；仍无则如实报错，不再重复建号。
			if existing, lookupErr := c.findExistingOAuthUser(ctx, provider, profile); lookupErr == nil && existing != nil {
				return existing, nil
			}
			klog.Errorf("oauth identity conflict but no existing user found(provider=%s): %v", provider, err)
			return nil, errOAuthBindingNotAllowed
		}
		klog.Errorf("failed to create oauth user(%s): %v", provider, err)
		return nil, apierrors.ErrServerInternal
	}
	return object, nil
}

func (c *controller) uniqueOAuthUserName(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	base = strings.ReplaceAll(base, " ", "_")
	base = util.TruncateRunes(base, 48)
	if base == "" {
		base = "oauth-user"
	}
	name := base
	for i := 0; i < 20; i++ {
		existing, err := c.factory.User().GetUserByName(ctx, name)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return name, nil
		}
		name = fmt.Sprintf("%s-%02d", base, i+1)
	}
	return fmt.Sprintf("%s-%s", base, util.RandomHex(4)), nil
}

func emailAllowedByOAuthConfig(cfg *model.OAuthProvider, email string) bool {
	domain := emailDomain(email)
	if cfg == nil || domain == "" || strings.TrimSpace(cfg.ConfigJSON) == "" {
		return false
	}
	var extra oauthProviderExtraConfig
	if err := json.Unmarshal([]byte(cfg.ConfigJSON), &extra); err != nil {
		return false
	}
	for _, allowed := range extra.EmailDomains {
		if strings.EqualFold(strings.TrimSpace(allowed), domain) {
			return true
		}
	}
	return false
}

func emailDomain(email string) string {
	parts := strings.Split(normalizeEmail(email), "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func validOAuthRedirectURI(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	// 收紧校验：仅接受 http/https 且带 host，并拒绝带用户信息（user:pass@）与片段（#）的地址，
	// 避免回调地址被注入凭据或借助 fragment 绕过上下游校验。
	if u.User != nil || u.Fragment != "" {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
