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
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/s3utils"
	"github.com/minio/minio-go/v7/pkg/sse"
	"github.com/minio/minio-go/v7/pkg/tags"
)

// generic 是基于 minio-go 的通用 S3 实现：服务「其他 S3 兼容」与自建存储，
// 也是各厂商原生适配器就绪前的统一实现。
// 迁移自控制层原有的 minio-go 直调：调用顺序、参数与错误形态保持一致，不做顺手修正。
type generic struct {
	client *minio.Client
}

// NewGeneric 构造 minio-go 客户端（通用 S3 兼容 / MinIO / 未知厂商的兜底实现）。
// 寻址方式与签名版本由控制层解析后经 Config 传入，这里不再推导。
func NewGeneric(cfg Config) (Adapter, error) {
	opts := &minio.Options{
		Creds:        newStorageCredentials(cfg.SignatureVersion, cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		Secure:       cfg.Secure,
		Region:       cfg.Region,
		BucketLookup: cfg.BucketLookup,
	}
	if cfg.Transport != nil {
		// 条件赋值而非字面量：避免 typed-nil（*http.Transport）塞入接口后非空判定失效
		opts.Transport = cfg.Transport
	}
	client, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return nil, wrapError(err)
	}
	return &generic{client: client}, nil
}

// newStorageCredentials 按签名版本构造凭据。S3 V2 不支持临时令牌，token 被忽略。
func newStorageCredentials(version, access, secret, token string) *credentials.Credentials {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "v2", "s3v2":
		return credentials.NewStaticV2(access, secret, "")
	default:
		return credentials.NewStaticV4(access, secret, token)
	}
}

// wrapError 把 SDK 错误包装为 *StorageError：就地提取错误码与 HTTP 状态码。
// 错误文本保持委托原始错误（见 StorageError.Error），与直接使用 SDK 错误的文案逐字一致。
func wrapError(err error) error {
	if err == nil {
		return nil
	}
	er := minio.ToErrorResponse(err)
	return &StorageError{Code: er.Code, Status: er.StatusCode, Err: err}
}

// —— A. Bucket 基础 ——

func (g *generic) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	buckets, err := g.client.ListBuckets(ctx)
	if err != nil {
		return nil, wrapError(err)
	}
	out := make([]BucketInfo, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, BucketInfo{Name: b.Name, CreationDate: b.CreationDate})
	}
	return out, nil
}

func (g *generic) CreateBucket(ctx context.Context, bucket, region string) error {
	if err := g.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) DeleteBucket(ctx context.Context, bucket string) error {
	if err := g.client.RemoveBucket(ctx, bucket); err != nil {
		return wrapError(err)
	}
	return nil
}

// —— B. 流式列举 ——

func (g *generic) ListObjectsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	return forwardObjectStream(ctx, g.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    opts.Prefix,
		Recursive: opts.Recursive,
		MaxKeys:   opts.MaxKeys,
	}))
}

func (g *generic) ListVersionsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	return forwardObjectStream(ctx, g.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:       opts.Prefix,
		Recursive:    opts.Recursive,
		MaxKeys:      opts.MaxKeys,
		WithVersions: true,
	}))
}

// forwardObjectStream 把 SDK 的对象通道转换为适配层通道，保持逐元素转发与 ctx 取消语义。
func forwardObjectStream(ctx context.Context, src <-chan minio.ObjectInfo) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		for item := range src {
			entry := ObjectEntry{ObjectInfo: toObjectInfo(item), Err: wrapError(item.Err)}
			select {
			case out <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// —— C. 分页列举 ——

func (g *generic) ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken, delimiter string, pageSize int) (*ObjectPage, error) {
	// ctx 不透传：与 minio-go Core 列举（内部 context.Background）保持一致，请求行为与改造前相同
	core := minio.Core{Client: g.client}
	res, err := core.ListObjectsV2(bucket, prefix, "", continuationToken, delimiter, pageSize)
	if err != nil {
		return nil, wrapError(err)
	}
	return toObjectPage(res.Contents, res.CommonPrefixes, res.IsTruncated, res.NextContinuationToken), nil
}

func (g *generic) ListObjectsByMarker(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	core := minio.Core{Client: g.client}
	res, err := core.ListObjects(bucket, prefix, marker, delimiter, pageSize)
	if err != nil {
		return nil, wrapError(err)
	}
	return toObjectPage(res.Contents, res.CommonPrefixes, res.IsTruncated, res.NextMarker), nil
}

func toObjectPage(contents []minio.ObjectInfo, prefixes []minio.CommonPrefix, truncated bool, nextToken string) *ObjectPage {
	page := &ObjectPage{
		Contents:    make([]ObjectInfo, 0, len(contents)),
		IsTruncated: truncated,
		NextToken:   nextToken,
	}
	for _, cp := range prefixes {
		page.CommonPrefixes = append(page.CommonPrefixes, cp.Prefix)
	}
	for _, o := range contents {
		page.Contents = append(page.Contents, toObjectInfo(o))
	}
	return page
}

func toObjectInfo(o minio.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:            o.Key,
		Size:           o.Size,
		LastModified:   o.LastModified,
		ETag:           o.ETag,
		VersionID:      o.VersionID,
		IsLatest:       o.IsLatest,
		IsDeleteMarker: o.IsDeleteMarker,
		StorageClass:   o.StorageClass,
	}
}

