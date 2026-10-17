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
	"errors"
	"hash/crc64"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// newCosTestAdapter 起一个假 COS 端点：所有 bucket 子域请求经自定义 DialContext 拨到测试服务器，
// 覆盖「请求构造 + 响应/错误解析」的接线级行为（无需真实账号）。
func newCosTestAdapter(t *testing.T, handler http.HandlerFunc) Adapter {
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
		Provider:         "cos",
		Endpoint:         "cos.ap-guangzhou.myqcloud.com",
		Secure:           false,
		Region:           "ap-guangzhou",
		AccessKeyID:      "AKIDEXAMPLE",
		SecretAccessKey:  "SECRETEXAMPLE",
		SignatureVersion: "", // 原生适配器忽略签名版本
		Transport:        transport,
	})
	if err != nil {
		t.Fatalf("newCos: %v", err)
	}
	return a
}

func TestCosListBuckets(t *testing.T) {
	var gotHost string
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<ListAllMyBucketsResult>
			<Owner><ID>qcs::cam::uin/1250000000:uin/1250000000</ID></Owner>
			<Buckets><Bucket>
				<Name>demo-1250000000</Name>
				<Location>ap-guangzhou</Location>
				<CreationDate>2024-01-02T03:04:05Z</CreationDate>
			</Bucket></Buckets>
		</ListAllMyBucketsResult>`))
	})

	buckets, err := a.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if gotHost != "cos.ap-guangzhou.myqcloud.com" {
		t.Errorf("服务级请求 Host = %q, want 服务端点", gotHost)
	}
	if len(buckets) != 1 || buckets[0].Name != "demo-1250000000" {
		t.Fatalf("buckets = %+v", buckets)
	}
	want := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if !buckets[0].CreationDate.Equal(want) {
		t.Errorf("CreationDate = %v, want %v", buckets[0].CreationDate, want)
	}
}

func TestCosStatObject(t *testing.T) {
	var gotHost, gotPath string
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotPath = r.Host, r.URL.Path
		h := w.Header()
		h.Set("ETag", `"abc123"`)
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Length", "123")
		h.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		h.Set("X-Cos-Storage-Class", "STANDARD_IA")
		h.Set("X-Cos-Version-Id", "v1")
		h.Set("X-Cos-Meta-Foo", "bar")
		w.WriteHeader(http.StatusOK)
	})

	stat, err := a.StatObject(context.Background(), "demo-1250000000", "docs/小结.txt", "")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if !strings.HasPrefix(gotHost, "demo-1250000000.cos.ap-guangzhou.myqcloud.com") {
		t.Errorf("bucket 级请求 Host = %q, want bucket 子域", gotHost)
	}
	if gotPath != "/docs/小结.txt" {
		t.Errorf("请求路径 = %q", gotPath)
	}
	if stat.ETag != "abc123" || stat.Size != 123 || stat.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("stat = %+v", stat)
	}
	if stat.StorageClass != "STANDARD_IA" || stat.VersionID != "v1" {
		t.Errorf("storage class / version = %q / %q", stat.StorageClass, stat.VersionID)
	}
	if stat.UserMetadata["Foo"] != "bar" {
		t.Errorf("UserMetadata = %+v, want Foo=bar", stat.UserMetadata)
	}
}

func TestCosGetBucketPolicyAbsent(t *testing.T) {
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["policy"]; !ok {
			t.Errorf("missing ?policy subresource: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<Error><Code>NoSuchBucketPolicy</Code><Message>no policy</Message></Error>`))
	})

	policy, err := a.GetBucketPolicy(context.Background(), "demo-1250000000")
	if err != nil {
		t.Fatalf("无策略应归一化为空值，got err: %v", err)
	}
	if policy != "" {
		t.Errorf("policy = %q, want empty", policy)
	}
}

