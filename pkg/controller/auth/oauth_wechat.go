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
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

// wechatWebProvider 对应微信开放平台「网站应用」扫码登录。
const wechatWebProvider = "wechat_web"

const (
	wechatQrConnectEndpoint   = "https://open.weixin.qq.com/connect/qrconnect"
	wechatAccessTokenEndpoint = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatUserInfoEndpoint    = "https://api.weixin.qq.com/sns/userinfo"
)

type wechatOAuthClient struct{}

type wechatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	OpenID      string `json:"openid"`
	UnionID     string `json:"unionid"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wechatUserInfoResponse struct {
	OpenID     string `json:"openid"`
	Nickname   string `json:"nickname"`
	HeadImgURL string `json:"headimgurl"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (wechatOAuthClient) LoginURL(cfg *model.OAuthProvider, state string) (string, error) {
	values := url.Values{}
	values.Set("appid", cfg.AppID)
	values.Set("redirect_uri", cfg.RedirectURI)
	values.Set("response_type", "code")
	values.Set("scope", "snsapi_login")
	values.Set("state", state)
	// 微信强制要求授权链接以 #wechat_redirect 结尾（fragment 不参与 query 编码）。
	return wechatQrConnectEndpoint + "?" + values.Encode() + "#wechat_redirect", nil
}

func (wechatOAuthClient) ExchangeUser(ctx context.Context, cfg *model.OAuthProvider, code string) (*oauthUserProfile, error) {
	client := &http.Client{Timeout: 12 * time.Second}

	tokenQuery := url.Values{}
	tokenQuery.Set("appid", cfg.AppID)
	tokenQuery.Set("secret", cfg.AppSecret)
	tokenQuery.Set("code", code)
	tokenQuery.Set("grant_type", "authorization_code")

	var tokenResp wechatAccessTokenResponse
	if err := getWechatJSON(ctx, client, wechatAccessTokenEndpoint+"?"+tokenQuery.Encode(), &tokenResp); err != nil {
		return nil, err
	}
	if err := tokenResp.error(); err != nil {
		return nil, fmt.Errorf("获取微信 access_token 失败: %w", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.OpenID == "" {
		return nil, fmt.Errorf("获取微信 access_token 失败: 返回数据不完整")
	}

	infoQuery := url.Values{}
	infoQuery.Set("access_token", tokenResp.AccessToken)
	infoQuery.Set("openid", tokenResp.OpenID)
	infoQuery.Set("lang", "zh_CN")

	var infoResp wechatUserInfoResponse
	if err := getWechatJSON(ctx, client, wechatUserInfoEndpoint+"?"+infoQuery.Encode(), &infoResp); err != nil {
		return nil, err
	}
	if err := infoResp.error(); err != nil {
		return nil, fmt.Errorf("获取微信用户信息失败: %w", err)
	}

	profile := &oauthUserProfile{
		Provider:  wechatWebProvider,
		Name:      sanitizeWechatNickname(infoResp.Nickname),
		AvatarURL: infoResp.HeadImgURL,
		OpenID:    util.FirstNonEmpty(infoResp.OpenID, tokenResp.OpenID),
		UnionID:   util.FirstNonEmpty(infoResp.UnionID, tokenResp.UnionID),
	}
	if profile.OpenID == "" && profile.UnionID == "" {
		return nil, fmt.Errorf("第三方登录未返回可绑定的用户标识")
	}
	return profile, nil
}

func (r wechatAccessTokenResponse) error() error {
	if r.ErrCode != 0 {
		return fmt.Errorf("%s", util.FirstNonEmpty(r.ErrMsg, fmt.Sprintf("errcode=%d", r.ErrCode)))
	}
	return nil
}

func (r wechatUserInfoResponse) error() error {
	if r.ErrCode != 0 {
		return fmt.Errorf("%s", util.FirstNonEmpty(r.ErrMsg, fmt.Sprintf("errcode=%d", r.ErrCode)))
	}
	return nil
}

// sanitizeWechatNickname 丢弃非 BMP（码点 > 0xFFFF）字符：微信昵称常含 emoji，
// 而本仓 users.name 列为 3 字节 utf8，4 字节字符会导致入库失败。
func sanitizeWechatNickname(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if r > 0xFFFF {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// getWechatJSON 发起 GET 请求并解码 JSON。微信业务错误以 HTTP 200 + body 内 errcode 表达，
// 由调用方在解码后显式判定 errcode，此 helper 只负责传输层（状态码/解码）。
func getWechatJSON(ctx context.Context, client *http.Client, endpoint string, out interface{}) error {
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
		return fmt.Errorf("微信接口请求失败: status=%d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
