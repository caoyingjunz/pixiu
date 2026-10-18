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
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"k8s.io/klog/v2"
)

// ossAdapter 是阿里云 OSS 的原生适配器（alibabacloud-oss-go-sdk-v2）。
//
// 语义要点：
//   - OSS 同时有 V1（marker）与 V2（continuation-token）列举：分页原语按调用方语义直选；
//     版本列举用 SDK 的 IsMix 模式（文档序混排版本与删除标记，等价 S3 的交错顺序）。
//   - policy 是原生 JSON 原样透传（PutBucketPolicy 收 io.Reader，Get 回字符串），无结构损失。
//   - 批量删除的响应里 SDK 不解析逐对象 <Error> 明细（DeleteMultipleObjectsResult 无 Errors 字段）：
//     用 Verbose（非 Quiet）模式回传的 Deleted 集合做「请求项 - 成功项」差集，差集项按失败上报。
//   - 「未设置」态：错误码命中历史空态表，或 HTTP 404 且非 NoSuchBucket。
type ossAdapter struct {
	client *oss.Client
}

// ossS3CompatHostRe 存量数据源可能填的是 OSS 的 S3 兼容端点（s3.oss-<region>.aliyuncs.com）；
// 原生 SDK 需要走服务级端点。这里做等价重写（同 region 同协议、同一份数据），并留下告警便于对账。
var ossS3CompatHostRe = regexp.MustCompile(`^s3\.oss-([a-z0-9-]+)\.aliyuncs\.com$`)

// NewOSS 构造阿里云 OSS 原生适配器。
func NewOSS(cfg Config) (Adapter, error) {
	schema := "https"
	if !cfg.Secure {
		schema = "http"
	}
	host := ossNormalizeEndpoint(cfg.Endpoint)
	if host != cfg.Endpoint {
		klog.Warningf("storage(oss): endpoint %s 为 S3 兼容端点，已按原生 SDK 语义重写为 %s", cfg.Endpoint, host)
	}

	config := &oss.Config{
		Region:              oss.Ptr(cfg.Region),
		Endpoint:            oss.Ptr(schema + "://" + host),
		CredentialsProvider: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
	}
	if cfg.Transport != nil {
		// 条件赋值而非字面量：避免 typed-nil（*http.Transport）塞入接口后非空判定失效；
		// 不注入时由 SDK 的 resolveHTTPClient 接管默认传输层（生产路径 clientFor 从不传 Transport）
		config.HttpClient = &http.Client{Transport: cfg.Transport}
	}
	client := oss.NewClient(config)
	return &ossAdapter{client: client}, nil
}

// ossNormalizeEndpoint 把 S3 兼容端点重写为原生服务级端点；其余端点原样返回。
func ossNormalizeEndpoint(host string) string {
	if m := ossS3CompatHostRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(host))); m != nil {
		return "oss-" + m[1] + ".aliyuncs.com"
	}
	return host
}

// wrapOSSError 把 SDK 错误包装为 *StorageError（OperationError 链内可 errors.As 到 ServiceError）。
func wrapOSSError(err error) error {
	if err == nil {
		return nil
	}
	var se *oss.ServiceError
	if errors.As(err, &se) {
		return &StorageError{Code: se.Code, Status: se.StatusCode, Err: err}
	}
	var ce *oss.ClientError
	if errors.As(err, &ce) {
		return &StorageError{Code: ce.Code, Message: ce.Message, Err: err}
	}
	return err
}

// ossAbsentConfig 判断错误是否属于「配置未设置」：历史空态码，或 HTTP 404 且非 NoSuchBucket。
func ossAbsentConfig(err error, codes ...string) bool {
	return absentCodes(wrapOSSError(err), codes...)
}

// —— A. Bucket 基础 ——

func (a *ossAdapter) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	res, err := a.client.ListBuckets(ctx, &oss.ListBucketsRequest{})
	if err != nil {
		return nil, wrapOSSError(err)
	}
	out := make([]BucketInfo, 0, len(res.Buckets))
	for _, b := range res.Buckets {
		info := BucketInfo{Name: ossString(b.Name)}
		if b.CreationDate != nil {
			info.CreationDate = *b.CreationDate
		}
		out = append(out, info)
	}
	return out, nil
}

