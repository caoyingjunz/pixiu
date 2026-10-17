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
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

func TestToObjectInfoMapping(t *testing.T) {
	now := time.Now()
	src := minio.ObjectInfo{
		Key:            "a/b.txt",
		Size:           42,
		LastModified:   now,
		ETag:           "e1",
		VersionID:      "v1",
		IsLatest:       true,
		IsDeleteMarker: true,
		StorageClass:   "STANDARD",
	}
	got := toObjectInfo(src)
	want := ObjectInfo{
		Key:            "a/b.txt",
		Size:           42,
		LastModified:   now,
		ETag:           "e1",
		VersionID:      "v1",
		IsLatest:       true,
		IsDeleteMarker: true,
		StorageClass:   "STANDARD",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("toObjectInfo() = %+v, want %+v", got, want)
	}
}

// ObjectStat 的头字段保持 http.Header 直通：控制层按头部名取值（Content-Encoding 等）。
func TestToObjectStatHeaderMapping(t *testing.T) {
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	h := http.Header{}
	h.Set("Content-Encoding", "gzip")
	h.Set("Cache-Control", "max-age=60")
	src := minio.ObjectInfo{
		Key:          "a/b.txt",
		VersionID:    "v9",
		Size:         10,
		ETag:         "e1",
		ContentType:  "text/plain",
		LastModified: now,
		Expires:      expires,
		StorageClass: "STANDARD",
		Metadata:     h,
		UserMetadata: minio.StringMap{"X-Amz-Storage-Class": "STANDARD", "K": "V"},
	}
	got := toObjectStat(src)
	if got.Key != "a/b.txt" || got.VersionID != "v9" || got.Size != 10 || got.ETag != "e1" {
		t.Errorf("基础字段映射不符: %+v", got)
	}
	if got.ContentType != "text/plain" || !got.LastModified.Equal(now) || !got.Expires.Equal(expires) {
		t.Errorf("内容类型/时间字段映射不符: %+v", got)
	}
	if got.Metadata.Get("Content-Encoding") != "gzip" {
		t.Errorf("Metadata[Content-Encoding] = %q, want gzip", got.Metadata.Get("Content-Encoding"))
	}
	if got.Metadata.Get("Cache-Control") != "max-age=60" {
		t.Errorf("Metadata[Cache-Control] = %q, want max-age=60", got.Metadata.Get("Cache-Control"))
	}
	if !reflect.DeepEqual(map[string]string(got.UserMetadata), map[string]string{"X-Amz-Storage-Class": "STANDARD", "K": "V"}) {
		t.Errorf("UserMetadata 映射不符: %+v", got.UserMetadata)
	}
}

func TestToObjectPageMapping(t *testing.T) {
	page := toObjectPage(
		[]minio.ObjectInfo{{Key: "a.txt", Size: 1}, {Key: "b.txt", Size: 2}},
		[]minio.CommonPrefix{{Prefix: "dir/"}},
		true,
		"tok",
	)
	if len(page.Contents) != 2 || page.Contents[0].Key != "a.txt" || page.Contents[1].Key != "b.txt" {
		t.Errorf("Contents 映射不符: %+v", page.Contents)
	}
	if !reflect.DeepEqual(page.CommonPrefixes, []string{"dir/"}) {
		t.Errorf("CommonPrefixes 映射不符: %+v", page.CommonPrefixes)
	}
	if !page.IsTruncated || page.NextToken != "tok" {
		t.Errorf("截断/续页字段映射不符: %+v", page)
	}
}

func TestLifecycleRuleRoundTrip(t *testing.T) {
	in := []LifecycleRule{
		{ID: "r1", Status: "Enabled", Prefix: "logs/", ExpirationDays: 30, AbortMultipartDays: 7},
		{ID: "r2", Status: "Disabled", Prefix: "", ExpirationDays: 0, AbortMultipartDays: 14},
	}
	out := fromLifecycleConfig(toLifecycleConfig(in))
	if !reflect.DeepEqual(out, in) {
		t.Errorf("lifecycle 往返丢失字段: got %+v, want %+v", out, in)
	}
}

func TestCorsRuleRoundTrip(t *testing.T) {
	in := []CORSRule{
		{
			ID:             "c1",
			AllowedOrigins: []string{"https://a.example.com"},
			AllowedMethods: []string{"GET", "PUT"},
			AllowedHeaders: []string{"*"},
			ExposeHeaders:  []string{"ETag"},
			MaxAgeSeconds:  3000,
		},
	}
	out := fromCorsConfig(toCorsConfig(in))
	if !reflect.DeepEqual(out, in) {
		t.Errorf("cors 往返丢失字段: got %+v, want %+v", out, in)
	}
}

func TestPartsPageMapping(t *testing.T) {
	now := time.Now()
	result := minio.ListObjectPartsResult{
		IsTruncated:          true,
		NextPartNumberMarker: 5,
		ObjectParts: []minio.ObjectPart{
			{PartNumber: 1, ETag: "e1", Size: 100, LastModified: now},
		},
	}
	page := toPartsPage(result)
	if !page.IsTruncated || page.NextPartNumberMarker != 5 {
		t.Errorf("分页字段映射不符: %+v", page)
	}
	if len(page.Parts) != 1 {
		t.Fatalf("Parts 数量 = %d, want 1", len(page.Parts))
	}
	want := PartInfo{PartNumber: 1, ETag: "e1", Size: 100, LastModified: now}
	if !reflect.DeepEqual(page.Parts[0], want) {
		t.Errorf("PartInfo 映射不符: got %+v, want %+v", page.Parts[0], want)
	}
}

// 碎片列表的 Size 现状恒为 0（SDK 语义）——映射必须原样保留，不得「顺手修正」。
func TestUploadsPageMappingKeepsSizeAsIs(t *testing.T) {
	now := time.Now()
	result := minio.ListMultipartUploadsResult{
		IsTruncated:        true,
		NextKeyMarker:      "k2",
		NextUploadIDMarker: "u2",
		Uploads: []minio.ObjectMultipartInfo{
			{Key: "a.txt", UploadID: "u1", Initiated: now, Size: 0},
			{Key: "b.txt", UploadID: "u3", Initiated: now, Size: 7},
		},
	}
	page := toUploadsPage(result)
	if !page.IsTruncated || page.NextKeyMarker != "k2" || page.NextUploadIDMarker != "u2" {
		t.Errorf("分页字段映射不符: %+v", page)
	}
	if len(page.Uploads) != 2 {
		t.Fatalf("Uploads 数量 = %d, want 2", len(page.Uploads))
	}
	if page.Uploads[0].Size != 0 || page.Uploads[1].Size != 7 {
		t.Errorf("Size 未被原样保留: %+v", page.Uploads)
	}
	if page.Uploads[0].Key != "a.txt" || page.Uploads[0].UploadID != "u1" || !page.Uploads[0].Initiated.Equal(now) {
		t.Errorf("MultipartUploadInfo 映射不符: %+v", page.Uploads[0])
	}
}
