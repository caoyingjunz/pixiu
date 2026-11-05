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
	"testing"
	"time"
)

func newTestKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	return pub, priv
}

func TestSignAndVerify(t *testing.T) {
	pub, priv := newTestKeys(t)

	p := &Payload{
		Customer:    "测试客户",
		Edition:     EditionProfessional,
		Features:    []Feature{FeatureBackup, FeatureAlert},
		MaxClusters: 10,
		MaxNodes:    100,
		ExpiresAt:   time.Now().Add(24 * time.Hour).Unix(),
	}
	signed, err := SignPayload(priv, p)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	got, err := VerifyLicense(pub, signed)
	if err != nil {
		t.Fatalf("验签失败: %v", err)
	}
	if got.Edition != EditionProfessional || got.Customer != "测试客户" {
		t.Fatalf("载荷内容不一致: %+v", got)
	}
	if !got.HasFeature(FeatureBackup) || got.HasFeature(FeatureAssistant) {
		t.Fatalf("功能判定错误: %+v", got.Features)
	}
}

func TestVerifyWithWrongKey(t *testing.T) {
	pub, _ := newTestKeys(t)
	_, priv := newTestKeys(t)

	signed, err := SignPayload(priv, &Payload{Edition: EditionEnterprise})
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	if _, err = VerifyLicense(pub, signed); err == nil {
		t.Fatal("换公钥后验签应当失败")
	}
}

func TestExpiredPayload(t *testing.T) {
	pub, priv := newTestKeys(t)

	signed, err := SignPayload(priv, &Payload{Edition: EditionProfessional, ExpiresAt: time.Now().Add(-time.Hour).Unix()})
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	got, err := VerifyLicense(pub, signed)
	if err != nil {
		t.Fatalf("验签失败: %v", err)
	}
	if !got.IsExpired() {
		t.Fatal("过期时间已过，IsExpired 应返回 true")
	}
}

func TestQuota(t *testing.T) {
	m := &Manager{state: StateValid, payload: &Payload{Edition: EditionProfessional, MaxClusters: 2}}

	if err := m.CheckClusterQuota(1); err != nil {
		t.Fatalf("未达上限不应报错: %v", err)
	}
	if err := m.CheckClusterQuota(2); err == nil {
		t.Fatal("达到上限应报错")
	}

	// 未授权 → 社区版默认配额
	community := &Manager{state: StateCommunity}
	q := community.Quota()
	if q.MaxClusters != DefaultCommunityMaxClusters || q.MaxNodes != DefaultCommunityMaxNodes {
		t.Fatalf("社区版默认配额错误: %+v", q)
	}

	// 无限制
	m.payload.MaxClusters = 0
	if err := m.CheckClusterQuota(10000); err != nil {
		t.Fatalf("0 表示不限制，不应报错: %v", err)
	}
}

func TestFingerprintBinding(t *testing.T) {
	// 当前环境指纹
	local := Fingerprint()
	if local == "" {
		t.Skip("无法获取机器指纹，跳过")
	}

	valid := &Payload{Edition: EditionProfessional, Fingerprint: local}
	if err := validate(valid); err != nil {
		t.Fatalf("指纹匹配不应报错: %v", err)
	}

	mismatch := &Payload{Edition: EditionProfessional, Fingerprint: "deadbeef"}
	if err := validate(mismatch); err == nil {
		t.Fatal("指纹不匹配应报错")
	}
}