func (a *ossAdapter) CreateBucket(ctx context.Context, bucket, region string) error {
	// OSS 的 bucket 区域由端点承载（oss-<region>.aliyuncs.com），创建请求无需区域配置
	if _, err := a.client.PutBucket(ctx, &oss.PutBucketRequest{Bucket: oss.Ptr(bucket)}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) DeleteBucket(ctx context.Context, bucket string) error {
	if _, err := a.client.DeleteBucket(ctx, &oss.DeleteBucketRequest{Bucket: oss.Ptr(bucket)}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

// —— B. 流式列举 ——

// ossListPageSize 单页列举条数（OSS 上限 1000）
const ossListPageSize = 1000

func (a *ossAdapter) ListObjectsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		delimiter := ""
		if !opts.Recursive {
			delimiter = "/"
		}
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = ossListPageSize
		}
		token := ""
		for {
			req := &oss.ListObjectsV2Request{
				Bucket:  oss.Ptr(bucket),
				Prefix:  ossPtrString(opts.Prefix),
				MaxKeys: int32(maxKeys),
			}
			if delimiter != "" {
				req.Delimiter = oss.Ptr(delimiter)
			}
			if token != "" {
				req.ContinuationToken = oss.Ptr(token)
			}
			res, err := a.client.ListObjectsV2(ctx, req)
			if err != nil {
				sendOSSEntry(ctx, out, ObjectEntry{Err: wrapOSSError(err)})
				return
			}
			// 与 minio 流一致：先对象、后目录前缀
			for _, o := range res.Contents {
				if !sendOSSEntry(ctx, out, ObjectEntry{ObjectInfo: ossObjectInfo(o)}) {
					return
				}
			}
			for _, cp := range res.CommonPrefixes {
				if !sendOSSEntry(ctx, out, ObjectEntry{ObjectInfo: ObjectInfo{Key: ossString(cp.Prefix)}}) {
					return
				}
			}
			if !res.IsTruncated {
				return
			}
			next := ossString(res.NextContinuationToken)
			if next == "" || next == token {
				return
			}
			token = next
		}
	}()
	return out
}

func (a *ossAdapter) ListVersionsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = ossListPageSize
		}
		keyMarker, versionIDMarker := "", ""
		for {
			req := &oss.ListObjectVersionsRequest{
				Bucket:  oss.Ptr(bucket),
				Prefix:  ossPtrString(opts.Prefix),
				MaxKeys: int32(maxKeys),
				// IsMix：SDK 把 <Version> 与 <DeleteMarker> 按文档序混排进同一列表，
				// 保持与 S3 一致的交错顺序（等价 minio 流的输出次序）。
				IsMix: true,
			}
			if keyMarker != "" {
				req.KeyMarker = oss.Ptr(keyMarker)
			}
			if versionIDMarker != "" {
				req.VersionIdMarker = oss.Ptr(versionIDMarker)
			}
			res, err := a.client.ListObjectVersions(ctx, req)
			if err != nil {
				sendOSSEntry(ctx, out, ObjectEntry{Err: wrapOSSError(err)})
				return
			}
			for _, entry := range res.ObjectVersionsDeleteMarkers {
				info := ossMixedVersionInfo(entry)
				if !sendOSSEntry(ctx, out, ObjectEntry{ObjectInfo: info}) {
					return
				}
			}
			if !res.IsTruncated {
				return
			}
			nextKey, nextVersion := ossString(res.NextKeyMarker), ossString(res.NextVersionIdMarker)
			if nextKey == keyMarker && nextVersion == versionIDMarker {
				return
			}
			if nextKey == "" && nextVersion == "" {
				return
			}
			keyMarker, versionIDMarker = nextKey, nextVersion
		}
	}()
	return out
}

func sendOSSEntry(ctx context.Context, out chan<- ObjectEntry, entry ObjectEntry) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

// ossObjectInfo 映射 V1/V2 列举的对象条目（ETag 去引号，与 minio 路径的规整行为一致）。
func ossObjectInfo(o oss.ObjectProperties) ObjectInfo {
	info := ObjectInfo{
		Key:          ossString(o.Key),
		Size:         o.Size,
		ETag:         trimOSSEtag(ossString(o.ETag)),
		StorageClass: ossString(o.StorageClass),
	}
	if o.LastModified != nil {
		info.LastModified = *o.LastModified
	}
	return info
}

// ossMixedVersionInfo 映射 IsMix 模式的版本/删除标记条目。
// 删除标记的判定不用 SDK 的 IsDeleteMarker()（其启发式「VersionId 非空且 Type 为空」会把
// 无 Type 字段的历史版本条目误判为删除标记，SDK 自带 fixture 即有该形态）；
// 删除标记的真实特征是：无 Type、无 ETag、Size 为 0（版本条目即便是空对象也必有 ETag）。
func ossMixedVersionInfo(v oss.ObjectMixProperties) ObjectInfo {
	info := ObjectInfo{
		Key:            ossString(v.Key),
		VersionID:      ossString(v.VersionId),
		IsLatest:       v.IsLatest,
		IsDeleteMarker: v.Type == nil && v.ETag == nil && v.Size == 0,
	}
	if !info.IsDeleteMarker {
		info.Size = v.Size
		info.ETag = trimOSSEtag(ossString(v.ETag))
		info.StorageClass = ossString(v.StorageClass)
	}
	if v.LastModified != nil {
		info.LastModified = *v.LastModified
	}
	return info
}