// —— D. 对象读写 ——

func (g *generic) StatObject(ctx context.Context, bucket, key, versionID string) (*ObjectStat, error) {
	info, err := g.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{VersionID: versionID})
	if err != nil {
		return nil, wrapError(err)
	}
	return toObjectStat(info), nil
}

func toObjectStat(info minio.ObjectInfo) *ObjectStat {
	return &ObjectStat{
		Key:          info.Key,
		VersionID:    info.VersionID,
		Size:         info.Size,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
		LastModified: info.LastModified,
		Expires:      info.Expires,
		StorageClass: info.StorageClass,
		Metadata:     info.Metadata,
		UserMetadata: info.UserMetadata,
	}
}

func (g *generic) GetObject(ctx context.Context, bucket, key string) (*ObjectReader, error) {
	// 仅构造句柄，不发起请求（对象与桶名的本地校验保持 SDK 客户端层行为）；
	// 元数据读取由调用方按需走 StatObject，请求序列与改造前一致
	obj, err := g.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, wrapError(err)
	}
	return &ObjectReader{Body: obj}, nil
}

func (g *generic) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	if _, err := g.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) CopyObject(ctx context.Context, src CopySrc, dest CopyDest) error {
	userMeta := dest.UserMetadata
	if dest.StorageClass != "" || dest.SSEAlgorithm != "" || dest.SSEKMSKeyID != "" {
		merged := make(map[string]string, len(dest.UserMetadata)+3)
		for k, v := range dest.UserMetadata {
			merged[k] = v
		}
		// minio-go 对以 X-Amz- 开头的用户元数据键按系统头直通（与改造前的隐式约定一致）
		if dest.StorageClass != "" {
			merged["X-Amz-Storage-Class"] = dest.StorageClass
		}
		if dest.SSEAlgorithm != "" {
			merged["X-Amz-Server-Side-Encryption"] = dest.SSEAlgorithm
		}
		if dest.SSEKMSKeyID != "" {
			merged["X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id"] = dest.SSEKMSKeyID
		}
		userMeta = merged
	}
	destOpts := minio.CopyDestOptions{
		Bucket:             dest.Bucket,
		Object:             dest.Object,
		ReplaceMetadata:    dest.ReplaceMetadata,
		ContentType:        dest.ContentType,
		ContentEncoding:    dest.ContentEncoding,
		ContentDisposition: dest.ContentDisposition,
		ContentLanguage:    dest.ContentLanguage,
		CacheControl:       dest.CacheControl,
		Expires:            dest.Expires,
		UserMetadata:       userMeta,
	}
	srcOpts := minio.CopySrcOptions{Bucket: src.Bucket, Object: src.Object, VersionID: src.VersionID}
	if _, err := g.client.CopyObject(ctx, destOpts, srcOpts); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) RemoveObject(ctx context.Context, bucket, key, versionID string) error {
	if err := g.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{VersionID: versionID}); err != nil {
		return wrapError(err)
	}
	return nil
}

// DeleteObjectsBatch 批量删除：代理 goroutine 把适配层删除项转换为 SDK 对象流，
// 失败明细保持 SDK 的输出顺序流出（成功对象不产生条目）。
func (g *generic) DeleteObjectsBatch(ctx context.Context, bucket string, refs <-chan DeleteRef) DeleteErrorStream {
	in := make(chan minio.ObjectInfo, 1000)
	go func() {
		defer close(in)
		for ref := range refs {
			in <- minio.ObjectInfo{Key: ref.Key, VersionID: ref.VersionID}
		}
	}()

	out := make(chan DeleteError, 1)
	go func() {
		defer close(out)
		for result := range g.client.RemoveObjects(ctx, bucket, in, minio.RemoveObjectsOptions{}) {
			out <- DeleteError{Key: result.ObjectName, Err: wrapError(result.Err)}
		}
	}()
	return out
}

