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

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// newAWSTestAdapter 起一个假 S3 端点（IP 形态端点走 path-style，请求直达测试服务器）。
func newAWSTestAdapter(t *testing.T, handler http.HandlerFunc) Adapter {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse httptest url: %v", err)
	}
	a, err := New(Config{
		Provider:        "aws",
		Endpoint:        u.Host, // IP:port → path-style
		Secure:          false,
		Region:          "us-east-1",
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "SECRETEXAMPLE",
	})
	if err != nil {
		t.Fatalf("newAWS: %v", err)
	}
	return a
}

func TestAWSNeedsPathStyle(t *testing.T) {
	cases := []struct {
		endpoint string
		want     bool
	}{
		{"127.0.0.1:9000", true},
		{"localhost:9000", true},
		{"[::1]:9000", true},
		{"s3.us-east-1.amazonaws.com", false},
		{"minio.internal.example.com:9000", false},
	}
	for _, c := range cases {
		if got := awsNeedsPathStyle(c.endpoint); got != c.want {
			t.Errorf("awsNeedsPathStyle(%q) = %v, want %v", c.endpoint, got, c.want)
		}
	}
}

func TestAWSCopySourceEncoding(t *testing.T) {
	src := CopySrc{Bucket: "demo", Object: "dir 目录/a b.txt"}
	if got := awsCopySource(src); got != "demo/dir%20%E7%9B%AE%E5%BD%95/a%20b.txt" {
		t.Errorf("awsCopySource = %q", got)
	}
	withVersion := CopySrc{Bucket: "demo", Object: "a.txt", VersionID: "v1 x"}
	if got := awsCopySource(withVersion); got != "demo/a.txt?versionId=v1+x" {
		t.Errorf("awsCopySource(version) = %q", got)
	}
}

func TestAWSListBucketsAgainstFakeEndpoint(t *testing.T) {
	var gotHost, gotPath string
	a := newAWSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotPath = r.Host, r.URL.Path
		if r.Header.Get("Authorization") == "" {
			t.Errorf("缺少 SigV4 Authorization 头")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListAllMyBucketsResult>
			<Owner><ID>owner-1</ID></Owner>
			<Buckets><Bucket>
				<Name>demo-bucket</Name>
				<CreationDate>2024-01-02T03:04:05.000Z</CreationDate>
			</Bucket></Buckets>
		</ListAllMyBucketsResult>`))
	})

	buckets, err := a.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	// 服务级请求为根路径（无 bucket 子域/前缀）
	if gotPath != "/" || strings.Contains(gotHost, "demo-bucket") {
		t.Errorf("请求 Host=%q Path=%q，期望根路径", gotHost, gotPath)
	}
	if len(buckets) != 1 || buckets[0].Name != "demo-bucket" || buckets[0].CreationDate.IsZero() {
		t.Fatalf("buckets = %+v", buckets)
	}
}

func TestAWSPathStyleBucketRequest(t *testing.T) {
	var gotPath string
	a := newAWSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListBucketResult><Name>demo-bucket</Name><IsTruncated>false</IsTruncated>
			<Contents><Key>k1</Key><Size>3</Size><ETag>"e1"</ETag></Contents></ListBucketResult>`))
	})

	page, err := a.ListObjectsV2Page(context.Background(), "demo-bucket", "", "", "", 10)
	if err != nil {
		t.Fatalf("ListObjectsV2Page: %v", err)
	}
	// path-style：bucket 在路径而非域名
	if gotPath != "/demo-bucket" {
		t.Errorf("请求路径 = %q, want /demo-bucket（path-style）", gotPath)
	}
	if len(page.Contents) != 1 || page.Contents[0].Key != "k1" || page.Contents[0].ETag != "e1" {
		t.Fatalf("contents = %+v", page.Contents)
	}
}

func TestAWSGetBucketPolicyAbsent(t *testing.T) {
	a := newAWSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<Error><Code>NoSuchBucketPolicy</Code><Message>no policy</Message><RequestId>r1</RequestId></Error>`))
	})

	policy, err := a.GetBucketPolicy(context.Background(), "demo-bucket")
	if err != nil {
		t.Fatalf("无策略应归一化为空值，got err: %v", err)
	}
	if policy != "" {
		t.Errorf("policy = %q, want empty", policy)
	}
}

func TestAWSDeleteObjectsBatch(t *testing.T) {
	a := newAWSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<DeleteResult>
			<Deleted><Key>a.txt</Key></Deleted>
			<Error><Key>b.txt</Key><Code>AccessDenied</Code><Message>denied</Message></Error>
		</DeleteResult>`))
	})

	refs := make(chan DeleteRef, 2)
	refs <- DeleteRef{Key: "a.txt"}
	refs <- DeleteRef{Key: "b.txt"}
	close(refs)

	var failures []DeleteError
	for item := range a.DeleteObjectsBatch(context.Background(), "demo-bucket", refs) {
		failures = append(failures, item)
	}
	if len(failures) != 1 || failures[0].Key != "b.txt" {
		t.Fatalf("failures = %+v, want 单条 b.txt", failures)
	}
	if ErrorCode(failures[0].Err) != "AccessDenied" {
		t.Errorf("failure code = %q", ErrorCode(failures[0].Err))
	}
}

