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

package storage

import "testing"

// TestProviderCapabilitiesMatrix 守护四家原生适配器就绪后的能力位契约：
// 六家厂商（s3/minio/aws/oss/cos/obs）的策略 / 默认加密 / 标签能力全部开启，
// 且 API 下发（ProviderCapabilities）与注册表一致。任何回退都会被本用例拦下。
func TestProviderCapabilitiesMatrix(t *testing.T) {
	want := map[string]Capabilities{
		"s3":    {BucketPolicy: true, SSEEncryption: true, BucketTagging: true},
		"minio": {BucketPolicy: true, SSEEncryption: true, BucketTagging: true},
		"aws":   {BucketPolicy: true, SSEEncryption: true, BucketTagging: true},
		"oss":   {BucketPolicy: true, SSEEncryption: true, BucketTagging: true},
		"cos":   {BucketPolicy: true, SSEEncryption: true, BucketTagging: true},
		"obs":   {BucketPolicy: true, SSEEncryption: true, BucketTagging: true},
	}

	keys := providerKeys()
	if len(keys) != len(want) {
		t.Fatalf("注册表键数 = %d, want %d: %v", len(keys), len(want), keys)
	}
	for _, key := range keys {
		wantCaps, ok := want[key]
		if !ok {
			t.Fatalf("注册表出现未知厂商 %q", key)
		}
		if got := providers[key].Capabilities(); got != wantCaps {
			t.Errorf("%s: Capabilities = %+v, want %+v", key, got, wantCaps)
		}
	}

	// 下发面：ProviderCapabilities 与注册表逐项一致（前端能力位全驱动依赖此契约）
	infos := ProviderCapabilities()
	if len(infos) != len(keys) {
		t.Fatalf("ProviderCapabilities 条数 = %d, want %d", len(infos), len(keys))
	}
	for _, info := range infos {
		caps := providers[info.Key].Capabilities()
		if info.Capabilities.BucketPolicy != caps.BucketPolicy ||
			info.Capabilities.SSEEncryption != caps.SSEEncryption ||
			info.Capabilities.BucketTagging != caps.BucketTagging {
			t.Errorf("%s: 下发能力位 %+v 与注册表 %+v 不一致", info.Key, info.Capabilities, caps)
		}
	}
}
