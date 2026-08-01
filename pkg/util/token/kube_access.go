/*
Copyright 2024 The Pixiu Authors.

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

package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const KubeAccessTokenPrefix = "pxk_"

// GenerateKubeAccessToken 生成 opaque 访问令牌：pxk_<jti>.<secret>
// 返回 plaintext、jti、tokenHash(sha256 hex)。
func GenerateKubeAccessToken() (plaintext, jti, tokenHash string, err error) {
	jtiBytes := make([]byte, 16)
	if _, err = rand.Read(jtiBytes); err != nil {
		return "", "", "", err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", err
	}

	jti = hex.EncodeToString(jtiBytes)
	secretEnc := base64.RawURLEncoding.EncodeToString(secret)
	plaintext = KubeAccessTokenPrefix + jti + "." + secretEnc
	tokenHash = HashKubeAccessToken(plaintext)
	return plaintext, jti, tokenHash, nil
}

func HashKubeAccessToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func IsKubeAccessToken(plaintext string) bool {
	return strings.HasPrefix(plaintext, KubeAccessTokenPrefix)
}

func ParseKubeAccessTokenJTI(plaintext string) (string, error) {
	if !IsKubeAccessToken(plaintext) {
		return "", fmt.Errorf("invalid kube access token prefix")
	}
	body := strings.TrimPrefix(plaintext, KubeAccessTokenPrefix)
	parts := strings.SplitN(body, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid kube access token format")
	}
	return parts[0], nil
}
