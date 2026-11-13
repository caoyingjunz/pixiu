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

// registerpixiuversion 把一次发布的 pixiu 版本元数据注册到 Hub（rainbow）的版本管理接口。
//
// 为什么需要它：pixiu 的升级检查依赖 Hub 的 GET /api/v2/pixiu/versions/latest，只有 Hub 里
// 存在版本记录，集群侧才能探测到新版本并触发升级。发布流水线在镜像推送完成后调用本工具，
// 取代人工在页面上重复录入版本号、commit 与镜像地址。
//
// 鉴权：Hub 的 /api/v2 在携带 AK（非 Bearer）时走 v2 签名，请求头需带
// X-ACCESS-KEY / X-TIMESTAMP / X-Rainbow-Sign-Version: 2 / X-Rainbow-Nonce，
// 裸签名（无 Bearer 前缀）放 Authorization。签名口径必须与 rainbow
// pkg/util/signatureutil 完全一致，否则一律 401：
//
//	message   = 参与字段按 key 字典序拼成 k=v& 连接
//	            字段固定为 accessKey/bodyHash/method/nonce/path/query/timestamp
//	            bodyHash = hex(sha256(body))，method 大写，query 为规范化后的原始串（本工具无 query）
//	signature = hex(HMAC-SHA256(secretKey, message))
//
// 用法（环境变量）：
//
//	RAINBOW_ENDPOINT   Hub 地址，默认 https://hub.pixiuio.com
//	RAINBOW_AK         管理员 AccessKey（普通用户 AK 会被 403 拒绝）
//	RAINBOW_SK         对应的 SecretKey
//	VERSION_TAG        版本号，需匹配 ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$
//	IMAGE              版本对应的镜像全名（含 tag）
//	COMMIT             版本对应的 commit（短 hash 即可）
//	RELEASE_TIME       发布时间，RFC3339；留空由服务端取当前时间
//	DESCRIPTION        版本说明
//
// 退出码：0 表示注册成功或版本已存在且元数据已同步；非 0 表示失败（打印服务端 message）。
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// signVersion Hub 当前的签名版本，升级协议时同步调整
	signVersion = "2"

	headerAccessKey   = "X-ACCESS-KEY"
	headerTimestamp   = "X-TIMESTAMP"
	headerSignVersion = "X-Rainbow-Sign-Version"
	headerNonce       = "X-Rainbow-Nonce"

	versionPath = "/api/v2/pixiu/versions"

	defaultEndpoint = "https://hub.pixiuio.com"
	requestTimeout  = 30 * time.Second
	maxBodyBytes    = 1 << 20
)

// versionPayload 与 Hub 的 CreatePixiuVersionRequest 字段对齐。
// 更新接口（PUT）忽略 version 字段（版本号来自 URI），多带无副作用。
type versionPayload struct {
	Version     string `json:"version"`
	Image       string `json:"image"`
	Commit      string `json:"commit"`
	ReleaseTime string `json:"release_time"`
	Description string `json:"description"`
}

// apiResponse 是 Hub 的统一响应封装：业务错误也是 HTTP 200，真实结果看 code。
type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type config struct {
	endpoint  string
	accessKey string
	secretKey string
	payload   versionPayload
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "注册 pixiu 版本失败: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	body, err := json.Marshal(cfg.payload)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %v", err)
	}

	resp, err := cfg.do(http.MethodPost, versionPath, body)
	if err != nil {
		return err
	}
	if resp.Code == http.StatusOK {
		fmt.Printf("已创建 pixiu 版本 %s（image=%s commit=%s）\n",
			cfg.payload.Version, cfg.payload.Image, cfg.payload.Commit)
		return nil
	}

	// 发布流水线可重跑：版本已存在时改为同步元数据（镜像地址/commit/发布时间/说明），
	// 而不是直接失败，避免一次重试就把 CI 标红。
	if !strings.Contains(resp.Message, "已存在") {
		return fmt.Errorf("创建版本被拒绝：%s", resp.Message)
	}
	fmt.Printf("版本 %s 已存在，改为更新元数据\n", cfg.payload.Version)

	updateBody, err := json.Marshal(cfg.payload)
	if err != nil {
		return fmt.Errorf("序列化更新请求体失败: %v", err)
	}
	resp, err = cfg.do(http.MethodPut, versionPath+"/"+url.PathEscape(cfg.payload.Version), updateBody)
	if err != nil {
		return err
	}
	if resp.Code != http.StatusOK {
		return fmt.Errorf("更新版本被拒绝：%s", resp.Message)
	}
	fmt.Printf("已更新 pixiu 版本 %s 的元数据\n", cfg.payload.Version)
	return nil
}

