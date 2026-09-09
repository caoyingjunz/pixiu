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

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

const (
	feishuProvider = "feishu"
	oauthStateTTL  = 5 * time.Minute
)

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

func (c *controller) ListOAuthProviders(ctx context.Context, enabledOnly bool) ([]*types.OAuthProviderSummary, error) {
	if !enabledOnly {
		if err := controllerutil.CheckRoot(ctx); err != nil {
			return nil, err
		}
	}
	objects, err := c.factory.OAuthProvider().List(ctx)
	if err != nil {
		klog.Errorf("failed to list oauth providers: %v", err)
		return nil, apierrors.ErrServerInternal
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
	old, err := c.getOAuthProvider(ctx, spec.Provider)
	if err != nil {
		klog.Errorf("failed to get oauth provider(%s) config: %v", spec.Provider, err)
		return nil, apierrors.ErrServerInternal
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

	saved, err := c.factory.OAuthProvider().Save(ctx, &model.OAuthProvider{
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
	providerClient, ok := oauthProviderClients[spec.Provider]
	if !ok {
		return nil, fmt.Errorf("%s登录暂未实现", spec.Name)
	}
	state := c.oauthState(spec.Provider)
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
	if !c.validateOAuthState(spec.Provider, req.State) {
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
		Provider:       o.Provider,
		Name:           firstNonEmpty(o.Name, spec.Name),
		LoginType:      firstNonEmpty(o.LoginType, spec.LoginType),
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

func (c *controller) findOrCreateOAuthUser(ctx context.Context, cfg *model.OAuthProvider, profile *oauthUserProfile) (*model.User, error) {
	provider := firstNonEmpty(profile.Provider, cfg.Provider)
	var object *model.User
	var err error
	if profile.UnionID != "" {
		object, err = c.factory.User().GetBy(ctx, db.WithOAuthUnionID(provider, profile.UnionID))
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
	}
	if object == nil && profile.OpenID != "" {
		object, err = c.factory.User().GetBy(ctx, db.WithOAuthOpenID(provider, profile.OpenID))
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
	}
	email := firstNonEmpty(profile.EnterpriseEmail, profile.Email)
	if object == nil && cfg.MatchEmail && email != "" {
		object, err = c.factory.User().GetBy(ctx, db.WithEmail(normalizeEmail(email)))
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
	}
	if object != nil {
		return c.bindOAuthProfile(ctx, object, provider, profile)
	}
	if !cfg.AutoCreateUser {
		return nil, fmt.Errorf("第三方账号未绑定 Pixiu 用户")
	}
	return c.createOAuthUser(ctx, cfg, provider, profile, email)
}

func (c *controller) bindOAuthProfile(ctx context.Context, object *model.User, provider string, profile *oauthUserProfile) (*model.User, error) {
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
	if err := c.factory.User().Update(ctx, object.Id, object.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to bind oauth user(%d, %s): %v", object.Id, provider, err)
		return nil, apierrors.ErrServerInternal
	}
	return c.factory.User().Get(ctx, object.Id)
}

func (c *controller) createOAuthUser(ctx context.Context, cfg *model.OAuthProvider, provider string, profile *oauthUserProfile, email string) (*model.User, error) {
	password, err := util.EncryptUserPassword(randomHex(24))
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	name := c.uniqueOAuthUserName(ctx, firstNonEmpty(profile.Name, email, profile.UserID, profile.OpenID, provider+"-user"))
	object, err := c.factory.User().Create(ctx, &model.User{
		Name:          name,
		Password:      password,
		Status:        model.UserStatusNormal,
		Role:          cfg.DefaultRole,
		Email:         normalizeEmail(email),
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
		return nil, apierrors.ErrServerInternal
	}
	return object, nil
}

func (c *controller) uniqueOAuthUserName(ctx context.Context, base string) string {
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
		existing, err := c.factory.User().GetUserByName(ctx, name)
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

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func (c *controller) oauthState(provider string) string {
	issuedAt := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randomHex(16)
	payload := strings.Join([]string{provider, issuedAt, nonce}, ":")
	signature := c.signOAuthState(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + signature))
}

func (c *controller) validateOAuthState(provider, state string) bool {
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
	expected := c.signOAuthState(payload)
	return hmac.Equal([]byte(parts[3]), []byte(expected))
}

func (c *controller) signOAuthState(payload string) string {
	mac := hmac.New(sha256.New, c.GetTokenKey())
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