// —— C. 分页列举 ——

func (a *ossAdapter) ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken, delimiter string, pageSize int) (*ObjectPage, error) {
	if pageSize <= 0 {
		pageSize = ossListPageSize
	}
	req := &oss.ListObjectsV2Request{
		Bucket:  oss.Ptr(bucket),
		Prefix:  ossPtrString(prefix),
		MaxKeys: int32(pageSize),
	}
	if continuationToken != "" {
		req.ContinuationToken = oss.Ptr(continuationToken)
	}
	if delimiter != "" {
		req.Delimiter = oss.Ptr(delimiter)
	}
	res, err := a.client.ListObjectsV2(ctx, req)
	if err != nil {
		return nil, wrapOSSError(err)
	}
	page := &ObjectPage{
		Contents:       make([]ObjectInfo, 0, len(res.Contents)),
		IsTruncated:    res.IsTruncated,
		NextToken:      ossString(res.NextContinuationToken),
		CommonPrefixes: make([]string, 0, len(res.CommonPrefixes)),
	}
	for _, o := range res.Contents {
		page.Contents = append(page.Contents, ossObjectInfo(o))
	}
	for _, cp := range res.CommonPrefixes {
		page.CommonPrefixes = append(page.CommonPrefixes, ossString(cp.Prefix))
	}
	return page, nil
}

func (a *ossAdapter) ListObjectsByMarker(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	if pageSize <= 0 {
		pageSize = ossListPageSize
	}
	req := &oss.ListObjectsRequest{
		Bucket:  oss.Ptr(bucket),
		Prefix:  ossPtrString(prefix),
		MaxKeys: int32(pageSize),
	}
	if marker != "" {
		req.Marker = oss.Ptr(marker)
	}
	if delimiter != "" {
		req.Delimiter = oss.Ptr(delimiter)
	}
	res, err := a.client.ListObjects(ctx, req)
	if err != nil {
		return nil, wrapOSSError(err)
	}
	page := &ObjectPage{
		Contents:       make([]ObjectInfo, 0, len(res.Contents)),
		IsTruncated:    res.IsTruncated,
		NextToken:      ossString(res.NextMarker),
		CommonPrefixes: make([]string, 0, len(res.CommonPrefixes)),
	}
	for _, o := range res.Contents {
		page.Contents = append(page.Contents, ossObjectInfo(o))
	}
	for _, cp := range res.CommonPrefixes {
		page.CommonPrefixes = append(page.CommonPrefixes, ossString(cp.Prefix))
	}
	return page, nil
}

// —— D. 对象读写 ——

func (a *ossAdapter) StatObject(ctx context.Context, bucket, key, versionID string) (*ObjectStat, error) {
	req := &oss.HeadObjectRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)}
	if versionID != "" {
		req.VersionId = oss.Ptr(versionID)
	}
	res, err := a.client.HeadObject(ctx, req)
	if err != nil {
		return nil, wrapOSSError(err)
	}
	stat := &ObjectStat{
		Key:          key,
		VersionID:    versionID,
		Size:         res.ContentLength,
		ETag:         trimOSSEtag(ossString(res.ETag)),
		ContentType:  ossString(res.ContentType),
		StorageClass: ossString(res.StorageClass),
		Metadata:     res.Headers,
		UserMetadata: res.Metadata,
	}
	if res.LastModified != nil {
		stat.LastModified = *res.LastModified
	}
	if stat.VersionID == "" {
		stat.VersionID = res.Headers.Get("x-oss-version-id")
	}
	if stat.Metadata == nil {
		stat.Metadata = http.Header{}
	}
	if stat.UserMetadata == nil {
		stat.UserMetadata = map[string]string{}
	}
	// 控制器按原始头读取 Content-Encoding / Cache-Control 等：Headers 缺失时兜底补齐常用项
	return stat, nil
}

func (a *ossAdapter) GetObject(ctx context.Context, bucket, key string) (*ObjectReader, error) {
	res, err := a.client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)})
	if err != nil {
		return nil, wrapOSSError(err)
	}
	return &ObjectReader{Body: res.Body}, nil
}

