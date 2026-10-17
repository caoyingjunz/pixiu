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

// Package adapter 定义对象存储的数据面适配层：接口（36 方法 / 7 组）、DTO 与工厂。
//
// 设计约束：
//   - 本包不反向依赖 pkg/controller/extension/storage（保持无环）：控制层解析并校验配置，
//     适配器只消费；寻址方式与签名版本的语义推导留在控制层的 provider 层，不在本包重复实现。
//   - 接口按数据面原语划分；聚合逻辑（分页三态、目录展开、zip 打包、重命名等）留在控制层，
//     与协议无关的产品规则不进入适配器。
//   - 错误统一包装为 *StorageError；配置类「未设置」态（404 且非 NoSuchBucket，或历史已知
//     空态码）由适配器归一化为空值返回，不抛错。
package adapter

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// Config 是构造适配器所需的运行时配置（由控制层解析校验后传入）。
// 寻址方式与签名版本两个 S3 系参数由控制层按 provider 语义计算，这里原样消费。
type Config struct {
	Provider         string                 // 厂商标识，P1 起用于分派；generic 为兜底实现
	Endpoint         string                 // 端点 host[:port]（控制层已完成 scheme/host 校验）
	Secure           bool                   // 端点使用 https 时为 true
	Region           string                 // 区域（已完成控制层兜底）
	AccessKeyID      string                 // AK
	SecretAccessKey  string                 // SK
	SessionToken     string                 // 临时令牌；S3 V2 签名下被忽略（与改造前一致）
	SignatureVersion string                 // "v2"/"s3v2" 走 V2 签名，其余走 V4
	BucketLookup     minio.BucketLookupType // 寻址方式（由控制层 provider.BucketLookup 计算）
	Transport        *http.Transport        // 自定义传输层（探测场景注入 SSRF 防护）；nil 用 SDK 默认
}

// BucketInfo 桶摘要。
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// ObjectInfo 对象/版本摘要（流式列举与分页列举共用）。
type ObjectInfo struct {
	Key            string
	Size           int64
	LastModified   time.Time
	ETag           string
	VersionID      string
	IsLatest       bool
	IsDeleteMarker bool
	StorageClass   string
}

// ObjectEntry 流式列举的元素；Err 非空表示该元素是失败项（SDK 通道语义）。
type ObjectEntry struct {
	ObjectInfo
	Err error
}

// ObjectStream 是流式列举的只读通道：消费到 channel 关闭为止。
type ObjectStream <-chan ObjectEntry

// ObjectPage 分页列举结果。
type ObjectPage struct {
	Contents       []ObjectInfo
	CommonPrefixes []string // delimiter 分层模式下的目录前缀
	IsTruncated    bool
	NextToken      string // V2 为 NextContinuationToken；V1(marker) 为 NextMarker
}

// ListOptions 流式列举参数。
type ListOptions struct {
	Prefix    string
	Recursive bool
	MaxKeys   int
}

// ObjectStat 对象元数据（StatObject 与 GetObjectACL 共用）。
// Metadata 保持 http.Header 形态：控制层直接按头部字段取值（Content-Encoding 等），语义与 SDK 一致。
type ObjectStat struct {
	Key          string
	VersionID    string
	Size         int64
	ETag         string
	ContentType  string
	LastModified time.Time
	Expires      time.Time
	StorageClass string
	// CannedACL 是 GetObjectACL 返回的权限归类（private/public-read/public-read-write）。
	// 部分厂商只能从授权列表近似推导，可能为空。
	CannedACL    string
	Metadata     http.Header
	UserMetadata map[string]string
}

// ObjectReader 对象读取句柄：Body 惰性拉取（首次 Read 才发起 GET，SDK 语义）。
// 只承载数据流本身：大小与内容类型属元数据，由 StatObject 提供（保持与改造前一致的请求序列）。
type ObjectReader struct {
	Body io.ReadCloser
}

