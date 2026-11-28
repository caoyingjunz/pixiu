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
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// withPublicKeys 测试期替换公钥加载器，注入 kid -> 公钥 映射。
func withPublicKeys(t *testing.T, keys map[string]ed25519.PublicKey) {
	t.Helper()
	old := publicKeysLoader
	publicKeysLoader = func() (map[string]ed25519.PublicKey, error) { return keys, nil }
	t.Cleanup(func() { publicKeysLoader = old })
}

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	return pub, priv
}

// TestKeyRotationByKid 验证轮转期新旧公钥并存时按 kid 选对公钥验签。
func TestKeyRotationByKid(t *testing.T) {
	oldPub, oldPriv := genKey(t)
	newPub, newPriv := genKey(t)
	withPublicKeys(t, map[string]ed25519.PublicKey{"": oldPub, "2026": newPub})

	// 新 kid 用新私钥签发
	signed, err := SignPayload(newPriv, &Payload{Edition: EditionProfessional, KeyID: "2026"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyLicenseWithKeys(signed)
	if err != nil || got.Edition != EditionProfessional {
		t.Fatalf("kid=2026 验签失败: %v", err)
	}

	// 空 kid（旧 license）用主公钥仍可验
	legacy, err := SignPayload(oldPriv, &Payload{Edition: EditionEnterprise})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyLicenseWithKeys(legacy); err != nil {
		t.Fatalf("旧 license 验签失败: %v", err)
	}

	// kid 选错公钥 -> 验签失败（fail-closed）
	if _, err = VerifyLicense(newPub, legacy); err == nil {
		t.Fatal("错误公钥验签应当失败")
	}

	// 未知 kid -> 明确错误
	unknown, _ := SignPayload(newPriv, &Payload{Edition: EditionProfessional, KeyID: "unknown"})
	if _, err = VerifyLicenseWithKeys(unknown); !errors.Is(err, ErrUnknownKeyID) {
		t.Fatalf("未知 kid 应返回 ErrUnknownKeyID，实际: %v", err)
	}
}

// TestTamperedPayloadRejected 验证篡改 payload 后签名校验失败。
func TestTamperedPayloadRejected(t *testing.T) {
	pub, priv := genKey(t)
	signed, _ := SignPayload(priv, &Payload{Edition: EditionEnterprise})
	parts := strings.SplitN(string(signed), ".", 2)
	if len(parts) != 2 {
		t.Fatal("签发的 license 格式非法")
	}

	// 篡改 payload（把 enterprise 改成 community），签名段保持不变
	var p Payload
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(rawPayload, &p); err != nil {
		t.Fatal(err)
	}
	p.Edition = EditionCommunity
	mutated, _ := json.Marshal(p)
	tampered := base64.RawURLEncoding.EncodeToString(mutated) + "." + parts[1]

	if _, err = VerifyLicense(pub, []byte(tampered)); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("篡改 payload 应返回 ErrInvalidSignature，实际: %v", err)
	}
}

// TestSummaryNoDeadlock 验证 Summary 与 Reload 并发时不会因递归读锁死锁。
func TestSummaryNoDeadlock(t *testing.T) {
	pub, priv := genKey(t)
	withPublicKeys(t, map[string]ed25519.PublicKey{"": pub})
	signed, _ := SignPayload(priv, &Payload{Edition: EditionProfessional, MaxClusters: 5})

	dir := t.TempDir()
	path := dir + "/license.key"
	if err := writeFile(path, signed); err != nil {
		t.Fatal(err)
	}
	m := &Manager{path: path, state: StateCommunity}
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = m.Summary()
			_ = m.Quota()
			_ = m.HasFeature(FeatureBackup)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = m.Reload()
		}
	}()
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Summary/Reload 并发发生死锁")
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