func (a *ossAdapter) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	req := &oss.PutObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(key),
		Body:   reader,
	}
	if size > 0 {
		req.ContentLength = oss.Ptr(size)
	}
	if strings.TrimSpace(contentType) != "" {
		req.ContentType = oss.Ptr(contentType)
	}
	if _, err := a.client.PutObject(ctx, req); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) CopyObject(ctx context.Context, src CopySrc, dest CopyDest) error {
	req := &oss.CopyObjectRequest{
		Bucket:          oss.Ptr(dest.Bucket),
		Key:             oss.Ptr(dest.Object),
		SourceBucket:    oss.Ptr(src.Bucket),
		SourceKey:       oss.Ptr(src.Object),
		SourceVersionId: ossPtrString(src.VersionID),
	}
	if dest.ReplaceMetadata {
		// OSS 的元数据替换指令为 REPLACE（默认 COPY 复制源元数据）
		req.MetadataDirective = oss.Ptr("REPLACE")
		req.ContentType = ossPtrString(dest.ContentType)
		req.ContentEncoding = ossPtrString(dest.ContentEncoding)
		req.ContentDisposition = ossPtrString(dest.ContentDisposition)
		req.CacheControl = ossPtrString(dest.CacheControl)
		if !dest.Expires.IsZero() {
			req.Expires = oss.Ptr(dest.Expires.UTC().Format(http.TimeFormat))
		}
		// 系统元数据由 CopyDest 显式字段承载，UserMetadata 只放用户自定义项
		if dest.StorageClass != "" {
			req.StorageClass = oss.StorageClassType(dest.StorageClass)
		}
		req.ServerSideEncryption = ossPtrString(dest.SSEAlgorithm)
		req.ServerSideEncryptionKeyId = ossPtrString(dest.SSEKMSKeyID)
		if len(dest.UserMetadata) > 0 {
			req.Metadata = dest.UserMetadata
		}
	}
	if _, err := a.client.CopyObject(ctx, req); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) RemoveObject(ctx context.Context, bucket, key, versionID string) error {
	req := &oss.DeleteObjectRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)}
	if versionID != "" {
		req.VersionId = oss.Ptr(versionID)
	}
	if _, err := a.client.DeleteObject(ctx, req); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

// ossDeleteBatchSize 单次批量删除的对象数上限（OSS/S3 一致为 1000）
const ossDeleteBatchSize = 1000

func (a *ossAdapter) DeleteObjectsBatch(ctx context.Context, bucket string, refs <-chan DeleteRef) DeleteErrorStream {
	out := make(chan DeleteError, 1)
	go func() {
		defer close(out)
		batch := make([]oss.ObjectIdentifier, 0, ossDeleteBatchSize)
		flush := func() bool {
			if len(batch) == 0 {
				return true
			}
			requested := batch
			batch = batch[:0]

			// Verbose 模式：Deleted 集合覆盖全部成功项（含「删除不存在的 key」这类幂等成功）
			res, err := a.client.DeleteMultipleObjects(ctx, &oss.DeleteMultipleObjectsRequest{
				Bucket: oss.Ptr(bucket),
				Delete: &oss.Delete{Objects: requested, Quiet: false},
			})
			if err != nil {
				name := requested[0].Key
				return sendOSSDeleteError(ctx, out, DeleteError{
					Key: ossString(name),
					Err: wrapOSSError(err),
				})
			}
			// SDK 不解析逐对象 <Error> 明细：以「请求项 - 成功项」差集识别失败项
			succeeded := make(map[string]struct{}, len(res.DeletedObjects))
			for _, d := range res.DeletedObjects {
				succeeded[ossString(d.Key)+"\x00"+ossString(d.VersionId)] = struct{}{}
			}
			for _, item := range requested {
				key := ossString(item.Key)
				if _, ok := succeeded[key+"\x00"+ossString(item.VersionId)]; ok {
					continue
				}
				entry := DeleteError{
					Key: key,
					Err: &StorageError{
						Code:    "DeleteObjectFailed",
						Message: fmt.Sprintf("object %s was not deleted (no per-object error detail from OSS SDK)", key),
					},
				}
				if !sendOSSDeleteError(ctx, out, entry) {
					return false
				}
			}
			return true
		}
		for {
			select {
			case ref, ok := <-refs:
				if !ok {
					flush()
					return
				}
				item := oss.ObjectIdentifier{Key: oss.Ptr(ref.Key)}
				if ref.VersionID != "" {
					item.VersionId = oss.Ptr(ref.VersionID)
				}
				batch = append(batch, item)
				if len(batch) >= ossDeleteBatchSize {
					if !flush() {
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func sendOSSDeleteError(ctx context.Context, out chan<- DeleteError, entry DeleteError) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

func (a *ossAdapter) PresignGetObject(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	res, err := a.client.Presign(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)}, oss.PresignExpires(expires))
	if err != nil {
		return "", wrapOSSError(err)
	}
	return res.URL, nil
}

// —— E. 对象标签与 ACL ——

func (a *ossAdapter) GetObjectTags(ctx context.Context, bucket, key, versionID string) (map[string]string, error) {
	req := &oss.GetObjectTaggingRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)}
	if versionID != "" {
		req.VersionId = oss.Ptr(versionID)
	}
	res, err := a.client.GetObjectTagging(ctx, req)
	if err != nil {
		if ossAbsentConfig(err, "NoSuchTagSet") {
			return map[string]string{}, nil
		}
		return nil, wrapOSSError(err)
	}
	return ossTagListToMap(res.Tags), nil
}

func (a *ossAdapter) SetObjectTags(ctx context.Context, bucket, key, versionID string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		req := &oss.DeleteObjectTaggingRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)}
		if versionID != "" {
			req.VersionId = oss.Ptr(versionID)
		}
		if _, err := a.client.DeleteObjectTagging(ctx, req); err != nil {
			return wrapOSSError(err)
		}
		return nil
	}
	if err := validateOSSTags(tagMap); err != nil {
		return err
	}
	req := &oss.PutObjectTaggingRequest{
		Bucket:  oss.Ptr(bucket),
		Key:     oss.Ptr(key),
		Tagging: ossTaggingOf(tagMap),
	}
	if versionID != "" {
		req.VersionId = oss.Ptr(versionID)
	}
	if _, err := a.client.PutObjectTagging(ctx, req); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