func TestCosBucketPolicyRoundTrip(t *testing.T) {
	const doc = `{"version":"2.0","Statement":[{"Principal":{"qcs":["qcs::cam::anyone:anyone"]},"Effect":"allow","Action":["name/cos:GetObject"],"Resource":["qcs::cos:ap-guangzhou:uid/1250000000:demo-1250000000/*"]}]}`

	var putBody string
	var deleteCalled bool
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
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

	// Get：结构化解析后重新序列化，文档关键字段必须保真
	policy, err := a.GetBucketPolicy(context.Background(), "demo-1250000000")
	if err != nil {
		t.Fatalf("GetBucketPolicy: %v", err)
	}
	if !strings.Contains(policy, "name/cos:GetObject") || !strings.Contains(policy, "qcs::cam::anyone:anyone") {
		t.Errorf("policy 回读丢失关键字段: %s", policy)
	}

	// Set：原文进入结构化模型后提交
	if err := a.SetBucketPolicy(context.Background(), "demo-1250000000", doc); err != nil {
		t.Fatalf("SetBucketPolicy: %v", err)
	}
	if !strings.Contains(putBody, "name/cos:GetObject") {
		t.Errorf("PUT body 丢失 action: %s", putBody)
	}

	// 空串 = 删除策略
	if err := a.SetBucketPolicy(context.Background(), "demo-1250000000", ""); err != nil {
		t.Fatalf("SetBucketPolicy(空): %v", err)
	}
	if !deleteCalled {
		t.Errorf("空策略应触发 DELETE ?policy")
	}
}

