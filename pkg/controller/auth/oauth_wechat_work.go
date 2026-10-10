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
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

// wechatWorkProvider 对应企业微信登录。支持两种模式（由 config_json.login_mode 选择）：
//   - sso（默认）：企业微信网站扫码登录（PC 浏览器，login.work.weixin.qq.com/wwlogin）
//   - web：企业微信内网页授权（微信客户端内置浏览器，open.weixin.qq.com/connect/oauth2）
const wechatWorkProvider = "wechat_work"

const (
	wechatWorkSSOLoginEndpoint     = "https://login.work.weixin.qq.com/wwlogin/sso/login"
	wechatWorkWebAuthorizeEndpoint = "https://open.weixin.qq.com/connect/oauth2/authorize"
	wechatWorkTokenEndpoint        = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	wechatWorkUserInfoEndpoint     = "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"
	wechatWorkUserDetailEndpoint   = "https://qyapi.weixin.qq.com/cgi-bin/user/get"

	wechatWorkTokenCacheSkew = 30 * time.Second
)

type wechatWorkOAuthClient struct{}

// wechatWorkExtraConfig 对应 config_json 中企业微信专有配置。
type wechatWorkExtraConfig struct {
	// AgentID 企业应用 id，仅 sso（网站扫码登录）模式需要。
	AgentID string `json:"agent_id"`
	// LoginMode 登录模式：sso（默认）| web。
	LoginMode string `json:"login_mode"`
}

type cachedWechatWorkToken struct {
	Token    string
	ExpireAt time.Time
}

// wechatWorkTokenCaches 缓存 corp access_token，key=corpid:corpsecret。
// 独立于飞书的缓存，避免不同平台 key 空间串扰。
var wechatWorkTokenCaches sync.Map

type wechatWorkTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type wechatWorkUserInfoResponse struct {
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
	UserID     string `json:"userid"`
	OpenID     string `json:"openid"`
	UserTicket string `json:"user_ticket"`
}

type wechatWorkUserDetailResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Name    string `json:"name"`
	Mobile  string `json:"mobile"`
	Email   string `json:"email"`
	Avatar  string `json:"avatar"`
}

func (wechatWorkOAuthClient) LoginURL(cfg *model.OAuthProvider, state string) (string, error) {
	extra := parseWechatWorkExtraConfig(cfg.ConfigJSON)
	if extra.LoginMode == wechatWorkLoginModeWeb {
		values := url.Values{}
		values.Set("appid", cfg.AppID)
		values.Set("redirect_uri", cfg.RedirectURI)
		values.Set("response_type", "code")
		values.Set("scope", "snsapi_base")
		values.Set("state", state)
		// 企业微信内网页授权强制要求授权链接以 #wechat_redirect 结尾（fragment 不参与 query 编码）。
		return wechatWorkWebAuthorizeEndpoint + "?" + values.Encode() + "#wechat_redirect", nil
	}

	// sso（默认）：企业微信网站扫码登录。
	if extra.AgentID == "" {
		return "", fmt.Errorf("企业微信扫码登录需配置 agent_id")
	}
	values := url.Values{}
	values.Set("login_type", "CorpApp")
	values.Set("appid", cfg.AppID)
	values.Set("agentid", extra.AgentID)
	values.Set("redirect_uri", cfg.RedirectURI)
	values.Set("state", state)
	return wechatWorkSSOLoginEndpoint + "?" + values.Encode(), nil
}

const (
	wechatWorkLoginModeSSO = "sso"
	wechatWorkLoginModeWeb = "web"
)

func (wechatWorkOAuthClient) ExchangeUser(ctx context.Context, cfg *model.OAuthProvider, code string) (*oauthUserProfile, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	accessToken, err := requestWechatWorkAccessToken(ctx, client, cfg)
	if err != nil {
		return nil, err
	}
	identity, err := requestWechatWorkUserInfo(ctx, client, accessToken, code)
	if err != nil {
		return nil, err
	}

	// 成员详情为 best-effort：非本企业成员（仅返回 openid）调用会报错，取不到时降级为无详情，不中断登录。
	var detail *wechatWorkUserDetailResponse
	if strings.TrimSpace(identity.UserID) != "" {
		detail, _ = requestWechatWorkUserDetail(ctx, client, accessToken, identity.UserID)
	}

	profile := &oauthUserProfile{
		Provider: wechatWorkProvider,
		// 企业成员返回 userid（企业内稳定标识，放 union 槽）、非成员/外部返回 openid。
		UnionID: identity.UserID,
		OpenID:  identity.OpenID,
	}
	if detail != nil {
		profile.Name = detail.Name
		profile.AvatarURL = detail.Avatar
		profile.Email = detail.Email
		profile.Mobile = detail.Mobile
	}
	if profile.OpenID == "" && profile.UnionID == "" {
		return nil, fmt.Errorf("第三方登录未返回可绑定的用户标识")
	}
	return profile, nil
}

