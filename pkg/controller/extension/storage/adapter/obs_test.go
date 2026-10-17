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

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

// newOBSTestAdapter 起一个假 OBS 端点：所有 bucket 子域请求经自定义 DialContext 拨到测试服务器，
// 覆盖「请求构造 + 响应/错误解析」的接线级行为（无需真实账号）。
func newOBSTestAdapter(t *testing.T, handler http.HandlerFunc) Adapter {
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
		Provider:        "obs",
		Endpoint:        "obs.cn-north-4.myhuaweicloud.com",
		Secure:          false,
		Region:          "cn-north-4",
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "SECRETEXAMPLE",
		Transport:       transport,
	})
	if err != nil {
		t.Fatalf("newOBS: %v", err)
	}
	return a
}

func TestOBSListBuckets(t *testing.T) {
	var gotHost string
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListAllMyBucketsResult>
			<Owner><ID>domain/1</ID></Owner>
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
	if gotHost == "" || strings.HasPrefix(gotHost, "demo-bucket.") {
		t.Errorf("服务级请求 Host = %q", gotHost)
	}
	if len(buckets) != 1 || buckets[0].Name != "demo-bucket" {
		t.Fatalf("buckets = %+v", buckets)
	}
	if buckets[0].CreationDate.IsZero() {
		t.Errorf("CreationDate 未解析: %+v", buckets[0])
	}
}

func TestOBSStatObject(t *testing.T) {
	var gotHost, gotPath string
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotPath = r.Host, r.URL.Path
		h := w.Header()
		h.Set("ETag", `"abc123"`)
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Length", "123")
		h.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		h.Set("x-obs-storage-class", "STANDARD")
		h.Set("x-obs-meta-foo", "bar")
		h.Set("x-obs-version-id", "v1")
		w.WriteHeader(http.StatusOK)
	})

	stat, err := a.StatObject(context.Background(), "demo-bucket", "docs/小结.txt", "")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if !strings.HasPrefix(gotHost, "demo-bucket.") {
		t.Errorf("bucket 级请求 Host = %q, want bucket 子域", gotHost)
	}
	if gotPath != "/docs/小结.txt" {
		t.Errorf("请求路径 = %q", gotPath)
	}
	if stat.ETag != "abc123" || stat.Size != 123 || stat.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("stat = %+v", stat)
	}
	if stat.StorageClass != "STANDARD" || stat.VersionID != "v1" {
		t.Errorf("storage class / version = %q / %q", stat.StorageClass, stat.VersionID)
	}
	// 用户元数据的键形态由 SDK 决定（去前缀），按值断言
	if len(stat.UserMetadata) != 1 || !containsValue(stat.UserMetadata, "bar") {
		t.Errorf("UserMetadata = %+v, want 单条 foo=bar", stat.UserMetadata)
	}
	if stat.Metadata.Get("Content-Encoding") == "" {
		// Content-Encoding 未设置时不应有值；这里验证头回填通道可用：Last-Modified 必须非空
		if stat.Metadata.Get("Last-Modified") == "" {
			t.Errorf("原始头回填缺失: %+v", stat.Metadata)
		}
	}
}

func containsValue(m map[string]string, want string) bool {
	for _, v := range m {
		if v == want {
			return true
		}
	}
	return false
}

func TestOBSGetBucketPolicyAbsent(t *testing.T) {
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
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

func TestOBSBucketPolicyRoundTrip(t *testing.T) {
	const doc = `{"Version":"2008-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::demo-bucket/*"]}]}`

	var putBody string
	var deleteCalled bool
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
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

func TestOBSDeleteObjectsBatch(t *testing.T) {
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.URL.RawQuery, "delete") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.RawQuery)
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

func TestOBSListObjectsPageMarkerPassthrough(t *testing.T) {
	var gotMarker string
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotMarker = r.URL.Query().Get("marker")
		w.Header().Set("Content-Type", "application/xml")
		// 平铺模式：服务端不返回 NextMarker
		_, _ = w.Write([]byte(`<ListBucketResult>
			<Name>demo-bucket</Name>
			<IsTruncated>true</IsTruncated>
			<Contents><Key>k1</Key><Size>1</Size><ETag>"e1"</ETag></Contents>
			<Contents><Key>k2</Key><Size>2</Size><ETag>"e2"</ETag></Contents>
		</ListBucketResult>`))
	})

	page, err := a.ListObjectsV2Page(context.Background(), "demo-bucket", "", "cursor-1", "", 2)
	if err != nil {
		t.Fatalf("ListObjectsV2Page: %v", err)
	}
	if gotMarker != "cursor-1" {
		t.Errorf("token 未以 marker 透传: marker=%q", gotMarker)
	}
	if len(page.Contents) != 2 || page.Contents[1].Key != "k2" || page.Contents[1].ETag != "e2" {
		t.Errorf("contents = %+v", page.Contents)
	}
	if !page.IsTruncated || page.NextToken != "k2" {
		t.Errorf("truncated=%v next=%q, want true/'k2'（平铺模式用最后 key 续读）", page.IsTruncated, page.NextToken)
	}
}

