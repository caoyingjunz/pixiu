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

package adapter

import "testing"

// TestNewDispatch 守护工厂分派契约：
// aws/oss/cos/obs 必须路由到各自的原生适配器；s3/minio/未知/空取值回落 generic（minio-go 兜底）。
func TestNewDispatch(t *testing.T) {
	newAdapter := func(provider string) Adapter {
		t.Helper()
		a, err := New(Config{
			Provider:        provider,
			Endpoint:        "storage.example.com",
			Secure:          true,
			Region:          "us-east-1",
			AccessKeyID:     "ak",
			SecretAccessKey: "sk",
		})
		if err != nil {
			t.Fatalf("New(%q): %v", provider, err)
		}
		return a
	}

	native := map[string]func(Adapter) bool{
		"aws": func(a Adapter) bool { _, ok := a.(*awsAdapter); return ok },
		"oss": func(a Adapter) bool { _, ok := a.(*ossAdapter); return ok },
		"cos": func(a Adapter) bool { _, ok := a.(*cosAdapter); return ok },
		"obs": func(a Adapter) bool { _, ok := a.(*obsAdapter); return ok },
	}
	for provider, check := range native {
		if a := newAdapter(provider); !check(a) {
			t.Errorf("New(%q) 未路由到原生适配器（got %T）", provider, a)
		}
	}

	// 大小写 / 空白变体：归一化（ToLower + TrimSpace）后仍须路由到原生实现
	variants := map[string]string{
		"OSS":   "oss",
		" aws ": "aws",
		"COS":   "cos",
		" OBS ": "obs",
	}
	for variant, base := range variants {
		if a := newAdapter(variant); !native[base](a) {
			t.Errorf("New(%q) 未归一化路由到 %s 原生适配器（got %T）", variant, base, a)
		}
	}

	fallback := []string{"s3", "minio", "unknown-vendor", "", " S3 ", "MINIO", "Unknown-Vendor"}
	for _, provider := range fallback {
		if a := newAdapter(provider); !isGenericAdapter(a) {
			t.Errorf("New(%q) 应回落 generic（got %T）", provider, a)
		}
	}
}

// isGenericAdapter 判断是否 generic（minio-go）实现。
func isGenericAdapter(a Adapter) bool {
	_, ok := a.(*generic)
	return ok
}
