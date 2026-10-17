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

import (
	"encoding/json"
	"strings"
	"testing"
)

// Build→Classify 往返：默认（S3 词汇）实现生成的两档公开策略必须可逆归类。
// 所有 provider 经嵌入继承默认实现，此用例同时守护「能力位 + 词汇」契约的默认面
// （P2~P5 各家覆盖词汇后仍须满足往返可逆）。
func TestAnonymousPolicyRoundTrip(t *testing.T) {
	for _, key := range providerKeys() {
		p := providers[key]
		for _, acl := range []string{"public-read", "public-read-write"} {
			text := p.BuildAnonymousPolicy("demo-bucket", "us-east-1", acl)
			if got := p.ClassifyAnonymousPolicy(text); got != acl {
				t.Errorf("%s: BuildAnonymousPolicy(%q) 往返 classify = %q, want %q", key, acl, got, acl)
			}
		}

		// 空策略 / 非法 JSON 均按最小权限处理
		if got := p.ClassifyAnonymousPolicy(""); got != "private" {
			t.Errorf("%s: 空策略 classify = %q, want private", key, got)
		}
		if got := p.ClassifyAnonymousPolicy("{not-json"); got != "private" {
			t.Errorf("%s: 非法 JSON classify = %q, want private", key, got)
		}

		// 指定 Principal 的授权不算匿名开放
		scoped := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::1:root"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::demo-bucket/*"}]}`
		if got := p.ClassifyAnonymousPolicy(scoped); got != "private" {
			t.Errorf("%s: 指定 Principal classify = %q, want private", key, got)
		}
	}
}

// TestCosAnonymousPolicyVocabulary COS 词汇表：文档样例归类 + build 资源路径格式。
func TestCosAnonymousPolicyVocabulary(t *testing.T) {
	p := providers[cosProviderKey]

	// 官方文档样例（小写 effect / name/cos: 前缀 / qcs anyone 身份）
	doc := `{
		"version": "2.0",
		"Statement": [{
			"Principal": {"qcs": ["qcs::cam::anyone:anyone"]},
			"Effect": "allow",
			"Action": ["name/cos:GetObject"],
			"Resource": ["qcs::cos:ap-guangzhou:uid/1250000000:test-12500000000/*"]
		}]
	}`
	if got := p.ClassifyAnonymousPolicy(doc); got != "public-read" {
		t.Errorf("COS 文档样例 classify = %q, want public-read", got)
	}

	// 读写两档：cos: 前缀（无 name/）同样识别
	rw := `{"version":"2.0","Statement":[{"Principal":{"qcs":["qcs::cam::anyone:anyone"]},"Effect":"Allow","Action":["cos:GetObject","cos:PutObject","cos:DeleteObject"],"Resource":["qcs::cos:ap-beijing:uid/1250000000:test-1250000000/*"]}]}`
	if got := p.ClassifyAnonymousPolicy(rw); got != "public-read-write" {
		t.Errorf("COS 读写策略 classify = %q, want public-read-write", got)
	}

	// 指定 qcs 身份（非匿名）不算开放
	scoped := `{"version":"2.0","Statement":[{"Principal":{"qcs":["qcs::cam::uin/100000000001:uin/100000000011"]},"Effect":"allow","Action":["name/cos:GetObject"],"Resource":["qcs::cos:ap-guangzhou:uid/1250000000:test-1250000000/*"]}]}`
	if got := p.ClassifyAnonymousPolicy(scoped); got != "private" {
		t.Errorf("COS 指定身份 classify = %q, want private", got)
	}

	// build 的资源路径：region + appid（从 bucket 名提取）必须落到 qcs 资源里
	text := p.BuildAnonymousPolicy("test-1250000000", "ap-guangzhou", "public-read")
	var built cosPolicyDoc
	if err := json.Unmarshal([]byte(text), &built); err != nil {
		t.Fatalf("build 输出不是合法 JSON: %v", err)
	}
	wantResource := "qcs::cos:ap-guangzhou:uid/1250000000:test-1250000000/*"
	if len(built.Statement) != 1 || len(built.Statement[0].Resource) != 1 || built.Statement[0].Resource[0] != wantResource {
		t.Errorf("build 资源 = %+v, want %q", built.Statement, wantResource)
	}
	if built.Version != "2.0" {
		t.Errorf("build version = %q, want 2.0", built.Version)
	}

	// bucket 名缺 appid 后缀（历史数据）：资源退化为 uid/* 通配
	fallback := p.BuildAnonymousPolicy("legacy-bucket", "", "public-read")
	if !strings.Contains(fallback, "qcs::cos:*:uid/*:legacy-bucket/*") {
		t.Errorf("无 appid 的 build 资源未退化通配: %s", fallback)
	}
}

// TestOSSAnonymousPolicyVocabulary OSS 词汇表：文档样例归类 + build 文档格式。
func TestOSSAnonymousPolicyVocabulary(t *testing.T) {
	p := providers[ossProviderKey]

	// 官方文档样例（Version 固定 1、Principal 为字符串 *、oss: 动作前缀）
	doc := `{
		"Version": "1",
		"Statement": [{
			"Effect": "Allow",
			"Principal": "*",
			"Action": ["oss:GetObject"],
			"Resource": ["acs:oss:*:*:my-bucket-123/*"]
		}]
	}`
	if got := p.ClassifyAnonymousPolicy(doc); got != "public-read" {
		t.Errorf("OSS 文档样例 classify = %q, want public-read", got)
	}

	// 全量动作（oss:*）与数组形式 Principal 均识别为读写
	rw := `{"Version":"1","Statement":[{"Effect":"Allow","Principal":["*"],"Action":["oss:*"],"Resource":["acs:oss:*:*:my-bucket-123/*"]}]}`
	if got := p.ClassifyAnonymousPolicy(rw); got != "public-read-write" {
		t.Errorf("OSS oss:* classify = %q, want public-read-write", got)
	}

	// 指定 UID 的授权不算匿名开放
	scoped := `{"Version":"1","Statement":[{"Effect":"Allow","Principal":["20214760404935xxxx"],"Action":["oss:GetObject"],"Resource":["acs:oss:*:*:my-bucket-123/*"]}]}`
	if got := p.ClassifyAnonymousPolicy(scoped); got != "private" {
		t.Errorf("OSS 指定 UID classify = %q, want private", got)
	}

	// build 文档：Version=1 + 资源为 acs:oss:*:*:<bucket>/*
	text := p.BuildAnonymousPolicy("my-bucket-123", "cn-hangzhou", "public-read-write")
	var built ossPolicyDoc
	if err := json.Unmarshal([]byte(text), &built); err != nil {
		t.Fatalf("build 输出不是合法 JSON: %v", err)
	}
	if built.Version != "1" {
		t.Errorf("build version = %q, want 1", built.Version)
	}
	wantResource := "acs:oss:*:*:my-bucket-123/*"
	if len(built.Statement) != 1 || len(built.Statement[0].Resource) != 1 || built.Statement[0].Resource[0] != wantResource {
		t.Errorf("build 资源 = %+v, want %q", built.Statement, wantResource)
	}
	if built.Statement[0].Principal != "*" {
		t.Errorf("build principal = %q, want *", built.Statement[0].Principal)
	}
}

// TestOBSAnonymousPolicyVocabulary OBS 词汇表：S3 格式与 OBS 原生格式双兼容 + build 文档格式。
func TestOBSAnonymousPolicyVocabulary(t *testing.T) {
	p := providers[obsProviderKey]

	// S3 格式样例（OBS API 参考即为该写法）
	s3Doc := `{
		"Version": "2008-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::mybucket/*"]
		}]
	}`
	if got := p.ClassifyAnonymousPolicy(s3Doc); got != "public-read" {
		t.Errorf("OBS S3 格式 classify = %q, want public-read", got)
	}

	// OBS 原生格式（Action 无前缀、Principal ID 形态）同样识别
	obsDoc := `{
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"ID": ["*"]},
			"Action": ["GetObject", "PutObject"],
			"Resource": ["mybucket/*"]
		}]
	}`
	if got := p.ClassifyAnonymousPolicy(obsDoc); got != "public-read-write" {
		t.Errorf("OBS 原生格式 classify = %q, want public-read-write", got)
	}

	// 指定具体账号的授权不算匿名开放
	scoped := `{"Version":"2008-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::1:root"},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::mybucket/*"]}]}`
	if got := p.ClassifyAnonymousPolicy(scoped); got != "private" {
		t.Errorf("OBS 指定账号 classify = %q, want private", got)
	}

	// build 使用 S3 格式（与前端模板语言一致）
	text := p.BuildAnonymousPolicy("mybucket", "cn-north-4", "public-read")
	var built bucketPolicyDoc
	if err := json.Unmarshal([]byte(text), &built); err != nil {
		t.Fatalf("build 输出不是合法 JSON: %v", err)
	}
	if built.Version != "2008-10-17" {
		t.Errorf("build version = %q, want 2008-10-17", built.Version)
	}
	if len(built.Statement) != 1 {
		t.Fatalf("build statements = %+v", built.Statement)
	}
	resources := flattenPolicyValues(built.Statement[0].Resource)
	if len(resources) != 1 || resources[0] != "arn:aws:s3:::mybucket/*" {
		t.Errorf("build 资源 = %+v", resources)
	}
	if !containsAny(flattenPolicyValues(built.Statement[0].Principal), "*") {
		t.Errorf("build principal 未命中匿名: %+v", built.Statement[0].Principal)
	}
	// build→classify 往返
	if got := p.ClassifyAnonymousPolicy(text); got != "public-read" {
		t.Errorf("OBS build→classify 往返 = %q, want public-read", got)
	}
}