// CopySrc 复制源。
type CopySrc struct {
	Bucket    string
	Object    string
	VersionID string
}

// CopyDest 复制目标；ReplaceMetadata 为 true 时以提供的元数据整体覆盖目标元数据（SDK 语义）。
// 系统元数据（存储类型 / SSE 算法 / KMS 主密钥）使用显式字段承载；UserMetadata 只放用户自定义元数据。
type CopyDest struct {
	Bucket             string
	Object             string
	ReplaceMetadata    bool
	ContentType        string
	ContentEncoding    string
	ContentDisposition string
	ContentLanguage    string
	CacheControl       string
	Expires            time.Time
	StorageClass       string
	SSEAlgorithm       string
	SSEKMSKeyID        string
	UserMetadata       map[string]string
}

// DeleteRef 批量删除项（可带版本号）。
type DeleteRef struct {
	Key       string
	VersionID string
}

// DeleteError 批量删除的失败明细；成功的对象不产生条目（SDK 语义）。
type DeleteError struct {
	Key string
	Err error
}

// DeleteErrorStream 是批量删除失败明细的只读通道：消费到 channel 关闭为止。
type DeleteErrorStream <-chan DeleteError

// PartInfo 已上传分片。
type PartInfo struct {
	PartNumber   int
	ETag         string
	Size         int64
	LastModified time.Time
}

// PartsPage 分片列举页。
type PartsPage struct {
	Parts                []PartInfo
	IsTruncated          bool
	NextPartNumberMarker int
}

// CompletePart 合并分片时的提交项。
type CompletePart struct {
	PartNumber int
	ETag       string
}

// CompletedObject 合并结果；Size 保持 SDK 返回值（既有语义下可能为 0，勿在此层「修正」）。
type CompletedObject struct {
	Key  string
	ETag string
	Size int64
}

// MultipartUploadInfo 未完成的分片上传（碎片）摘要；Size 保持 SDK 返回值（既有语义下恒为 0）。
type MultipartUploadInfo struct {
	Key       string
	UploadID  string
	Initiated time.Time
	Size      int64
}

// UploadsPage 碎片列举页。
type UploadsPage struct {
	Uploads            []MultipartUploadInfo
	IsTruncated        bool
	NextKeyMarker      string
	NextUploadIDMarker string
}

// LifecycleRule 生命周期规则（Days 类配置的最小集）。
type LifecycleRule struct {
	ID                 string
	Status             string
	Prefix             string
	ExpirationDays     int
	AbortMultipartDays int
}

// CORSRule 跨域规则。
type CORSRule struct {
	ID             string
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposeHeaders  []string
	MaxAgeSeconds  int
}

// 版本控制状态（与 S3 BucketVersioningConfiguration.Status 取值一致）。
const (
	VersioningEnabled   = "Enabled"
	VersioningSuspended = "Suspended"
)