func TestCosDeleteObjectsBatch(t *testing.T) {
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["delete"]; !ok {
			t.Errorf("missing ?delete subresource")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<DeleteResult>
			<Error><Key>b.txt</Key><Code>AccessDenied</Code><Message>denied</Message></Error>
		</DeleteResult>`))
	})

	refs := make(chan DeleteRef, 2)
	refs <- DeleteRef{Key: "a.txt"}
	refs <- DeleteRef{Key: "b.txt"}
	close(refs)

	var failures []DeleteError
	for item := range a.DeleteObjectsBatch(context.Background(), "demo-1250000000", refs) {
		failures = append(failures, item)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %+v, want 1 entry", failures)
	}
	if failures[0].Key != "b.txt" || ErrorCode(failures[0].Err) != "AccessDenied" {
		t.Errorf("failure = %+v (code=%q)", failures[0], ErrorCode(failures[0].Err))
	}
}

func TestCosUploadPartETag(t *testing.T) {
	const payload = "payload"
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("partNumber") != "2" || r.URL.Query().Get("uploadId") != "up-1" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("ETag", `"p2etag"`)
		// SDK 对上传做 CRC64 校验：回包必须携带与载荷一致的 crc64（真实 COS 亦然）
		crc := crc64.Checksum([]byte(payload), crc64.MakeTable(crc64.ECMA))
		w.Header().Set("x-cos-hash-crc64ecma", strconv.FormatUint(crc, 10))
		w.WriteHeader(http.StatusOK)
	})

	part, err := a.UploadPart(context.Background(), "demo-1250000000", "big.bin", "up-1", 2, strings.NewReader("payload"), 7)
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	if part.PartNumber != 2 || part.ETag != "p2etag" || part.Size != 7 {
		t.Errorf("part = %+v", part)
	}
}

func TestCosListObjectsPageMarkerPassthrough(t *testing.T) {
	var gotMarker string
	a := newCosTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotMarker = r.URL.Query().Get("marker")
		w.Header().Set("Content-Type", "application/xml")
		// 平铺模式：服务端不返回 NextMarker
		_, _ = w.Write([]byte(`<ListBucketResult>
			<Name>demo-1250000000</Name>
			<IsTruncated>true</IsTruncated>
			<Contents><Key>k1</Key><Size>1</Size></Contents>
			<Contents><Key>k2</Key><Size>2</Size></Contents>
		</ListBucketResult>`))
	})

	page, err := a.ListObjectsV2Page(context.Background(), "demo-1250000000", "", "cursor-1", "", 2)
	if err != nil {
		t.Fatalf("ListObjectsV2Page: %v", err)
	}
	if gotMarker != "cursor-1" {
		t.Errorf("token 未以 marker 透传: marker=%q", gotMarker)
	}
	if len(page.Contents) != 2 || page.Contents[1].Key != "k2" || page.Contents[1].Size != 2 {
		t.Errorf("contents = %+v", page.Contents)
	}
	if !page.IsTruncated || page.NextToken != "k2" {
		t.Errorf("truncated=%v nextToken=%q, want true/'k2'（平铺模式用最后 key 续读）", page.IsTruncated, page.NextToken)
	}
}

func TestCosAbsentConfigClassification(t *testing.T) {
	resp := func(status int) *http.Response { return &http.Response{StatusCode: status} }

	notFoundPolicy := wrapCosError(&cos.ErrorResponse{Response: resp(404), Code: "NoSuchBucketPolicy"})
	if !absentCodes(notFoundPolicy, "NoSuchBucketPolicy") {
		t.Errorf("404 NoSuchBucketPolicy 应判定为未配置")
	}
	noSuchBucket := wrapCosError(&cos.ErrorResponse{Response: resp(404), Code: "NoSuchBucket"})
	if absentCodes(noSuchBucket) {
		t.Errorf("NoSuchBucket 是真实错误，不得判定为未配置")
	}
	denied := wrapCosError(&cos.ErrorResponse{Response: resp(403), Code: "AccessDenied"})
	if absentCodes(denied) {
		t.Errorf("403 不得判定为未配置")
	}
	if ErrorCode(denied) != "AccessDenied" || !errors.Is(denied, denied) {
		t.Errorf("ErrorCode = %q", ErrorCode(denied))
	}
}

func TestMergeCosVersionsOrdering(t *testing.T) {
	versions := []cos.ListVersionsResultVersion{
		{Key: "a.txt", VersionId: "v2", IsLatest: true, LastModified: "2024-01-03T00:00:00Z", Size: 2, ETag: `"e2"`},
		{Key: "a.txt", VersionId: "v1", LastModified: "2024-01-01T00:00:00Z", Size: 1},
		{Key: "b.txt", VersionId: "v1", IsLatest: true, LastModified: "2024-01-02T00:00:00Z", Size: 3},
	}
	deleteMarkers := []cos.ListVersionsResultDeleteMarker{
		{Key: "a.txt", VersionId: "dm1", IsLatest: false, LastModified: "2024-01-02T00:00:00Z"},
	}

	merged := mergeCosVersions(versions, deleteMarkers)
	if len(merged) != 4 {
		t.Fatalf("merged = %d entries", len(merged))
	}
	// key 升序；同 key 内按修改时间倒序（删除标记按时间参与交错）
	wantOrder := []string{"a.txt/v2", "a.txt/dm1", "a.txt/v1", "b.txt/v1"}
	for i, w := range wantOrder {
		got := merged[i].Key + "/" + merged[i].VersionID
		if got != w {
			t.Errorf("order[%d] = %s, want %s", i, got, w)
		}
	}
	if !merged[1].IsDeleteMarker || merged[1].Size != 0 {
		t.Errorf("delete marker 映射错误: %+v", merged[1])
	}
	if merged[0].ETag != "e2" {
		t.Errorf("ETag 未去引号: %q", merged[0].ETag)
	}
}

func TestCosUserMetadataAndETagHelpers(t *testing.T) {
	h := http.Header{}
	h.Set("X-Cos-Meta-Trace-Id", "t1")
	h.Set("Content-Type", "text/plain")
	meta := cosUserMetadata(h)
	if len(meta) != 1 || meta["Trace-Id"] != "t1" {
		t.Errorf("cosUserMetadata = %+v", meta)
	}
	if trimCosETag(` "abc" `) != "abc" {
		t.Errorf("trimCosETag: %q", trimCosETag(` "abc" `))
	}
	if !parseCosTime("2024-01-02T03:04:05Z").Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("parseCosTime RFC3339 解析失败")
	}
	if !parseCosTime("not-a-time").IsZero() {
		t.Errorf("非法时间应返回零值")
	}
}
