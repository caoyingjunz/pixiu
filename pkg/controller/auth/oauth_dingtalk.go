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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

// dingtalkProvider 对应钉钉「扫码登录」（新版 oauth2 开放能力，v1.0 API）。
const dingtalkProvider = "dingtalk"

const (
	dingtalkAuthorizeEndpoint       = "https://login.dingtalk.com/oauth2/auth"
	dingtalkUserAccessTokenEndpoint = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
	dingtalkUserMeEndpoint          = "https://api.dingtalk.com/v1.0/contact/users/me"
)

type dingtalkOAuthClient struct{}

type dingtalkUserTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpireIn     int    `json:"expireIn"`
	CorpID       string `json:"corpId"`
}

type dingtalkUserResponse struct {
	Nick      string `json:"nick"`
	AvatarURL string `json:"avatarUrl"`
	Mobile    string `json:"mobile"`
	OpenID    string `json:"openId"`
	UnionID   string `json:"unionId"`
	Email     string `json:"email"`
}

// dingtalkErrorResponse 对应钉钉 v1.0 API 的错误体（非 2xx 时返回）。
type dingtalkErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (dingtalkOAuthClient) LoginURL(cfg *model.OAuthProvider, state string) (string, error) {
	values := url.Values{}
	values.Set("redirect_uri", cfg.RedirectURI)
	values.Set("response_type", "code")
	values.Set("client_id", cfg.AppID)
	values.Set("scope", "openid")
	values.Set("state", state)
	values.Set("prompt", "consent")
	return dingtalkAuthorizeEndpoint + "?" + values.Encode(), nil
}

func (dingtalkOAuthClient) ExchangeUser(ctx context.Context, cfg *model.OAuthProvider, code string) (*oauthUserProfile, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	token, err := requestDingtalkUserAccessToken(ctx, client, cfg, code)
	if err != nil {
		return nil, err
	}
	user, err := requestDingtalkUser(ctx, client, token.AccessToken)
	if err != nil {
		return nil, err
	}
	profile := &oauthUserProfile{
		Provider:  dingtalkProvider,
		Name:      user.Nick,
		AvatarURL: user.AvatarURL,
		OpenID:    user.OpenID,
		UnionID:   user.UnionID,
		Email:     user.Email,
		Mobile:    user.Mobile,
	}
	if profile.OpenID == "" && profile.UnionID == "" {
		return nil, fmt.Errorf("第三方登录未返回可绑定的用户标识")
	}
	return profile, nil
}

// requestDingtalkUserAccessToken 用授权码换取用户级 accessToken。
// 钉钉 v1.0 API 以 HTTP 状态码表错（非 2xx 即失败），错误体形如 {"code","message","requestid"}。
func requestDingtalkUserAccessToken(ctx context.Context, client *http.Client, cfg *model.OAuthProvider, code string) (*dingtalkUserTokenResponse, error) {
	body := map[string]string{
		"clientId":     cfg.AppID,
		"clientSecret": cfg.AppSecret,
		"code":         code,
		"grantType":    "authorization_code",
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dingtalkUserAccessTokenEndpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("获取钉钉 userAccessToken 失败: %s", dingtalkErrorSummary(resp))
	}
	var out dingtalkUserTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("获取钉钉 userAccessToken 失败: 返回数据不完整")
	}
	return &out, nil
}

func requestDingtalkUser(ctx context.Context, client *http.Client, accessToken string) (*dingtalkUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dingtalkUserMeEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("获取钉钉用户信息失败: %s", dingtalkErrorSummary(resp))
	}
	var out dingtalkUserResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// dingtalkErrorSummary 读取非 2xx 响应体，拼装 status + code/message 摘要。
func dingtalkErrorSummary(resp *http.Response) string {
	var apiErr dingtalkErrorResponse
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&apiErr)
	detail := util.FirstNonEmpty(apiErr.Message, apiErr.Code)
	if detail == "" {
		return fmt.Sprintf("status=%d", resp.StatusCode)
	}
	return fmt.Sprintf("status=%d %s", resp.StatusCode, detail)
}
