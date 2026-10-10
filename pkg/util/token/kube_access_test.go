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
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// 测试用固定输入与期望值（golden 值由独立实现计算得出，用于冻结算法行为）
const (
	testJWTKey      = "test-jwt-key-123"
	testOtherJWTKey = "other-jwt-key-456"
	testPlaintext   = "pxk_0123456789abcdef0123456789abcdef.dGVzdC1zZWNyZXQ"
	// sha256("pixiu/kube-access-token-seal/v1:" + testJWTKey)
	testSealKeyHex = "d035b833d42d511f04310464e9ef3c9c483b20974418f0e0c8995d6f134369ec"
	// HMAC-SHA256(testSealKey, "pixiu/kube-access-token/v1\x00" + testPlaintext)
	testHashHex = "28542103a6d0f430aff5a0b1e1a982ea0482a6569ee287e332a9d430e3c6860d"
	// 同一明文在不同 sealKey 下的 HMAC 结果（testOtherJWTKey 派生）
	testOtherKeyHashHex = "e4d46a0401c975225d5f7eae44e264fb63aaed9c5475839b1cbb5acb9bf1d1db"
)

func TestSealKeyFromJWTKey(t *testing.T) {
	sealKey := SealKeyFromJWTKey([]byte(testJWTKey))
	if got := hex.EncodeToString(sealKey); got != testSealKeyHex {
		t.Errorf("SealKeyFromJWTKey() = %s, want %s", got, testSealKeyHex)
	}

	// 确定性：同一 jwt_key 派生出相同 sealKey
	if second := SealKeyFromJWTKey([]byte(testJWTKey)); hex.EncodeToString(second) != testSealKeyHex {
		t.Error("SealKeyFromJWTKey() is not deterministic for the same jwt_key")
	}

	// 不同 jwt_key 派生出不同 sealKey
	if other := SealKeyFromJWTKey([]byte(testOtherJWTKey)); hex.EncodeToString(other) == testSealKeyHex {
		t.Error("SealKeyFromJWTKey() returned the same key for different jwt_key")
	}
}

func TestHashKubeAccessToken(t *testing.T) {
	sealKey := SealKeyFromJWTKey([]byte(testJWTKey))

	// 固定 key 下 HMAC 结果确定（golden hex）
	if got := HashKubeAccessToken(testPlaintext, sealKey); got != testHashHex {
		t.Errorf("HashKubeAccessToken() = %s, want %s", got, testHashHex)
	}

	// 重复计算保持一致
	if got := HashKubeAccessToken(testPlaintext, sealKey); got != testHashHex {
		t.Errorf("HashKubeAccessToken() not deterministic: got %s", got)
	}

	// 不同 key 结果不同（密钥参与哈希）
	otherKey := SealKeyFromJWTKey([]byte(testOtherJWTKey))
	if got := HashKubeAccessToken(testPlaintext, otherKey); got != testOtherKeyHashHex {
		t.Errorf("HashKubeAccessToken(otherKey) = %s, want %s", got, testOtherKeyHashHex)
	}

	// 与旧版裸 sha256 结果不同（不含历史哈希回退）
	sum := sha256.Sum256([]byte(testPlaintext))
	if got := HashKubeAccessToken(testPlaintext, sealKey); got == hex.EncodeToString(sum[:]) {
		t.Error("HashKubeAccessToken() equals legacy unsalted sha256 result")
	}
}

func TestGenerateKubeAccessToken(t *testing.T) {
	sealKey := SealKeyFromJWTKey([]byte(testJWTKey))

	plaintext, jti, tokenHash, err := GenerateKubeAccessToken(sealKey)
	if err != nil {
		t.Fatalf("GenerateKubeAccessToken() error = %v", err)
	}

	// 令牌格式保持 pxk_<jti>.<secret>
	if !IsKubeAccessToken(plaintext) {
		t.Errorf("token %q missing prefix %s", plaintext, KubeAccessTokenPrefix)
	}
	if !strings.HasPrefix(plaintext, KubeAccessTokenPrefix+jti+".") {
		t.Errorf("token %q does not embed jti %s", plaintext, jti)
	}
	if len(jti) != 32 { // 16 字节 hex
		t.Errorf("jti length = %d, want 32", len(jti))
	}

	// 哈希与同 key 下重新计算一致
	if tokenHash != HashKubeAccessToken(plaintext, sealKey) {
		t.Error("tokenHash does not match HashKubeAccessToken with the same sealKey")
	}
	if len(tokenHash) != 64 { // HMAC-SHA256 hex，列长度与旧版一致
		t.Errorf("tokenHash length = %d, want 64", len(tokenHash))
	}

	// 两次生成的令牌不相同
	plaintext2, _, _, err := GenerateKubeAccessToken(sealKey)
	if err != nil {
		t.Fatalf("GenerateKubeAccessToken() second call error = %v", err)
	}
	if plaintext == plaintext2 {
		t.Error("two generated tokens are identical")
	}
}

func TestIsKubeAccessToken(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		want      bool
	}{
		{name: "case1", plaintext: testPlaintext, want: true},
		{name: "case2", plaintext: KubeAccessTokenPrefix, want: true},
		{name: "case3", plaintext: "Bearer pxk_abc.def", want: false},
		{name: "case4", plaintext: "", want: false},
		{name: "case5", plaintext: "eyJhbGciOiJIUzI1NiJ9.x.y", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKubeAccessToken(tt.plaintext); got != tt.want {
				t.Errorf("IsKubeAccessToken(%q) = %v, want %v", tt.plaintext, got, tt.want)
			}
		})
	}
}
