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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newOSSTestAdapter 起一个假 OSS 端点：所有 bucket 子域请求经自定义 DialContext 拨到测试服务器，
// 覆盖「请求构造 + 响应/错误解析」的接线级行为（无需真实账号）。
func newOSSTestAdapter(t *testing.T, handler http.HandlerFunc) Adapter {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse httptest url: %v", err)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", u.Host)
		},
	}
	a, err := New(Config{
		Provider:        "oss",
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		Secure:          false,
		Region:          "cn-hangzhou",
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "SECRETEXAMPLE",
		Transport:       transport,
	})
	if err != nil {
		t.Fatalf("newOSS: %v", err)
	}
	return a
}

func TestOSSListBuckets(t *testing.T) {
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListAllMyBucketsResult>
			<Owner><ID>1250000000</ID></Owner>
			<Buckets><Bucket>
				<Name>demo-bucket</Name>
				<Location>oss-cn-hangzhou</Location>
				<CreationDate>2024-01-02T03:04:05.000Z</CreationDate>
			</Bucket></Buckets>
		</ListAllMyBucketsResult>`))
	})

	buckets, err := a.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Name != "demo-bucket" {
		t.Fatalf("buckets = %+v", buckets)
	}
	if buckets[0].CreationDate.IsZero() {
		t.Errorf("CreationDate 未解析: %+v", buckets[0])
	}
}

func TestOSSStatObject(t *testing.T) {
	var gotPath string
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		h := w.Header()
		h.Set("ETag", `"abc123"`)
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Length", "123")
		h.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		h.Set("x-oss-storage-class", "IA")
		h.Set("x-oss-meta-foo", "bar")
		w.WriteHeader(http.StatusOK)
	})

	stat, err := a.StatObject(context.Background(), "demo-bucket", "docs/小结.txt", "")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if gotPath != "/docs/小结.txt" {
		t.Errorf("请求路径 = %q", gotPath)
	}
	if stat.ETag != "abc123" || stat.Size != 123 || stat.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("stat = %+v", stat)
	}
	if stat.StorageClass != "IA" {
		t.Errorf("StorageClass = %q", stat.StorageClass)
	}
	if stat.UserMetadata["foo"] != "bar" {
		t.Errorf("UserMetadata = %+v, want foo=bar", stat.UserMetadata)
	}
	if stat.Metadata.Get("Last-Modified") == "" {
		t.Errorf("原始响应头缺失: %+v", stat.Metadata)
	}
}

func TestOSSGetBucketPolicyAbsent(t *testing.T) {
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["policy"]; !ok {
			t.Errorf("missing ?policy subresource: %s", r.URL.RawQuery)
		}
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

func TestOSSBucketPolicyRoundTrip(t *testing.T) {
	const doc = `{"Version":"1","Statement":[{"Effect":"Allow","Principal":"*","Action":["oss:GetObject"],"Resource":["acs:oss:*:*:demo-bucket/*"]}]}`

	var putBody string
	var deleteCalled bool
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(doc))
		case http.MethodPut:
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			putBody = string(buf)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Get：raw 透传，一字不差
	policy, err := a.GetBucketPolicy(context.Background(), "demo-bucket")
	if err != nil {
		t.Fatalf("GetBucketPolicy: %v", err)
	}
	if policy != doc {
		t.Errorf("policy 原文未透传: %s", policy)
	}

	// Set：原文提交
	if err := a.SetBucketPolicy(context.Background(), "demo-bucket", doc); err != nil {
		t.Fatalf("SetBucketPolicy: %v", err)
	}
	if putBody != doc {
		t.Errorf("PUT body = %s", putBody)
	}

	// 空串 = 删除策略
	if err := a.SetBucketPolicy(context.Background(), "demo-bucket", ""); err != nil {
		t.Fatalf("SetBucketPolicy(空): %v", err)
	}
	if !deleteCalled {
		t.Errorf("空策略应触发 DELETE ?policy")
	}
}

func TestOSSDeleteObjectsBatch(t *testing.T) {
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["delete"]; !ok {
			t.Errorf("missing ?delete subresource")
		}
		if !strings.Contains(r.URL.RawQuery, "delete") {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		// Verbose 模式：Deleted 只覆盖成功项（b.txt 视为失败）
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<DeleteResult><Deleted><Key>a.txt</Key></Deleted></DeleteResult>`))
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
	if ErrorCode(failures[0].Err) != "DeleteObjectFailed" {
		t.Errorf("failure code = %q", ErrorCode(failures[0].Err))
	}
}