func TestOBSListVersionsMergedOrder(t *testing.T) {
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListVersionsResult>
			<Name>demo-bucket</Name>
			<IsTruncated>false</IsTruncated>
			<Version>
				<Key>a.txt</Key><VersionId>v2</VersionId><IsLatest>true</IsLatest>
				<LastModified>2024-01-03T00:00:00.000Z</LastModified>
				<ETag>"e2"</ETag><Size>2</Size><StorageClass>STANDARD</StorageClass>
			</Version>
			<Version>
				<Key>a.txt</Key><VersionId>v1</VersionId><IsLatest>false</IsLatest>
				<LastModified>2024-01-01T00:00:00.000Z</LastModified>
				<ETag>"e1"</ETag><Size>1</Size>
			</Version>
			<DeleteMarker>
				<Key>a.txt</Key><VersionId>dm1</VersionId><IsLatest>false</IsLatest>
				<LastModified>2024-01-02T00:00:00.000Z</LastModified>
			</DeleteMarker>
			<Version>
				<Key>b.txt</Key><VersionId>v9</VersionId><IsLatest>true</IsLatest>
				<LastModified>2024-01-04T00:00:00.000Z</LastModified>
				<ETag>"e9"</ETag><Size>9</Size>
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
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	// 合并口径：key 升序；同 key 内修改时间倒序（删除标记按时间参与交错）
	wantOrder := []string{"a.txt/v2", "a.txt/dm1", "a.txt/v1", "b.txt/v9"}
	for i, want := range wantOrder {
		got := items[i].Key + "/" + items[i].VersionID
		if got != want {
			t.Errorf("order[%d] = %s, want %s", i, got, want)
		}
	}
	if !items[1].IsDeleteMarker || items[1].Size != 0 {
		t.Errorf("delete marker 映射错误: %+v", items[1])
	}
	if items[0].ETag != "e2" {
		t.Errorf("ETag 未去引号: %q", items[0].ETag)
	}
}

func TestOBSPresignGetObject(t *testing.T) {
	a := newOBSTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Presign 不应发起请求")
	})
	u, err := a.PresignGetObject(context.Background(), "demo-bucket", "docs/小结.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}
	if !strings.Contains(u, "demo-bucket.") || !strings.Contains(u, "obs.cn-north-4") {
		t.Errorf("presigned url = %s", u)
	}
}

func TestOBSAbsentConfigClassification(t *testing.T) {
	// 404 + NoSuchBucketPolicy → 未配置
	notFound := wrapOBSError(&obs.ObsError{BaseModel: obs.BaseModel{StatusCode: 404}, Code: "NoSuchBucketPolicy", Status: "404 Not Found"})
	if !absentCodes(notFound, "NoSuchBucketPolicy") {
		t.Errorf("404 NoSuchBucketPolicy 应判定为未配置")
	}
	// 404 + NoSuchBucket → 真实错误
	noSuchBucket := wrapOBSError(&obs.ObsError{BaseModel: obs.BaseModel{StatusCode: 404}, Code: "NoSuchBucket", Status: "404 Not Found"})
	if absentCodes(noSuchBucket) {
		t.Errorf("NoSuchBucket 是真实错误，不得判定为未配置")
	}
	// Status 文本兜底解析（StatusCode 缺失场景）
	statusOnly := wrapOBSError(&obs.ObsError{Code: "NoSuchCORSConfiguration", Status: "404 Not Found"})
	if !absentCodes(statusOnly, "NoSuchCORSConfiguration") {
		t.Errorf("Status 文本 404 应可兜底判定为未配置")
	}
}
