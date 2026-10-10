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
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

const (
	feishuProvider          = "feishu"
	feishuAppTokenCacheSkew = 30 * time.Second
)

type feishuOAuthClient struct{}

type cachedFeishuAppToken struct {
	Token    string
	ExpireAt time.Time
}

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

var feishuAppTokenCaches sync.Map

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
		Name:            util.FirstNonEmpty(info.Data.Name, tokenResp.Data.Name),
		AvatarURL:       util.FirstNonEmpty(info.Data.AvatarURL, tokenResp.Data.AvatarURL),
		OpenID:          util.FirstNonEmpty(info.Data.OpenID, tokenResp.Data.OpenID),
		UnionID:         util.FirstNonEmpty(info.Data.UnionID, tokenResp.Data.UnionID),
		Email:           util.FirstNonEmpty(info.Data.Email, tokenResp.Data.Email),
		EnterpriseEmail: util.FirstNonEmpty(info.Data.EnterpriseEmail, tokenResp.Data.EnterpriseEmail),
		UserID:          util.FirstNonEmpty(info.Data.UserID, tokenResp.Data.UserID),
		Mobile:          util.FirstNonEmpty(info.Data.Mobile, tokenResp.Data.Mobile),
	}
	if profile.OpenID == "" && profile.UnionID == "" {
		return nil, fmt.Errorf("第三方登录未返回可绑定的用户标识")
	}
	return profile, nil
}

func requestFeishuAppAccessToken(ctx context.Context, client *http.Client, cfg *model.OAuthProvider) (string, error) {
	cacheKey := cfg.AppID + ":" + cfg.AppSecret
	if cached, ok := feishuAppTokenCaches.Load(cacheKey); ok {
		item := cached.(cachedFeishuAppToken)
		if item.Token != "" && time.Now().Add(feishuAppTokenCacheSkew).Before(item.ExpireAt) {
			return item.Token, nil
		}
		feishuAppTokenCaches.Delete(cacheKey)
	}

	body := map[string]string{"app_id": cfg.AppID, "app_secret": cfg.AppSecret}
	var out feishuAppTokenResponse
	if err := postFeishuJSON(ctx, client, "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal", "", body, &out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.AppAccessToken == "" {
		return "", fmt.Errorf("获取飞书 app_access_token 失败: %s", util.FirstNonEmpty(out.Msg, fmt.Sprintf("code=%d", out.Code)))
	}
	expire := time.Duration(out.Expire) * time.Second
	if expire <= 0 {
		expire = 2 * time.Hour
	}
	feishuAppTokenCaches.Store(cacheKey, cachedFeishuAppToken{
		Token:    out.AppAccessToken,
		ExpireAt: time.Now().Add(expire),
	})
	return out.AppAccessToken, nil
}

func requestFeishuUserAccessToken(ctx context.Context, client *http.Client, appToken, code string) (*feishuAccessTokenResponse, error) {
	body := map[string]string{"grant_type": "authorization_code", "code": code}
	var out feishuAccessTokenResponse
	if err := postFeishuJSON(ctx, client, "https://open.feishu.cn/open-apis/authen/v1/access_token", appToken, body, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 || out.Data.AccessToken == "" {
		return nil, fmt.Errorf("获取飞书 user_access_token 失败: %s", util.FirstNonEmpty(out.Msg, fmt.Sprintf("code=%d", out.Code)))
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
		return nil, fmt.Errorf("获取飞书用户信息失败: %s", util.FirstNonEmpty(out.Msg, fmt.Sprintf("code=%d", out.Code)))
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