func (g *generic) PresignGetObject(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	u, err := g.client.PresignedGetObject(ctx, bucket, key, expires, nil)
	if err != nil {
		return "", wrapError(err)
	}
	return u.String(), nil
}

// —— E. 对象标签与 ACL ——

func (g *generic) GetObjectTags(ctx context.Context, bucket, key, versionID string) (map[string]string, error) {
	tagSet, err := g.client.GetObjectTagging(ctx, bucket, key, minio.GetObjectTaggingOptions{VersionID: versionID})
	if err != nil {
		return nil, wrapError(err)
	}
	if tagSet == nil {
		return map[string]string{}, nil
	}
	return tagSet.ToMap(), nil
}

func (g *generic) SetObjectTags(ctx context.Context, bucket, key, versionID string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		if err := g.client.RemoveObjectTagging(ctx, bucket, key, minio.RemoveObjectTaggingOptions{VersionID: versionID}); err != nil {
			return wrapError(err)
		}
		return nil
	}
	tagSet, err := tags.NewTags(tagMap, true)
	if err != nil {
		// 标签输入不合法（用户输入问题）：标记 InvalidTag，控制层映射为 400
		return &StorageError{Code: CodeInvalidTag, Status: http.StatusBadRequest, Err: err}
	}
	if err := g.client.PutObjectTagging(ctx, bucket, key, tagSet, minio.PutObjectTaggingOptions{VersionID: versionID}); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) GetObjectACL(ctx context.Context, bucket, key string) (*ObjectStat, error) {
	info, err := g.client.GetObjectACL(ctx, bucket, key)
	if err != nil {
		return nil, wrapError(err)
	}
	if info == nil {
		return nil, nil
	}
	stat := toObjectStat(*info)
	// minio 的 ACL 响应把 canned ACL 放在 X-Amz-Acl 响应头：转成显式字段供控制层读取
	stat.CannedACL = stat.Metadata.Get("X-Amz-Acl")
	return stat, nil
}

// —— F. Bucket 配置 ——
// 「未设置」态一律归一化为空值返回（不抛错）：错误码命中历史空态码，或
// HTTP 404 且非 NoSuchBucket（桶不存在是真实错误）。
// 注意：minio-go 的 GetBucketPolicy/GetBucketCors 已在客户端层吞掉各自空态并返回空值。

func (g *generic) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	policy, err := g.client.GetBucketPolicy(ctx, bucket)
	if err != nil {
		if absentCodes(wrapError(err), "NoSuchBucketPolicy") {
			return "", nil
		}
		return "", wrapError(err)
	}
	return policy, nil
}

func (g *generic) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	if err := g.client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) GetBucketVersioning(ctx context.Context, bucket string) (string, error) {
	cfg, err := g.client.GetBucketVersioning(ctx, bucket)
	if err != nil {
		// 版本控制错误不归一化：维持「读取失败即报错」的既有语义
		return "", wrapError(err)
	}
	return cfg.Status, nil
}

func (g *generic) SetBucketVersioning(ctx context.Context, bucket, status string) error {
	if err := g.client.SetBucketVersioning(ctx, bucket, minio.BucketVersioningConfiguration{Status: status}); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) GetBucketEncryption(ctx context.Context, bucket string) (bool, error) {
	enc, err := g.client.GetBucketEncryption(ctx, bucket)
	if err != nil {
		if absentCodes(wrapError(err), "ServerSideEncryptionConfigurationNotFoundError") {
			return false, nil
		}
		return false, wrapError(err)
	}
	return enc != nil && len(enc.Rules) > 0, nil
}

func (g *generic) SetBucketEncryption(ctx context.Context, bucket string) error {
	if err := g.client.SetBucketEncryption(ctx, bucket, sse.NewConfigurationSSES3()); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) RemoveBucketEncryption(ctx context.Context, bucket string) error {
	if err := g.client.RemoveBucketEncryption(ctx, bucket); err != nil {
		if absentCodes(wrapError(err), "ServerSideEncryptionConfigurationNotFoundError") {
			return nil
		}
		return wrapError(err)
	}
	return nil
}

