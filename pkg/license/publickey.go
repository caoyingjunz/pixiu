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

package license

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrPublicKeyNotConfigured 未注入验签公钥（商业授权未启用）。
var ErrPublicKeyNotConfigured = errors.New("license: 未配置验签公钥，商业授权未启用")

// ErrUnknownKeyID license 指定的 kid 无对应验签公钥。
var ErrUnknownKeyID = errors.New("license: 未知的公钥标识 kid")

// embeddedPublicKeyHex 默认（主）验签公钥（Ed25519，hex 编码），由构建期通过 ldflags 注入：
//
//	go build -ldflags "-X github.com/caoyingjunz/pixiu/pkg/license.embeddedPublicKeyHex=<hex>" ./cmd/pixiu-server
//
// 不在源码中硬编码可用公钥：公钥虽非机密，但一旦随源码附带默认密钥对，
// 其配套私钥也可能被复用，任何人都能自行签发企业版 license，使门控形同虚设。
// 默认留空时商业授权功能不可用，服务运行在社区版，不影响正常构建与启动。
var embeddedPublicKeyHex string

// embeddedPublicKeys 轮转用的多公钥集合，格式 `kid:hex,kid:hex`，同样由 ldflags 注入。
// 与 embeddedPublicKeyHex 可同时使用：主公钥对应空 kid，本集合用于平滑换证期并存的新旧公钥。
var embeddedPublicKeys string

var (
	publicKeysOnce sync.Once
	publicKeysMap  map[string]ed25519.PublicKey
	publicKeysErr  error
)

// publicKeysLoader 提供公钥集合；单元测试可替换为返回测试公钥的实现。
var publicKeysLoader = defaultPublicKeysLoader

// parseEmbeddedPublicKeys 返回按 kid 索引的公钥集合（空 kid 为主公钥）。
func parseEmbeddedPublicKeys() (map[string]ed25519.PublicKey, error) {
	return publicKeysLoader()
}

// defaultPublicKeysLoader 惰性解析 ldflags 注入的公钥（仅解析一次）。
func defaultPublicKeysLoader() (map[string]ed25519.PublicKey, error) {
	publicKeysOnce.Do(func() {
		keys := make(map[string]ed25519.PublicKey)
		mainHex := strings.TrimSpace(embeddedPublicKeyHex)
		if mainHex != "" {
			pub, err := ParsePublicKey(mainHex)
			if err != nil {
				publicKeysErr = fmt.Errorf("内嵌主验签公钥非法: %w", err)
				return
			}
			keys[""] = pub
		}
		for _, entry := range strings.Split(embeddedPublicKeys, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			kid, hexKey, ok := strings.Cut(entry, ":")
			if !ok || strings.TrimSpace(kid) == "" {
				publicKeysErr = fmt.Errorf("内嵌公钥条目格式非法（期望 kid:hex）: %s", entry)
				return
			}
			pub, err := ParsePublicKey(hexKey)
			if err != nil {
				publicKeysErr = fmt.Errorf("kid=%s 验签公钥非法: %w", kid, err)
				return
			}
			keys[strings.TrimSpace(kid)] = pub
		}
		if len(keys) == 0 {
			publicKeysErr = ErrPublicKeyNotConfigured
			return
		}
		publicKeysMap = keys
	})
	return publicKeysMap, publicKeysErr
}

// PublicKey 按 kid 返回对应验签公钥；kid 为空时返回主公钥。
func PublicKey(kid string) (ed25519.PublicKey, error) {
	keys, err := parseEmbeddedPublicKeys()
	if err != nil {
		return nil, err
	}
	pub, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKeyID, kid)
	}
	return pub, nil
}

// EmbeddedPublicKey 返回默认（主）验签公钥，保持单公钥场景的兼容调用。
func EmbeddedPublicKey() (ed25519.PublicKey, error) {
	return PublicKey("")
}

// AllPublicKeys 返回全部已注入公钥（供单测/诊断），键为 kid。
func AllPublicKeys() (map[string]ed25519.PublicKey, error) {
	return parseEmbeddedPublicKeys()
}
