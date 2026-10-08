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

package datasourceauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	nacosTokenDefaultTTL = 5 * time.Hour
	nacosTokenMinTTL     = time.Minute
)

var errNacosAuthDisabled = errors.New("nacos authentication is disabled")

type RequestExecutor func(*http.Request) (*http.Response, error)

type Authenticator struct {
	mu          sync.Mutex
	nacosTokens map[int64]nacosTokenEntry
}

type nacosTokenEntry struct {
	token       string
	expiresAt   time.Time
	fingerprint string
}

func NewAuthenticator() *Authenticator {
	return &Authenticator{nacosTokens: make(map[int64]nacosTokenEntry)}
}

func (a *Authenticator) RequiresPodProxy(datasource *types.Datasource) bool {
	if isNacosDatasource(datasource) {
		return hasNacosCredentials(datasource)
	}
	return BasicAuthorization(datasource) != ""
}

func (a *Authenticator) Prepare(
	ctx context.Context,
	datasource *types.Datasource,
	target *url.URL,
	execute RequestExecutor,
) (string, error) {
	if !isNacosDatasource(datasource) {
		return BasicAuthorization(datasource), nil
	}
	if !hasNacosCredentials(datasource) {
		return "", nil
	}
	if target == nil || execute == nil {
		return "", fmt.Errorf("nacos authentication requires a request target and executor")
	}

	token, err := a.nacosToken(datasource, func() (string, time.Duration, error) {
		loginTarget := *target
		loginTarget.Path = nacosLoginPath(target.Path, datasource.Config.Nacos)
		loginTarget.RawPath = ""
		loginTarget.RawQuery = ""
		request, err := newNacosLoginRequest(ctx, datasource, loginTarget.String())
		if err != nil {
			return "", 0, err
		}
		response, err := execute(request)
		if err != nil {
			return "", 0, err
		}
		defer response.Body.Close()
		return parseNacosLoginResponse(response)
	})
	if err != nil {
		return "", err
	}
	if token != "" {
		query := target.Query()
		query.Set("accessToken", token)
		target.RawQuery = query.Encode()
	}
	return "", nil
}

