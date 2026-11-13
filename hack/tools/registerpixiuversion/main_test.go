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

package main

import "testing"

// TestSignWithMatchesHub 锁定签名口径。
// 期望值由 Hub 侧（rainbow pkg/util/signatureutil）用同一组入参算出，
// 任何字段拼接口径变化（排序、大小写、bodyHash、query 规范化）都会让这里失败——
// 口径不一致在线上表现为 401，所以必须有测试兜住。
func TestSignWithMatchesHub(t *testing.T) {
	const (
		ak      = "test-ak"
		sk      = "test-sk"
		nonce   = "0f8fad5b-d9cb-469f-a165-70867728950e"
		ts      = "1762996800"
		method  = "POST"
		path    = "/api/v2/pixiu/versions"
		rawQ    = ""
		wantSig = "9abcecb263e0e88b8d28209364758a9f94432cb35240cb2f360f39bd1a65de6a"
		wantSum = "284888dd597248a4f304817b333f1dcb9bcdd1131e88f075eeb516f5056feaa2"
	)
	body := []byte(`{"version":"v2.0.2-beta.2","image":"reg/pixiu:v2.0.2-beta.2","commit":"1d7b6a87","release_time":"2026-11-13T09:20:00+08:00","description":"Release pixiu v2.0.2-beta.2 (#715)"}`)

	if got := hashBody(body); got != wantSum {
		t.Fatalf("bodyHash 不一致: got %s, want %s", got, wantSum)
	}
	if got := signWith(ak, sk, method, path, rawQ, body, ts, nonce); got != wantSig {
		t.Fatalf("签名不一致: got %s, want %s", got, wantSig)
	}
}

// TestNewNonce 保证 nonce 为 UUID v4 格式且每次不同（服务端按 nonce 防重放，重复会被拒）。
func TestNewNonce(t *testing.T) {
	first, second := newNonce(), newNonce()
	if first == second {
		t.Fatalf("nonce 重复: %s", first)
	}
	if len(first) != 36 || first[14] != '4' {
		t.Fatalf("nonce 不是 UUID v4: %s", first)
	}
}

// TestCanonicalQuery 与 Hub 口径一致：按 key 字典序重排，空 query 返回空串。
func TestCanonicalQuery(t *testing.T) {
	if got := canonicalQuery(""); got != "" {
		t.Fatalf("空 query 应返回空串, got %q", got)
	}
	if got, want := canonicalQuery("b=2&a=1"), "a=1&b=2"; got != want {
		t.Fatalf("query 规范化结果不对: got %q, want %q", got, want)
	}
}
