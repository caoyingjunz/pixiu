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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// license 文件编码格式：<base64url(payload JSON)>.<base64url(signature)>
const licenseSeparator = "."

var (
	ErrInvalidFormat     = errors.New("license 格式非法，期望 <payload>.<signature> 编码")
	ErrInvalidSignature  = errors.New("license 签名验证失败")
	ErrMalformedPayload  = errors.New("license 载荷解析失败")
	ErrPrivateKeyInvalid = errors.New("私钥格式非法，期望 64 字节 ed25519 私钥（hex 或 base64url）")
	ErrPublicKeyInvalid  = errors.New("公钥格式非法，期望 32 字节 ed25519 公钥（hex 或 base64url）")
)

// decodeKey 兼容 hex 与 base64url 两种密钥编码
func decodeKey(s string, size int) ([]byte, error) {
	s = strings.TrimSpace(s)
	if raw, err := hex.DecodeString(s); err == nil && len(raw) == size {
		return raw, nil
	}
	if raw, err := base64.RawURLEncoding.DecodeString(s); err == nil && len(raw) == size {
		return raw, nil
	}
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil && len(raw) == size {
		return raw, nil
	}
	return nil, fmt.Errorf("密钥长度或编码错误，期望 %d 字节", size)
}

// ParsePrivateKey 解析 ed25519 私钥（hex 或 base64url 编码，64 字节 seed+pub）
func ParsePrivateKey(s string) (ed25519.PrivateKey, error) {
	raw, err := decodeKey(s, ed25519.PrivateKeySize)
	if err != nil {
		return nil, ErrPrivateKeyInvalid
	}
	return ed25519.PrivateKey(raw), nil
}

// ParsePublicKey 解析 ed25519 公钥（hex 或 base64url 编码，32 字节）
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	raw, err := decodeKey(s, ed25519.PublicKeySize)
	if err != nil {
		return nil, ErrPublicKeyInvalid
	}
	return ed25519.PublicKey(raw), nil
}

// SignPayload 对载荷签名，返回 license 文件内容（一行文本）
func SignPayload(priv ed25519.PrivateKey, p *Payload) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, ErrPrivateKeyInvalid
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("序列化 license 载荷失败: %w", err)
	}
	sig := ed25519.Sign(priv, payload)

	encoded := base64.RawURLEncoding.EncodeToString(payload) + licenseSeparator +
		base64.RawURLEncoding.EncodeToString(sig)
	return []byte(encoded), nil
}

// VerifyLicense 验证 license 文件内容并解析出载荷。
// 依次校验：编码格式 → 签名 → 载荷 JSON。
func VerifyLicense(pub ed25519.PublicKey, data []byte) (*Payload, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, ErrPublicKeyInvalid
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, ErrInvalidFormat
	}

	parts := strings.SplitN(text, licenseSeparator, 2)
	if len(parts) != 2 {
		return nil, ErrInvalidFormat
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: payload base64 解码失败", ErrMalformedPayload)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: signature base64 解码失败", ErrMalformedPayload)
	}
	if !ed25519.Verify(pub, payload, sig) {
		return nil, ErrInvalidSignature
	}

	var p Payload
	if err = json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
	}
	return &p, nil
}