func (g *generic) GetBucketTagging(ctx context.Context, bucket string) (map[string]string, error) {
	tagSet, err := g.client.GetBucketTagging(ctx, bucket)
	if err != nil {
		if absentCodes(wrapError(err), "NoSuchTagSet") {
			return nil, nil
		}
		return nil, wrapError(err)
	}
	if tagSet == nil {
		return map[string]string{}, nil
	}
	return tagSet.ToMap(), nil
}

func (g *generic) SetBucketTagging(ctx context.Context, bucket string, tagMap map[string]string) error {
	// failOnExist=false：桶标签是覆盖写（对象标签为 true，保持各自既有语义）
	tagSet, err := tags.NewTags(tagMap, false)
	if err != nil {
		// 标签输入不合法（用户输入问题）：标记 InvalidTag，控制层映射为 400
		return &StorageError{Code: CodeInvalidTag, Status: http.StatusBadRequest, Err: err}
	}
	if err := g.client.SetBucketTagging(ctx, bucket, tagSet); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) GetBucketLifecycle(ctx context.Context, bucket string) ([]LifecycleRule, error) {
	cfg, err := g.client.GetBucketLifecycle(ctx, bucket)
	if err != nil {
		if absentCodes(wrapError(err), "NoSuchLifecycleConfiguration") {
			return nil, nil
		}
		return nil, wrapError(err)
	}
	return fromLifecycleConfig(cfg), nil
}

func (g *generic) SetBucketLifecycle(ctx context.Context, bucket string, rules []LifecycleRule) error {
	// 空规则集合由 SDK 转换为「删除配置」请求，与改造前一致
	if err := g.client.SetBucketLifecycle(ctx, bucket, toLifecycleConfig(rules)); err != nil {
		return wrapError(err)
	}
	return nil
}

func fromLifecycleConfig(cfg *lifecycle.Configuration) []LifecycleRule {
	rules := make([]LifecycleRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		rules = append(rules, LifecycleRule{
			ID:                 r.ID,
			Status:             r.Status,
			Prefix:             r.RuleFilter.Prefix,
			ExpirationDays:     int(r.Expiration.Days),
			AbortMultipartDays: int(r.AbortIncompleteMultipartUpload.DaysAfterInitiation),
		})
	}
	return rules
}

func toLifecycleConfig(rules []LifecycleRule) *lifecycle.Configuration {
	cfg := lifecycle.NewConfiguration()
	for _, r := range rules {
		rule := lifecycle.Rule{
			ID:         r.ID,
			Status:     r.Status,
			RuleFilter: lifecycle.Filter{Prefix: r.Prefix},
		}
		if r.ExpirationDays > 0 {
			rule.Expiration = lifecycle.Expiration{Days: lifecycle.ExpirationDays(r.ExpirationDays)}
		}
		if r.AbortMultipartDays > 0 {
			rule.AbortIncompleteMultipartUpload = lifecycle.AbortIncompleteMultipartUpload{DaysAfterInitiation: lifecycle.ExpirationDays(r.AbortMultipartDays)}
		}
		cfg.Rules = append(cfg.Rules, rule)
	}
	return cfg
}

func (g *generic) GetBucketCors(ctx context.Context, bucket string) ([]CORSRule, error) {
	cfg, err := g.client.GetBucketCors(ctx, bucket)
	if err != nil {
		if absentCodes(wrapError(err), "NoSuchCORSConfiguration") {
			return nil, nil
		}
		return nil, wrapError(err)
	}
	if cfg == nil {
		return nil, nil
	}
	return fromCorsConfig(cfg), nil
}

func (g *generic) SetBucketCors(ctx context.Context, bucket string, rules []CORSRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除 CORS 配置（nil 配置由 SDK 转换为删除请求）
		if err := g.client.SetBucketCors(ctx, bucket, nil); err != nil {
			if absentCodes(wrapError(err), "NoSuchCORSConfiguration") {
				return nil
			}
			return wrapError(err)
		}
		return nil
	}
	if err := g.client.SetBucketCors(ctx, bucket, toCorsConfig(rules)); err != nil {
		return wrapError(err)
	}
	return nil
}

func fromCorsConfig(cfg *cors.Config) []CORSRule {
	rules := make([]CORSRule, 0, len(cfg.CORSRules))
	for _, r := range cfg.CORSRules {
		rules = append(rules, CORSRule{
			ID:             r.ID,
			AllowedOrigins: r.AllowedOrigin,
			AllowedMethods: r.AllowedMethod,
			AllowedHeaders: r.AllowedHeader,
			ExposeHeaders:  r.ExposeHeader,
			MaxAgeSeconds:  r.MaxAgeSeconds,
		})
	}
	return rules
}