func parseWechatWorkExtraConfig(raw string) wechatWorkExtraConfig {
	var extra wechatWorkExtraConfig
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &extra)
	}
	extra.AgentID = strings.TrimSpace(extra.AgentID)
	extra.LoginMode = strings.ToLower(strings.TrimSpace(extra.LoginMode))
	if extra.LoginMode == "" {
		extra.LoginMode = wechatWorkLoginModeSSO
	}
	return extra
}

func requestWechatWorkAccessToken(ctx context.Context, client *http.Client, cfg *model.OAuthProvider) (string, error) {
	cacheKey := cfg.AppID + ":" + cfg.AppSecret
	if cached, ok := wechatWorkTokenCaches.Load(cacheKey); ok {
		item := cached.(cachedWechatWorkToken)
		if item.Token != "" && time.Now().Add(wechatWorkTokenCacheSkew).Before(item.ExpireAt) {
			return item.Token, nil
		}
		wechatWorkTokenCaches.Delete(cacheKey)
	}

	query := url.Values{}
	query.Set("corpid", cfg.AppID)
	query.Set("corpsecret", cfg.AppSecret)

	var out wechatWorkTokenResponse
	if err := getWechatWorkJSON(ctx, client, wechatWorkTokenEndpoint+"?"+query.Encode(), &out); err != nil {
		return "", err
	}
	// 企业微信业务错误以 HTTP 200 + errcode 表达。
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", fmt.Errorf("获取企业微信 access_token 失败: %s", util.FirstNonEmpty(out.ErrMsg, fmt.Sprintf("errcode=%d", out.ErrCode)))
	}
	expire := time.Duration(out.ExpiresIn) * time.Second
	if expire <= 0 {
		expire = 2 * time.Hour
	}
	wechatWorkTokenCaches.Store(cacheKey, cachedWechatWorkToken{
		Token:    out.AccessToken,
		ExpireAt: time.Now().Add(expire),
	})
	return out.AccessToken, nil
}

func requestWechatWorkUserInfo(ctx context.Context, client *http.Client, accessToken, code string) (*wechatWorkUserInfoResponse, error) {
	query := url.Values{}
	query.Set("access_token", accessToken)
	query.Set("code", code)

	var out wechatWorkUserInfoResponse
	if err := getWechatWorkJSON(ctx, client, wechatWorkUserInfoEndpoint+"?"+query.Encode(), &out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("获取企业微信用户身份失败: %s", util.FirstNonEmpty(out.ErrMsg, fmt.Sprintf("errcode=%d", out.ErrCode)))
	}
	return &out, nil
}

func requestWechatWorkUserDetail(ctx context.Context, client *http.Client, accessToken, userID string) (*wechatWorkUserDetailResponse, error) {
	query := url.Values{}
	query.Set("access_token", accessToken)
	query.Set("userid", userID)

	var out wechatWorkUserDetailResponse
	if err := getWechatWorkJSON(ctx, client, wechatWorkUserDetailEndpoint+"?"+query.Encode(), &out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("获取企业微信成员详情失败: %s", util.FirstNonEmpty(out.ErrMsg, fmt.Sprintf("errcode=%d", out.ErrCode)))
	}
	return &out, nil
}

// getWechatWorkJSON 发起 GET 请求并解码 JSON。企业微信业务错误以 HTTP 200 + body 内 errcode 表达，
// 由调用方在解码后显式判定 errcode，此 helper 只负责传输层（状态码/解码）。
func getWechatWorkJSON(ctx context.Context, client *http.Client, endpoint string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("企业微信接口请求失败: status=%d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