// ossMaxObjectTags 对象标签数量上限（S3/OSS 一致）
const ossMaxObjectTags = 10

// validateOSSTags 本地校验对象标签，命中用户输入问题时返回 InvalidTag（控制层映射为 400）。
func validateOSSTags(tagMap map[string]string) error {
	if len(tagMap) > ossMaxObjectTags {
		return &StorageError{
			Code: CodeInvalidTag, Status: http.StatusBadRequest,
			Message: fmt.Sprintf("too many object tags: %d, limit is %d", len(tagMap), ossMaxObjectTags),
		}
	}
	for k, v := range tagMap {
		if strings.TrimSpace(k) == "" {
			return &StorageError{Code: CodeInvalidTag, Status: http.StatusBadRequest, Message: "object tag key is required"}
		}
		if len(k) > 128 || len(v) > 256 {
			return &StorageError{
				Code: CodeInvalidTag, Status: http.StatusBadRequest,
				Message: fmt.Sprintf("object tag %s exceeds the length limit", k),
			}
		}
	}
	return nil
}

func (a *ossAdapter) GetObjectACL(ctx context.Context, bucket, key string) (*ObjectStat, error) {
	res, err := a.client.GetObjectAcl(ctx, &oss.GetObjectAclRequest{Bucket: oss.Ptr(bucket), Key: oss.Ptr(key)})
	if err != nil {
		return nil, wrapOSSError(err)
	}
	stat := &ObjectStat{Key: key, Metadata: http.Header{}}
	// OSS 的 GET acl 返回授权结构；优先取响应头里的 canned ACL，缺失时退回结构字段
	acl := res.Headers.Get("x-oss-acl")
	if acl == "" {
		acl = ossString(res.ACL)
	}
	stat.CannedACL = acl
	return stat, nil
}

// —— F. Bucket 配置 ——

func (a *ossAdapter) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	res, err := a.client.GetBucketPolicy(ctx, &oss.GetBucketPolicyRequest{Bucket: oss.Ptr(bucket)})
	if err != nil {
		if ossAbsentConfig(err, "NoSuchBucketPolicy") {
			return "", nil
		}
		return "", wrapOSSError(err)
	}
	if strings.TrimSpace(res.Body) == "" {
		return "", nil
	}
	return res.Body, nil
}

func (a *ossAdapter) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	if strings.TrimSpace(policy) == "" {
		if _, err := a.client.DeleteBucketPolicy(ctx, &oss.DeleteBucketPolicyRequest{Bucket: oss.Ptr(bucket)}); err != nil {
			if ossAbsentConfig(err, "NoSuchBucketPolicy") {
				return nil
			}
			return wrapOSSError(err)
		}
		return nil
	}
	// 契约是「策略原文透传」：OSS 的 put 收 io.Reader，天然无结构损失
	if _, err := a.client.PutBucketPolicy(ctx, &oss.PutBucketPolicyRequest{
		Bucket: oss.Ptr(bucket),
		Body:   strings.NewReader(policy),
	}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) GetBucketVersioning(ctx context.Context, bucket string) (string, error) {
	res, err := a.client.GetBucketVersioning(ctx, &oss.GetBucketVersioningRequest{Bucket: oss.Ptr(bucket)})
	if err != nil {
		// 版本控制错误不归一化：维持「读取失败即报错」的既有语义
		return "", wrapOSSError(err)
	}
	return ossString(res.VersionStatus), nil
}