func BasicAuthorization(datasource *types.Datasource) string {
	if datasource == nil || isNacosDatasource(datasource) {
		return ""
	}

	var username, password string
	switch datasource.Type {
	case model.DatasourceTypeLog, model.DatasourceTypeMiddleware:
		if datasource.Config.Log == nil {
			return ""
		}
		username = datasource.Config.Log.UserName
		password = datasource.Config.Log.Password
	case model.DatasourceTypeAlert:
		if datasource.Config.Alert == nil {
			return ""
		}
		username = datasource.Config.Alert.UserName
		password = datasource.Config.Alert.Password
	default:
		return ""
	}
	if strings.TrimSpace(username) == "" && strings.TrimSpace(password) == "" {
		return ""
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

func DatasourceURL(datasource *types.Datasource) (string, error) {
	if datasource == nil {
		return "", fmt.Errorf("datasource is required")
	}
	switch datasource.Type {
	case model.DatasourceTypeLog, model.DatasourceTypeMiddleware:
		if datasource.Config.Log != nil && strings.TrimSpace(datasource.Config.Log.URL) != "" {
			return datasource.Config.Log.URL, nil
		}
	case model.DatasourceTypeAlert:
		if datasource.Config.Alert != nil && strings.TrimSpace(datasource.Config.Alert.URL) != "" {
			return datasource.Config.Alert.URL, nil
		}
	}
	return "", fmt.Errorf("datasource %d has no proxy URL", datasource.Id)
}

func ValidateExternalTarget(datasource *types.Datasource, target *url.URL) error {
	baseURL, err := DatasourceURL(datasource)
	if err != nil {
		return err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid datasource URL: %w", err)
	}
	if target == nil || !strings.EqualFold(base.Hostname(), target.Hostname()) || base.Scheme != target.Scheme {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	if !isNacosDatasource(datasource) && effectivePort(base) != effectivePort(target) {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	return nil
}

func (a *Authenticator) nacosToken(datasource *types.Datasource, login func() (string, time.Duration, error)) (string, error) {
	fingerprint := nacosCredentialFingerprint(datasource)
	now := time.Now()
	a.mu.Lock()
	cached, ok := a.nacosTokens[datasource.Id]
	a.mu.Unlock()
	if ok && cached.fingerprint == fingerprint && cached.expiresAt.After(now) {
		return cached.token, nil
	}

	token, ttl, err := login()
	if errors.Is(err, errNacosAuthDisabled) {
		token, ttl, err = "", 10*time.Minute, nil
	}
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = nacosTokenDefaultTTL
	}
	cacheTTL := ttl - time.Minute
	if cacheTTL < nacosTokenMinTTL {
		cacheTTL = nacosTokenMinTTL
	}
	a.mu.Lock()
	a.nacosTokens[datasource.Id] = nacosTokenEntry{
		token:       token,
		expiresAt:   now.Add(cacheTTL),
		fingerprint: fingerprint,
	}
	a.mu.Unlock()
	return token, nil
}

func hasNacosCredentials(datasource *types.Datasource) bool {
	return datasource != nil && datasource.Config.Log != nil && strings.TrimSpace(datasource.Config.Log.UserName) != ""
}

func isNacosDatasource(datasource *types.Datasource) bool {
	return datasource != nil && datasource.SubType == model.DatasourceSubTypeNacos
}

func nacosCredentialFingerprint(datasource *types.Datasource) string {
	version := ""
	if datasource.Config.Nacos != nil {
		version = datasource.Config.Nacos.Version
	}
	return strings.Join([]string{datasource.Config.Log.URL, datasource.Config.Log.UserName, datasource.Config.Log.Password, version}, "\x00")
}

func newNacosLoginRequest(ctx context.Context, datasource *types.Datasource, target string) (*http.Request, error) {
	body := url.Values{
		"username": {datasource.Config.Log.UserName},
		"password": {datasource.Config.Log.Password},
	}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	for _, header := range datasource.Config.Headers {
		if key, value := strings.TrimSpace(header.Key), strings.TrimSpace(header.Value); key != "" && value != "" {
			request.Header.Set(key, value)
		}
	}
	return request, nil
}

func nacosLoginPath(requestPath string, config *types.NacosSourceConfig) string {
	if strings.HasPrefix(requestPath, "/nacos/v3/") {
		return "/nacos/v3/auth/user/login"
	}
	if strings.HasPrefix(requestPath, "/v3/") || (config != nil && config.Version == "v3") {
		return "/v3/auth/user/login"
	}
	return "/nacos/v1/auth/login"
}

func parseNacosLoginResponse(response *http.Response) (string, time.Duration, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", 0, err
	}
	if response.StatusCode == http.StatusNotFound {
		return "", 0, errNacosAuthDisabled
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", 0, fmt.Errorf("nacos login failed: %s", nacosResponseMessage(body))
	}

	var payload struct {
		Code        json.RawMessage `json:"code"`
		Data        json.RawMessage `json:"data"`
		AccessToken string          `json:"accessToken"`
		TokenTTL    json.RawMessage `json:"tokenTtl"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", 0, fmt.Errorf("invalid nacos login response: %w", err)
	}
	if nacosResponseFailed(payload.Code) {
		return "", 0, fmt.Errorf("nacos login failed: %s", nacosResponseMessage(body))
	}

	token, ttlRaw := payload.AccessToken, payload.TokenTTL
	if len(payload.Data) > 0 && string(payload.Data) != "null" {
		var data struct {
			AccessToken string          `json:"accessToken"`
			TokenTTL    json.RawMessage `json:"tokenTtl"`
		}
		if err := json.Unmarshal(payload.Data, &data); err == nil {
			if token == "" {
				token = data.AccessToken
			}
			if len(ttlRaw) == 0 {
				ttlRaw = data.TokenTTL
			}
		}
	}
	if token == "" {
		return "", 0, fmt.Errorf("nacos login response did not include accessToken")
	}
	return token, parseNacosTokenTTL(ttlRaw), nil
}

func nacosResponseFailed(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var code int
	if err := json.Unmarshal(raw, &code); err == nil {
		return code != 0 && code != http.StatusOK
	}
	var text string
	return json.Unmarshal(raw, &text) == nil && text != "" && text != "0" && text != "200"
}

func parseNacosTokenTTL(raw json.RawMessage) time.Duration {
	if len(raw) == 0 || string(raw) == "null" {
		return nacosTokenDefaultTTL
	}
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if seconds, err := strconv.ParseInt(text, 10, 64); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return nacosTokenDefaultTTL
}

func nacosResponseMessage(body []byte) string {
	var payload struct {
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		var detail string
		if json.Unmarshal(payload.Data, &detail) == nil && strings.TrimSpace(detail) != "" {
			return detail
		}
		if strings.TrimSpace(payload.Message) != "" {
			return payload.Message
		}
	}
	if text := strings.TrimSpace(string(body)); text != "" {
		return text
	}
	return "unknown error"
}

func effectivePort(raw *url.URL) int {
	if raw == nil {
		return 0
	}
	if port, err := strconv.Atoi(raw.Port()); err == nil && port > 0 {
		return port
	}
	if raw.Scheme == "https" {
		return 443
	}
	return 80
}