// TestAWSWiringWithS3CompatibleServer 是接线级端到端验证（真实签名 + 真实服务器）：
// 默认跳过，设置以下环境变量后运行（本地 MinIO / S3Proxy 等 S3 兼容端点均可）：
//
//	PIXIU_STORAGE_AWS_WIRING_ENDPOINT=http://127.0.0.1:19000
//	PIXIU_STORAGE_AWS_WIRING_AK=<access key>
//	PIXIU_STORAGE_AWS_WIRING_SK=<secret key>
//
// 覆盖：ListBuckets / CreateBucket / PutObject / StatObject / GetObject / PresignGetObject
// （预签名 URL 直接经 HTTP 取回）/ ListObjects（流式与分页）/ DeleteObjectsBatch / DeleteBucket。
func TestAWSWiringWithS3CompatibleServer(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("PIXIU_STORAGE_AWS_WIRING_ENDPOINT"))
	if endpoint == "" {
		t.Skip("未设置 PIXIU_STORAGE_AWS_WIRING_ENDPOINT，跳过接线级验证")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		t.Fatalf("invalid PIXIU_STORAGE_AWS_WIRING_ENDPOINT: %q", endpoint)
	}
	a, err := New(Config{
		Provider:        "aws",
		Endpoint:        u.Host,
		Secure:          u.Scheme == "https",
		Region:          "us-east-1",
		AccessKeyID:     os.Getenv("PIXIU_STORAGE_AWS_WIRING_AK"),
		SecretAccessKey: os.Getenv("PIXIU_STORAGE_AWS_WIRING_SK"),
	})
	if err != nil {
		t.Fatalf("newAWS: %v", err)
	}
	// 真实拨号：复用控制层同款 SSRF 安全传输层（此处未用 probe 场景，保持默认传输即可）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const bucket = "pixiu-wiring-test"
	_ = a.DeleteBucket(ctx, bucket) // 清掉上次失败残留（不存在时报错可忽略）

	if err := a.CreateBucket(ctx, bucket, "us-east-1"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	buckets, err := a.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	found := false
	for _, b := range buckets {
		if b.Name == bucket {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListBuckets 未包含 %s: %+v", bucket, buckets)
	}

	// Put / Stat / Get：SigV4 签名必须被服务器接受。
	// 键名用 ASCII：S3Proxy 的文件系统后端按本机 locale 存取，中文键会被其后端拒绝（与本适配器无关）
	const key = "docs/wiring-check.txt"
	payload := "pixiu storage wiring payload"
	if err := a.PutObject(ctx, bucket, key, strings.NewReader(payload), int64(len(payload)), "text/plain; charset=utf-8"); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	stat, err := a.StatObject(ctx, bucket, key, "")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if stat.Size != int64(len(payload)) || !strings.HasPrefix(stat.ContentType, "text/plain") {
		t.Errorf("stat = %+v", stat)
	}
	reader, err := a.GetObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	body, err := io.ReadAll(reader.Body)
	reader.Body.Close()
	if err != nil || string(body) != payload {
		t.Fatalf("GetObject body = %q, err = %v", string(body), err)
	}

	// 预签名：URL 由服务器校验签名后返回对象
	presigned, err := a.PresignGetObject(ctx, bucket, key, 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}
	resp, err := http.Get(presigned)
	if err != nil {
		t.Fatalf("presigned GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned GET status = %d", resp.StatusCode)
	}
	signedBody, _ := io.ReadAll(resp.Body)
	if string(signedBody) != payload {
		t.Errorf("presigned body = %q", string(signedBody))
	}

	// 列举：流式（递归）与分页
	streamed := 0
	for item := range a.ListObjectsStream(ctx, bucket, ListOptions{Recursive: true}) {
		if item.Err != nil {
			t.Fatalf("ListObjectsStream: %v", item.Err)
		}
		streamed++
	}
	if streamed != 1 {
		t.Errorf("流式列举 = %d 条, want 1", streamed)
	}
	page, err := a.ListObjectsV2Page(ctx, bucket, "docs/", "", "/", 10)
	if err != nil {
		t.Fatalf("ListObjectsV2Page: %v", err)
	}
	if len(page.Contents) != 1 || page.Contents[0].Key != key {
		t.Errorf("分页列举 = %+v", page.Contents)
	}

	// 批量删除（真实服务器回执）
	refs := make(chan DeleteRef, 1)
	refs <- DeleteRef{Key: key}
	close(refs)
	failed := 0
	for item := range a.DeleteObjectsBatch(ctx, bucket, refs) {
		failed++
		t.Logf("delete error: key=%s err=%v", item.Key, item.Err)
	}
	if failed != 0 {
		t.Fatalf("DeleteObjectsBatch 出现 %d 条失败", failed)
	}
	if _, err := a.StatObject(ctx, bucket, key, ""); err == nil {
		t.Errorf("删除后 StatObject 应报错")
	}

	if err := a.DeleteBucket(ctx, bucket); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
}