func (a *ossAdapter) SetBucketVersioning(ctx context.Context, bucket, status string) error {
	_, err := a.client.PutBucketVersioning(ctx, &oss.PutBucketVersioningRequest{
		Bucket:                  oss.Ptr(bucket),
		VersioningConfiguration: &oss.VersioningConfiguration{Status: oss.VersioningStatusType(status)},
	})
	if err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) GetBucketEncryption(ctx context.Context, bucket string) (bool, error) {
	res, err := a.client.GetBucketEncryption(ctx, &oss.GetBucketEncryptionRequest{Bucket: oss.Ptr(bucket)})
	if err != nil {
		// OSS「未配置加密」的空态码是 NoSuchServerSideEncryptionRule（与 AWS 的措辞不同）
		if ossAbsentConfig(err, "NoSuchServerSideEncryptionRule", "ServerSideEncryptionConfigurationNotFoundError") {
			return false, nil
		}
		return false, wrapOSSError(err)
	}
	if res.ServerSideEncryptionRule == nil || res.ServerSideEncryptionRule.ApplyServerSideEncryptionByDefault == nil {
		return false, nil
	}
	return strings.TrimSpace(ossString(res.ServerSideEncryptionRule.ApplyServerSideEncryptionByDefault.SSEAlgorithm)) != "", nil
}