func TestOSSListObjectsV2Page(t *testing.T) {
	var gotToken string
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("continuation-token")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListBucketResult>
			<Name>demo-bucket</Name>
			<IsTruncated>true</IsTruncated>
			<NextContinuationToken>cursor-2</NextContinuationToken>
			<Contents><Key>k1</Key><Size>1</Size><ETag>"e1"</ETag></Contents>
			<CommonPrefixes><Prefix>dir/</Prefix></CommonPrefixes>
		</ListBucketResult>`))
	})

	page, err := a.ListObjectsV2Page(context.Background(), "demo-bucket", "", "cursor-1", "/", 10)
	if err != nil {
		t.Fatalf("ListObjectsV2Page: %v", err)
	}
	if gotToken != "cursor-1" {
		t.Errorf("continuationToken 未透传: %q", gotToken)
	}
	if len(page.Contents) != 1 || page.Contents[0].Key != "k1" || page.Contents[0].ETag != "e1" {
		t.Errorf("contents = %+v", page.Contents)
	}
	if len(page.CommonPrefixes) != 1 || page.CommonPrefixes[0] != "dir/" {
		t.Errorf("commonPrefixes = %+v", page.CommonPrefixes)
	}
	if !page.IsTruncated || page.NextToken != "cursor-2" {
		t.Errorf("truncated=%v next=%q", page.IsTruncated, page.NextToken)
	}
}

func TestOSSListVersionsStreamMixedOrder(t *testing.T) {
	var gotIsMixQuery string
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		// IsMix 是纯客户端开关：线上请求只应带 versions 子资源
		gotIsMixQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListVersionsResult>
			<Name>demo-bucket</Name>
			<IsTruncated>false</IsTruncated>
			<Version>
				<Key>a.txt</Key><VersionId>v2</VersionId><IsLatest>true</IsLatest>
				<LastModified>2024-01-03T00:00:00.000Z</LastModified>
				<ETag>"e2"</ETag><Type>Normal</Type><Size>2</Size><StorageClass>Standard</StorageClass>
			</Version>
			<DeleteMarker>
				<Key>a.txt</Key><VersionId>dm1</VersionId><IsLatest>false</IsLatest>
				<LastModified>2024-01-02T00:00:00.000Z</LastModified>
			</DeleteMarker>
			<Version>
				<Key>a.txt</Key><VersionId>v1</VersionId><IsLatest>false</IsLatest>
				<LastModified>2024-01-01T00:00:00.000Z</LastModified>
				<ETag>"e1"</ETag><Type>Normal</Type><Size>1</Size>
			</Version>
		</ListVersionsResult>`))
	})

	var items []ObjectInfo
	for item := range a.ListVersionsStream(context.Background(), "demo-bucket", ListOptions{Recursive: true}) {
		if item.Err != nil {
			t.Fatalf("ListVersionsStream: %v", item.Err)
		}
		items = append(items, item.ObjectInfo)
	}
	if !strings.Contains(gotIsMixQuery, "versions") {
		t.Errorf("versions 子资源缺失: %q", gotIsMixQuery)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	// 文档序必须保真（版本/删除标记交错）
	wantIDs := []string{"v2", "dm1", "v1"}
	for i, want := range wantIDs {
		if items[i].VersionID != want {
			t.Errorf("order[%d] = %s, want %s", i, items[i].VersionID, want)
		}
	}
	if items[1].IsDeleteMarker != true || items[0].IsDeleteMarker != false || items[2].IsDeleteMarker != false {
		t.Errorf("delete marker 标记错误: %+v", items)
	}
	if items[0].ETag != "e2" || items[0].Size != 2 {
		t.Errorf("版本条目字段缺失: %+v", items[0])
	}
}

func TestOSSPresignGetObject(t *testing.T) {
	a := newOSSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Presign 不应发起请求")
	})
	u, err := a.PresignGetObject(context.Background(), "demo-bucket", "docs/小结.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}
	if !strings.Contains(u, "demo-bucket.") || !strings.Contains(strings.ToLower(u), "signature") {
		t.Errorf("presigned url = %s", u)
	}
}

func TestOSSNormalizeEndpoint(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// S3 兼容端点重写为原生服务级端点
		{"s3.oss-cn-hangzhou.aliyuncs.com", "oss-cn-hangzhou.aliyuncs.com"},
		{"S3.OSS-CN-BEIJING.ALIYUNCS.COM", "oss-cn-beijing.aliyuncs.com"},
		// 原生端点与自定义端点原样保留
		{"oss-cn-hangzhou.aliyuncs.com", "oss-cn-hangzhou.aliyuncs.com"},
		{"oss-cn-hangzhou-internal.aliyuncs.com", "oss-cn-hangzhou-internal.aliyuncs.com"},
		{"custom.example.com", "custom.example.com"},
	}
	for _, c := range cases {
		if got := ossNormalizeEndpoint(c.in); got != c.want {
			t.Errorf("ossNormalizeEndpoint(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestOSSNilTransportDoesNotPanic 回归守护（F1）：cfg.Transport 为 nil 时不得把 typed-nil 注入
// HttpClient（生产路径 clientFor 从不传 Transport，历史缺陷会在首个请求期 panic）。
// 断言真实走「空 Transport」分支：由 SDK 默认传输层发起请求，对不可达端点返回普通 error 而非 panic。
func TestOSSNilTransportDoesNotPanic(t *testing.T) {
	a, err := New(Config{
		Provider:        "oss",
		Endpoint:        "127.0.0.1:9", // discard 端口：连接必被拒，秒级失败
		Secure:          false,
		Region:          "cn-hangzhou",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		// Transport 刻意不注入：覆盖生产路径的空 Transport 分支
	})
	if err != nil {
		t.Fatalf("New(oss, 空 Transport): %v", err)
	}
	if _, err := a.ListBuckets(context.Background()); err == nil {
		t.Fatalf("不可达端点应返回 error（且不得 panic）")
	}
}