func toCorsConfig(rules []CORSRule) *cors.Config {
	corsRules := make([]cors.Rule, 0, len(rules))
	for _, r := range rules {
		corsRules = append(corsRules, cors.Rule{
			ID:            r.ID,
			AllowedOrigin: r.AllowedOrigins,
			AllowedMethod: r.AllowedMethods,
			AllowedHeader: r.AllowedHeaders,
			ExposeHeader:  r.ExposeHeaders,
			MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	return cors.NewConfig(corsRules)
}

// —— G. 分片上传 ——

func (g *generic) InitMultipart(ctx context.Context, bucket, key, contentType, storageClass string) (string, error) {
	core := minio.Core{Client: g.client}
	uploadID, err := core.NewMultipartUpload(ctx, bucket, key, minio.PutObjectOptions{
		ContentType:  contentType,
		StorageClass: storageClass,
	})
	if err != nil {
		return "", wrapError(err)
	}
	return uploadID, nil
}

func (g *generic) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*PartInfo, error) {
	core := minio.Core{Client: g.client}
	part, err := core.PutObjectPart(ctx, bucket, key, uploadID, partNumber, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		return nil, wrapError(err)
	}
	info := toPartInfo(part)
	return &info, nil
}

func (g *generic) ListParts(ctx context.Context, bucket, key, uploadID string, partNumberMarker, maxParts int) (*PartsPage, error) {
	core := minio.Core{Client: g.client}
	result, err := core.ListObjectParts(ctx, bucket, key, uploadID, partNumberMarker, maxParts)
	if err != nil {
		return nil, wrapError(err)
	}
	return toPartsPage(result), nil
}

func toPartInfo(part minio.ObjectPart) PartInfo {
	return PartInfo{
		PartNumber:   part.PartNumber,
		ETag:         part.ETag,
		Size:         part.Size,
		LastModified: part.LastModified,
	}
}

func toPartsPage(result minio.ListObjectPartsResult) *PartsPage {
	page := &PartsPage{
		IsTruncated:          result.IsTruncated,
		NextPartNumberMarker: result.NextPartNumberMarker,
	}
	page.Parts = make([]PartInfo, 0, len(result.ObjectParts))
	for _, part := range result.ObjectParts {
		page.Parts = append(page.Parts, toPartInfo(part))
	}
	return page
}

func (g *generic) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompletedObject, error) {
	core := minio.Core{Client: g.client}
	completeParts := make([]minio.CompletePart, 0, len(parts))
	for _, p := range parts {
		completeParts = append(completeParts, minio.CompletePart{PartNumber: p.PartNumber, ETag: p.ETag})
	}
	info, err := core.CompleteMultipartUpload(ctx, bucket, key, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return nil, wrapError(err)
	}
	return &CompletedObject{Key: info.Key, ETag: info.ETag, Size: info.Size}, nil
}

func (g *generic) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	core := minio.Core{Client: g.client}
	if err := core.AbortMultipartUpload(ctx, bucket, key, uploadID); err != nil {
		return wrapError(err)
	}
	return nil
}

func (g *generic) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (*UploadsPage, error) {
	// 本地校验沿用 SDK 客户端层的同款规则，保证错误形态与改造前一致（Core 自身不校验）
	if err := s3utils.CheckValidBucketName(bucket); err != nil {
		return nil, wrapError(err)
	}
	if err := s3utils.CheckValidObjectNamePrefix(prefix); err != nil {
		return nil, wrapError(err)
	}
	core := minio.Core{Client: g.client}
	result, err := core.ListMultipartUploads(ctx, bucket, prefix, keyMarker, uploadIDMarker, "", maxUploads)
	if err != nil {
		return nil, wrapError(err)
	}
	return toUploadsPage(result), nil
}

func toUploadsPage(result minio.ListMultipartUploadsResult) *UploadsPage {
	page := &UploadsPage{
		IsTruncated:        result.IsTruncated,
		NextKeyMarker:      result.NextKeyMarker,
		NextUploadIDMarker: result.NextUploadIDMarker,
	}
	page.Uploads = make([]MultipartUploadInfo, 0, len(result.Uploads))
	for _, u := range result.Uploads {
		page.Uploads = append(page.Uploads, MultipartUploadInfo{
			Key:       u.Key,
			UploadID:  u.UploadID,
			Initiated: u.Initiated,
			Size:      u.Size,
		})
	}
	return page
}