func (a *ossAdapter) SetBucketEncryption(ctx context.Context, bucket string) error {
	_, err := a.client.PutBucketEncryption(ctx, &oss.PutBucketEncryptionRequest{
		Bucket: oss.Ptr(bucket),
		ServerSideEncryptionRule: &oss.ServerSideEncryptionRule{
			// SSE-OSS：OSS 托管密钥的 AES256（对齐 S3 的 SSE-S3 语义）
			ApplyServerSideEncryptionByDefault: &oss.ApplyServerSideEncryptionByDefault{
				SSEAlgorithm: oss.Ptr("AES256"),
			},
		},
	})
	if err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) RemoveBucketEncryption(ctx context.Context, bucket string) error {
	if _, err := a.client.DeleteBucketEncryption(ctx, &oss.DeleteBucketEncryptionRequest{Bucket: oss.Ptr(bucket)}); err != nil {
		if ossAbsentConfig(err, "NoSuchServerSideEncryptionRule") {
			return nil
		}
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) GetBucketTagging(ctx context.Context, bucket string) (map[string]string, error) {
	res, err := a.client.GetBucketTags(ctx, &oss.GetBucketTagsRequest{Bucket: oss.Ptr(bucket)})
	if err != nil {
		if ossAbsentConfig(err, "NoSuchTagSet") {
			return nil, nil
		}
		return nil, wrapOSSError(err)
	}
	if res.Tagging == nil {
		return nil, nil
	}
	return ossTagsToMap(res.Tagging), nil
}

func (a *ossAdapter) SetBucketTagging(ctx context.Context, bucket string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		if _, err := a.client.DeleteBucketTags(ctx, &oss.DeleteBucketTagsRequest{Bucket: oss.Ptr(bucket)}); err != nil {
			if ossAbsentConfig(err, "NoSuchTagSet") {
				return nil
			}
			return wrapOSSError(err)
		}
		return nil
	}
	if err := validateOSSTags(tagMap); err != nil {
		return err
	}
	if _, err := a.client.PutBucketTags(ctx, &oss.PutBucketTagsRequest{
		Bucket:  oss.Ptr(bucket),
		Tagging: ossTaggingOf(tagMap),
	}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) GetBucketLifecycle(ctx context.Context, bucket string) ([]LifecycleRule, error) {
	res, err := a.client.GetBucketLifecycle(ctx, &oss.GetBucketLifecycleRequest{Bucket: oss.Ptr(bucket)})
	if err != nil {
		if ossAbsentConfig(err, "NoSuchLifecycleConfiguration") {
			return nil, nil
		}
		return nil, wrapOSSError(err)
	}
	if res.LifecycleConfiguration == nil {
		return nil, nil
	}
	rules := make([]LifecycleRule, 0, len(res.LifecycleConfiguration.Rules))
	for _, r := range res.LifecycleConfiguration.Rules {
		rule := LifecycleRule{
			ID:     ossString(r.ID),
			Status: ossString(r.Status),
			Prefix: ossString(r.Prefix),
		}
		if r.Expiration != nil && r.Expiration.Days != nil {
			rule.ExpirationDays = int(*r.Expiration.Days)
		}
		if r.AbortMultipartUpload != nil && r.AbortMultipartUpload.Days != nil {
			rule.AbortMultipartDays = int(*r.AbortMultipartUpload.Days)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (a *ossAdapter) SetBucketLifecycle(ctx context.Context, bucket string, rules []LifecycleRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除生命周期配置
		if _, err := a.client.DeleteBucketLifecycle(ctx, &oss.DeleteBucketLifecycleRequest{Bucket: oss.Ptr(bucket)}); err != nil {
			if ossAbsentConfig(err, "NoSuchLifecycleConfiguration") {
				return nil
			}
			return wrapOSSError(err)
		}
		return nil
	}
	ossRules := make([]oss.LifecycleRule, 0, len(rules))
	for _, r := range rules {
		rule := oss.LifecycleRule{
			ID:     ossPtrString(r.ID),
			Status: ossPtrString(r.Status),
			Prefix: ossPtrString(r.Prefix),
		}
		if r.ExpirationDays > 0 {
			days := int32(r.ExpirationDays)
			rule.Expiration = &oss.LifecycleRuleExpiration{Days: &days}
		}
		if r.AbortMultipartDays > 0 {
			days := int32(r.AbortMultipartDays)
			rule.AbortMultipartUpload = &oss.LifecycleRuleAbortMultipartUpload{Days: &days}
		}
		ossRules = append(ossRules, rule)
	}
	if _, err := a.client.PutBucketLifecycle(ctx, &oss.PutBucketLifecycleRequest{
		Bucket:                 oss.Ptr(bucket),
		LifecycleConfiguration: &oss.LifecycleConfiguration{Rules: ossRules},
	}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) GetBucketCors(ctx context.Context, bucket string) ([]CORSRule, error) {
	res, err := a.client.GetBucketCors(ctx, &oss.GetBucketCorsRequest{Bucket: oss.Ptr(bucket)})
	if err != nil {
		if ossAbsentConfig(err, "NoSuchCORSConfiguration") {
			return nil, nil
		}
		return nil, wrapOSSError(err)
	}
	if res.CORSConfiguration == nil {
		return nil, nil
	}
	out := make([]CORSRule, 0, len(res.CORSConfiguration.CORSRules))
	for _, r := range res.CORSConfiguration.CORSRules {
		rule := CORSRule{
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
		}
		if r.MaxAgeSeconds != nil {
			rule.MaxAgeSeconds = int(*r.MaxAgeSeconds)
		}
		out = append(out, rule)
	}
	return out, nil
}

func (a *ossAdapter) SetBucketCors(ctx context.Context, bucket string, rules []CORSRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除 CORS 配置
		if _, err := a.client.DeleteBucketCors(ctx, &oss.DeleteBucketCorsRequest{Bucket: oss.Ptr(bucket)}); err != nil {
			if ossAbsentConfig(err, "NoSuchCORSConfiguration") {
				return nil
			}
			return wrapOSSError(err)
		}
		return nil
	}
	ossRules := make([]oss.CORSRule, 0, len(rules))
	for _, r := range rules {
		rule := oss.CORSRule{
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
		}
		if r.MaxAgeSeconds > 0 {
			seconds := int64(r.MaxAgeSeconds)
			rule.MaxAgeSeconds = &seconds
		}
		// COS/S3 的规则 ID 在 OSS SDK 的 CORSRule 里没有承载字段，写入时丢弃
		ossRules = append(ossRules, rule)
	}
	if _, err := a.client.PutBucketCors(ctx, &oss.PutBucketCorsRequest{
		Bucket:            oss.Ptr(bucket),
		CORSConfiguration: &oss.CORSConfiguration{CORSRules: ossRules},
	}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

// —— G. 分片上传 ——

func (a *ossAdapter) InitMultipart(ctx context.Context, bucket, key, contentType, storageClass string) (string, error) {
	req := &oss.InitiateMultipartUploadRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(key),
	}
	if strings.TrimSpace(contentType) != "" {
		req.ContentType = oss.Ptr(contentType)
	}
	if strings.TrimSpace(storageClass) != "" {
		req.StorageClass = oss.StorageClassType(storageClass)
	}
	res, err := a.client.InitiateMultipartUpload(ctx, req)
	if err != nil {
		return "", wrapOSSError(err)
	}
	return ossString(res.UploadId), nil
}

func (a *ossAdapter) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*PartInfo, error) {
	req := &oss.UploadPartRequest{
		Bucket:     oss.Ptr(bucket),
		Key:        oss.Ptr(key),
		UploadId:   oss.Ptr(uploadID),
		PartNumber: int32(partNumber),
		Body:       reader,
	}
	if size > 0 {
		req.ContentLength = oss.Ptr(size)
	}
	res, err := a.client.UploadPart(ctx, req)
	if err != nil {
		return nil, wrapOSSError(err)
	}
	// OSS 的 UploadPart 响应只给 ETag；Size 为本地已知值，LastModified 缺失保持零值
	return &PartInfo{PartNumber: partNumber, ETag: trimOSSEtag(ossString(res.ETag)), Size: size}, nil
}

func (a *ossAdapter) ListParts(ctx context.Context, bucket, key, uploadID string, partNumberMarker, maxParts int) (*PartsPage, error) {
	if maxParts <= 0 {
		maxParts = ossListPageSize
	}
	req := &oss.ListPartsRequest{
		Bucket:           oss.Ptr(bucket),
		Key:              oss.Ptr(key),
		UploadId:         oss.Ptr(uploadID),
		MaxParts:         int32(maxParts),
		PartNumberMarker: int32(partNumberMarker),
	}
	res, err := a.client.ListParts(ctx, req)
	if err != nil {
		return nil, wrapOSSError(err)
	}
	page := &PartsPage{
		IsTruncated:          res.IsTruncated,
		NextPartNumberMarker: int(res.NextPartNumberMarker),
	}
	page.Parts = make([]PartInfo, 0, len(res.Parts))
	for _, p := range res.Parts {
		// OSS SDK 的 UploadPart 只承载 PartNumber/ETag：Size/LastModified 缺失保持零值
		page.Parts = append(page.Parts, PartInfo{
			PartNumber: int(p.PartNumber),
			ETag:       trimOSSEtag(ossString(p.ETag)),
		})
	}
	return page, nil
}

func (a *ossAdapter) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompletedObject, error) {
	ossParts := make([]oss.UploadPart, 0, len(parts))
	for _, p := range parts {
		ossParts = append(ossParts, oss.UploadPart{PartNumber: int32(p.PartNumber), ETag: oss.Ptr(p.ETag)})
	}
	res, err := a.client.CompleteMultipartUpload(ctx, &oss.CompleteMultipartUploadRequest{
		Bucket:                  oss.Ptr(bucket),
		Key:                     oss.Ptr(key),
		UploadId:                oss.Ptr(uploadID),
		CompleteMultipartUpload: &oss.CompleteMultipartUpload{Parts: ossParts},
	})
	if err != nil {
		return nil, wrapOSSError(err)
	}
	// OSS/S3 的 Complete 响应不含对象大小（与 minio 路径的既有语义一致，Size 恒 0）
	return &CompletedObject{Key: ossString(res.Key), ETag: trimOSSEtag(ossString(res.ETag))}, nil
}

func (a *ossAdapter) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	if _, err := a.client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{
		Bucket:   oss.Ptr(bucket),
		Key:      oss.Ptr(key),
		UploadId: oss.Ptr(uploadID),
	}); err != nil {
		return wrapOSSError(err)
	}
	return nil
}