func loadConfig() (*config, error) {
	cfg := &config{
		endpoint:  strings.TrimSuffix(strings.TrimSpace(env("RAINBOW_ENDPOINT", defaultEndpoint)), "/"),
		accessKey: strings.TrimSpace(os.Getenv("RAINBOW_AK")),
		secretKey: strings.TrimSpace(os.Getenv("RAINBOW_SK")),
		payload: versionPayload{
			Version:     strings.TrimSpace(os.Getenv("VERSION_TAG")),
			Image:       strings.TrimSpace(os.Getenv("IMAGE")),
			Commit:      strings.TrimSpace(os.Getenv("COMMIT")),
			ReleaseTime: strings.TrimSpace(os.Getenv("RELEASE_TIME")),
			Description: strings.TrimSpace(os.Getenv("DESCRIPTION")),
		},
	}

	if cfg.accessKey == "" || cfg.secretKey == "" {
		return nil, fmt.Errorf("缺少 RAINBOW_AK / RAINBOW_SK（需管理员 AK，普通用户 AK 会被 403 拒绝）")
	}
	if cfg.payload.Version == "" {
		return nil, fmt.Errorf("缺少 VERSION_TAG")
	}
	if cfg.payload.Image == "" {
		return nil, fmt.Errorf("缺少 IMAGE")
	}
	if cfg.payload.Commit == "" {
		return nil, fmt.Errorf("缺少 COMMIT")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// do 发起一次带 v2 签名的请求，返回解析后的响应体。
// 网络/解码错误返回 error；业务错误（code != 200）由调用方按 message 分支处理。
func (c *config) do(method, path string, body []byte) (*apiResponse, error) {
	req, err := http.NewRequest(method, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %v", err)
	}

	signature, timestamp, nonce := signV2(c.accessKey, c.secretKey, method, path, "", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerAccessKey, c.accessKey)
	req.Header.Set(headerTimestamp, timestamp)
	req.Header.Set(headerSignVersion, signVersion)
	req.Header.Set(headerNonce, nonce)
	req.Header.Set("Authorization", signature)

	client := &http.Client{Timeout: requestTimeout}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %v", path, err)
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 %s 返回 HTTP %d: %s", path, httpResp.StatusCode, truncate(string(data)))
	}

	resp := &apiResponse{}
	if err = json.Unmarshal(data, resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v（原文 %s）", err, truncate(string(data)))
	}
	return resp, nil
}

// signV2 按 Hub 的 v2 协议生成签名、时间戳与 nonce。
// 每次请求（含重试）都必须重新生成，时间窗 ±5 分钟、nonce 服务端防重放。
func signV2(accessKey, secretKey, method, path, rawQuery string, body []byte) (signature, timestamp, nonce string) {
	nonce = newNonce()
	timestamp = strconv.FormatInt(time.Now().Unix(), 10)
	signature = signWith(accessKey, secretKey, method, path, rawQuery, body, timestamp, nonce)
	return signature, timestamp, nonce
}

// signWith 用给定的时间戳与 nonce 计算签名，供单测锁定算法口径（生产调用走 signV2）。
func signWith(accessKey, secretKey, method, path, rawQuery string, body []byte, timestamp, nonce string) string {
	params := map[string]string{
		"accessKey": accessKey,
		"bodyHash":  hashBody(body),
		"method":    strings.ToUpper(method),
		"nonce":     nonce,
		"path":      path,
		"query":     canonicalQuery(rawQuery),
		"timestamp": timestamp,
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var message strings.Builder
	for i, k := range keys {
		if i > 0 {
			message.WriteString("&")
		}
		message.WriteString(k)
		message.WriteString("=")
		message.WriteString(params[k])
	}

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// hashBody 与 Hub 保持一致：空 body 也返回空串的 sha256。
func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// canonicalQuery 按 key 字典序（同 key 多值再按值排序）重排 query。
// 本工具不带 query，保留实现是为了与 Hub 口径完全对齐。
func canonicalQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	for k := range values {
		sort.Strings(values[k])
	}
	return values.Encode()
}

// newNonce 生成 UUID v4 作为一次性随机数；随机源不可用时退化为时间戳，
// 仅影响防重放的严格程度，不阻断发布。
func newNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func truncate(s string) string {
	const limit = 512
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