// Adapter 对象存储数据面接口（36 方法 / 7 组）。
//
// 归一化约定：Bucket 配置类的「未设置」态（404 且非 NoSuchBucket，或历史已知空态码）
// 由适配器归一化为空值/零值返回，不抛错；其余错误统一包装为 *StorageError，
// 控制层通过 ErrorCode/IsNotFound 判定后映射为产品错误。
type Adapter interface {
	// —— A. Bucket 基础（3）——

	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	CreateBucket(ctx context.Context, bucket, region string) error
	DeleteBucket(ctx context.Context, bucket string) error

	// —— B. 流式列举（2；channel 迭代器，元素携带 Err）——

	ListObjectsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream
	ListVersionsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream

	// —— C. 分页列举（2）——

	ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken, delimiter string, pageSize int) (*ObjectPage, error)
	ListObjectsByMarker(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error)

	// —— D. 对象读写（7）——

	StatObject(ctx context.Context, bucket, key, versionID string) (*ObjectStat, error)
	GetObject(ctx context.Context, bucket, key string) (*ObjectReader, error)
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
	CopyObject(ctx context.Context, src CopySrc, dest CopyDest) error
	RemoveObject(ctx context.Context, bucket, key, versionID string) error
	// DeleteObjectsBatch 批量删除：从 refs 持续读取删除项，失败明细从返回通道流出；
	// 调用方关闭 refs（或取消 ctx）即收尾。成功的对象不产生条目。
	DeleteObjectsBatch(ctx context.Context, bucket string, refs <-chan DeleteRef) DeleteErrorStream
	PresignGetObject(ctx context.Context, bucket, key string, expires time.Duration) (string, error)

	// —— E. 对象标签与 ACL（3）——

	GetObjectTags(ctx context.Context, bucket, key, versionID string) (map[string]string, error)
	// SetObjectTags 覆盖写入对象标签；空 map 表示清除全部标签。
	SetObjectTags(ctx context.Context, bucket, key, versionID string, tagMap map[string]string) error
	GetObjectACL(ctx context.Context, bucket, key string) (*ObjectStat, error)

	// —— F. Bucket 配置（13；「未设置」态归一化为空值）——

	GetBucketPolicy(ctx context.Context, bucket string) (string, error)
	// SetBucketPolicy 保存策略原文；空串表示删除策略。
	SetBucketPolicy(ctx context.Context, bucket, policy string) error
	// GetBucketVersioning 返回版本控制状态（"" / "Enabled" / "Suspended"）；错误不做归一化。
	GetBucketVersioning(ctx context.Context, bucket string) (string, error)
	SetBucketVersioning(ctx context.Context, bucket, status string) error
	GetBucketEncryption(ctx context.Context, bucket string) (bool, error)
	SetBucketEncryption(ctx context.Context, bucket string) error
	RemoveBucketEncryption(ctx context.Context, bucket string) error
	// GetBucketTagging 返回桶标签；未配置时返回 (nil, nil)。
	GetBucketTagging(ctx context.Context, bucket string) (map[string]string, error)
	SetBucketTagging(ctx context.Context, bucket string, tagMap map[string]string) error
	GetBucketLifecycle(ctx context.Context, bucket string) ([]LifecycleRule, error)
	// SetBucketLifecycle 覆盖写入规则；空规则集合等于删除配置。
	SetBucketLifecycle(ctx context.Context, bucket string, rules []LifecycleRule) error
	GetBucketCors(ctx context.Context, bucket string) ([]CORSRule, error)
	// SetBucketCors 覆盖写入规则；空规则集合等于删除 CORS 配置。
	SetBucketCors(ctx context.Context, bucket string, rules []CORSRule) error

	// —— G. 分片上传（6）——

	InitMultipart(ctx context.Context, bucket, key, contentType, storageClass string) (string, error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*PartInfo, error)
	ListParts(ctx context.Context, bucket, key, uploadID string, partNumberMarker, maxParts int) (*PartsPage, error)
	CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompletedObject, error)
	AbortMultipart(ctx context.Context, bucket, key, uploadID string) error
	// ListMultipartUploads 列举未完成的分片上传（碎片）；maxUploads 为 0 时不下发 max-uploads 参数。
	ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (*UploadsPage, error)
}

// New 按配置构造适配器。
// 已就绪的原生适配器按 provider 分派；其余厂商与未知取值回落 generic（minio-go）实现。
func New(cfg Config) (Adapter, error) {
	switch normalizeProviderKey(cfg.Provider) {
	case "cos":
		return newCos(cfg)
	case "oss":
		return newOSS(cfg)
	case "obs":
		return newOBS(cfg)
	case "aws":
		return newAWS(cfg)
	}
	// s3 / minio 与未知取值回落 generic（minio-go）实现
	return newGeneric(cfg)
}

// normalizeProviderKey 归一化厂商标识（与控制层 providerFor 的取值口径一致）。
func normalizeProviderKey(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = "s3"
	}
	return key
}