func (a *ossAdapter) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (*UploadsPage, error) {
	if maxUploads <= 0 {
		maxUploads = ossListPageSize
	}
	req := &oss.ListMultipartUploadsRequest{
		Bucket:     oss.Ptr(bucket),
		MaxUploads: int32(maxUploads),
	}
	if prefix != "" {
		req.Prefix = oss.Ptr(prefix)
	}
	if keyMarker != "" {
		req.KeyMarker = oss.Ptr(keyMarker)
	}
	if uploadIDMarker != "" {
		req.UploadIdMarker = oss.Ptr(uploadIDMarker)
	}
	res, err := a.client.ListMultipartUploads(ctx, req)
	if err != nil {
		return nil, wrapOSSError(err)
	}
	page := &UploadsPage{
		IsTruncated:        res.IsTruncated,
		NextKeyMarker:      ossString(res.NextKeyMarker),
		NextUploadIDMarker: ossString(res.NextUploadIdMarker),
	}
	page.Uploads = make([]MultipartUploadInfo, 0, len(res.Uploads))
	for _, u := range res.Uploads {
		info := MultipartUploadInfo{Key: ossString(u.Key), UploadID: ossString(u.UploadId)}
		if u.Initiated != nil {
			info.Initiated = *u.Initiated
		}
		// OSS 的 ListMultipartUploads 不返回已上传大小（与 minio 路径一致，保持 0）
		page.Uploads = append(page.Uploads, info)
	}
	return page, nil
}

// —— 工具函数 ——

// ossString 解引用可空字符串。
func ossString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// ossPtrString 空串转 nil（对可空请求字段保持「不发送」语义）。
func ossPtrString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return oss.Ptr(v)
}

// trimOSSEtag 去掉 ETag 的引号（与 minio 路径的规整行为一致）。
func trimOSSEtag(etag string) string {
	return strings.Trim(strings.TrimSpace(etag), `"`)
}

// ossTaggingOf 构造 OSS 的标签容器。
func ossTaggingOf(tagMap map[string]string) *oss.Tagging {
	tags := make([]oss.Tag, 0, len(tagMap))
	for k, v := range tagMap {
		tags = append(tags, oss.Tag{Key: oss.Ptr(k), Value: oss.Ptr(v)})
	}
	return &oss.Tagging{TagSet: &oss.TagSet{Tags: tags}}
}

// ossTagsToMap 摊平 OSS 的标签容器。
func ossTagsToMap(tagging *oss.Tagging) map[string]string {
	out := make(map[string]string)
	if tagging == nil || tagging.TagSet == nil {
		return out
	}
	return ossTagListToMap(tagging.TagSet.Tags)
}

// ossTagListToMap 把标签列表摊平为 map（对象标签接口直接返回列表）。
func ossTagListToMap(tags []oss.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		out[ossString(t.Key)] = ossString(t.Value)
	}
	return out
}
