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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// cos 是腾讯云 COS 的原生适配器（cos-go-sdk-v5）。
//
// 语义要点（与 generic 实现的行为对齐，见 storage 包升级说明）：
//   - COS 的列出接口只有 V1（marker）语义：不分页接口一律以 marker 实现，
//     ListObjectsV2Page 的 continuationToken 与 NextToken 均为 marker 透传（对调用方不透明，契约不变）。
//   - COS 的 policy 读写是结构化 JSON（BucketPutPolicyOptions），Get 侧解析后重新序列化返回原文。
//   - Bucket 级 Client 以 bucket 子域构造（<bucket>.<endpoint-host>），按 bucket 缓存。
//   - 「未设置」态由 COS 以 404 返回（错误码历史空态表 + 404 且非 NoSuchBucket 双规则归一化）。
type cosAdapter struct {
	schema string // http / https
	host   string // 服务级端点 host[:port]（cos.<region>.myqcloud.com）
	http   *http.Client

	mu      sync.Mutex
	buckets map[string]*cos.Client
}

func newCos(cfg Config) (Adapter, error) {
	schema := "https"
	if !cfg.Secure {
		schema = "http"
	}
	auth := &cos.AuthorizationTransport{
		SecretID:     cfg.AccessKeyID,
		SecretKey:    cfg.SecretAccessKey,
		SessionToken: cfg.SessionToken,
	}
	if cfg.Transport != nil {
		auth.Transport = cfg.Transport
	}
	return &cosAdapter{
		schema:  schema,
		host:    cfg.Endpoint,
		http:    &http.Client{Transport: auth},
		buckets: make(map[string]*cos.Client),
	}, nil
}

// clientForBucket 返回（并缓存）某个 bucket 作用域的客户端。
// COS SDK 的 BaseURL 是 bucket 级：bucket 子域 + 服务端点域名。
func (a *cosAdapter) clientForBucket(bucket string) (*cos.Client, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, &StorageError{Code: "InvalidBucketName", Status: http.StatusBadRequest, Message: "bucket is required"}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.buckets[bucket]; ok {
		return c, nil
	}
	c := cos.NewClient(&cos.BaseURL{
		BucketURL: &url.URL{Scheme: a.schema, Host: bucket + "." + a.host},
	}, a.http)
	a.buckets[bucket] = c
	return c, nil
}

// serviceClient 返回服务级客户端（仅 ListBuckets 使用）。
func (a *cosAdapter) serviceClient() *cos.Client {
	return cos.NewClient(&cos.BaseURL{
		ServiceURL: &url.URL{Scheme: a.schema, Host: a.host},
	}, a.http)
}

// wrapCosError 把 SDK 错误包装为 *StorageError，就地提取错误码与 HTTP 状态码。
// 注意：cos.ErrorResponse.Error() 在 Response 为 nil 时会空指针，此时退化为显式 Message。
func wrapCosError(err error) error {
	if err == nil {
		return nil
	}
	var er *cos.ErrorResponse
	if errors.As(err, &er) {
		if er.Response != nil {
			return &StorageError{Code: er.Code, Status: er.Response.StatusCode, Err: err}
		}
		return &StorageError{Code: er.Code, Message: fmt.Sprintf("cos: %s: %s", er.Code, er.Message)}
	}
	return err
}

// cosAbsentConfig 判断错误是否属于「配置未设置」：历史空态码，或 HTTP 404 且非 NoSuchBucket。
func cosAbsentConfig(err error, codes ...string) bool {
	return absentCodes(wrapCosError(err), codes...)
}

// —— A. Bucket 基础 ——

func (a *cosAdapter) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	res, _, err := a.serviceClient().Service.Get(ctx)
	if err != nil {
		return nil, wrapCosError(err)
	}
	out := make([]BucketInfo, 0, len(res.Buckets))
	for _, b := range res.Buckets {
		out = append(out, BucketInfo{Name: b.Name, CreationDate: parseCosTime(b.CreationDate)})
	}
	return out, nil
}

func (a *cosAdapter) CreateBucket(ctx context.Context, bucket, region string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	// COS 的 bucket 区域由 bucket 域名承载，创建请求无需区域配置
	if _, err := c.Bucket.Put(ctx, nil); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) DeleteBucket(ctx context.Context, bucket string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if _, err := c.Bucket.Delete(ctx); err != nil {
		return wrapCosError(err)
	}
	return nil
}

// —— B. 流式列举 ——

// cosListPageSize 单页列举条数（COS 上限 1000）
const cosListPageSize = 1000

func (a *cosAdapter) ListObjectsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		c, err := a.clientForBucket(bucket)
		if err != nil {
			sendCosEntry(ctx, out, ObjectEntry{Err: err})
			return
		}
		delimiter := ""
		if !opts.Recursive {
			delimiter = "/"
		}
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = cosListPageSize
		}
		marker := ""
		for {
			res, _, err := c.Bucket.Get(ctx, &cos.BucketGetOptions{
				Prefix:    opts.Prefix,
				Delimiter: delimiter,
				Marker:    marker,
				MaxKeys:   maxKeys,
			})
			if err != nil {
				sendCosEntry(ctx, out, ObjectEntry{Err: wrapCosError(err)})
				return
			}
			// 与 minio 流一致：先对象、后目录前缀（CommonPrefixes 仅 delimiter 模式出现）
			for _, o := range res.Contents {
				if !sendCosEntry(ctx, out, ObjectEntry{ObjectInfo: cosObjectInfo(o)}) {
					return
				}
			}
			for _, cp := range res.CommonPrefixes {
				if !sendCosEntry(ctx, out, ObjectEntry{ObjectInfo: ObjectInfo{Key: cp}}) {
					return
				}
			}
			if !res.IsTruncated {
				return
			}
			// S3 的 NextMarker 仅在 delimiter 模式下由服务端返回；平铺模式用本页最后一个 key 续读
			next := res.NextMarker
			if next == "" && len(res.Contents) > 0 {
				next = res.Contents[len(res.Contents)-1].Key
			}
			if next == "" || next == marker {
				return
			}
			marker = next
		}
	}()
	return out
}

func (a *cosAdapter) ListVersionsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		c, err := a.clientForBucket(bucket)
		if err != nil {
			sendCosEntry(ctx, out, ObjectEntry{Err: err})
			return
		}
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = cosListPageSize
		}
		keyMarker, versionIDMarker := "", ""
		for {
			res, _, err := c.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{
				Prefix:          opts.Prefix,
				KeyMarker:       keyMarker,
				VersionIdMarker: versionIDMarker,
				MaxKeys:         maxKeys,
			})
			if err != nil {
				sendCosEntry(ctx, out, ObjectEntry{Err: wrapCosError(err)})
				return
			}
			for _, info := range mergeCosVersions(res.Version, res.DeleteMarker) {
				if !sendCosEntry(ctx, out, ObjectEntry{ObjectInfo: info}) {
					return
				}
			}
			if !res.IsTruncated {
				return
			}
			if res.NextKeyMarker == keyMarker && res.NextVersionIdMarker == versionIDMarker {
				return
			}
			if res.NextKeyMarker == "" && res.NextVersionIdMarker == "" {
				return
			}
			keyMarker, versionIDMarker = res.NextKeyMarker, res.NextVersionIdMarker
		}
	}()
	return out
}

// sendCosEntry 向流通道发送元素；ctx 取消则放弃并返回 false。
func sendCosEntry(ctx context.Context, out chan<- ObjectEntry, entry ObjectEntry) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

// mergeCosVersions 把 COS 的「版本 / 删除标记」两个数组合并为单流。
// COS 的 XML 解码丢失了服务端的交错顺序，这里按 S3 语义近似：key 升序，同 key 内按修改时间倒序。
func mergeCosVersions(versions []cos.ListVersionsResultVersion, deleteMarkers []cos.ListVersionsResultDeleteMarker) []ObjectInfo {
	items := make([]ObjectInfo, 0, len(versions)+len(deleteMarkers))
	for _, v := range versions {
		items = append(items, ObjectInfo{
			Key:          v.Key,
			VersionID:    v.VersionId,
			IsLatest:     v.IsLatest,
			Size:         v.Size,
			LastModified: parseCosTime(v.LastModified),
			ETag:         trimCosETag(v.ETag),
			StorageClass: v.StorageClass,
		})
	}
	for _, d := range deleteMarkers {
		items = append(items, ObjectInfo{
			Key:            d.Key,
			VersionID:      d.VersionId,
			IsLatest:       d.IsLatest,
			IsDeleteMarker: true,
			LastModified:   parseCosTime(d.LastModified),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Key != items[j].Key {
			return items[i].Key < items[j].Key
		}
		return items[i].LastModified.After(items[j].LastModified)
	})
	return items
}

// —— C. 分页列举（COS 仅 V1 marker 语义，token 即 marker 透传）——

func (a *cosAdapter) ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken, delimiter string, pageSize int) (*ObjectPage, error) {
	return a.listBucketPage(ctx, bucket, prefix, continuationToken, delimiter, pageSize)
}

func (a *cosAdapter) ListObjectsByMarker(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	return a.listBucketPage(ctx, bucket, prefix, marker, delimiter, pageSize)
}

func (a *cosAdapter) listBucketPage(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	if pageSize <= 0 {
		pageSize = cosListPageSize
	}
	res, _, err := c.Bucket.Get(ctx, &cos.BucketGetOptions{
		Prefix:    prefix,
		Delimiter: delimiter,
		Marker:    marker,
		MaxKeys:   pageSize,
	})
	if err != nil {
		return nil, wrapCosError(err)
	}
	page := &ObjectPage{
		Contents:    make([]ObjectInfo, 0, len(res.Contents)),
		IsTruncated: res.IsTruncated,
		NextToken:   res.NextMarker,
	}
	for _, o := range res.Contents {
		page.Contents = append(page.Contents, cosObjectInfo(o))
	}
	page.CommonPrefixes = append(page.CommonPrefixes, res.CommonPrefixes...)
	if page.IsTruncated && page.NextToken == "" && len(res.Contents) > 0 {
		// 平铺模式服务端不返回 NextMarker：用本页最后 key 作为续读游标
		page.NextToken = res.Contents[len(res.Contents)-1].Key
	}
	return page, nil
}

// cosObjectInfo 把 COS 的对象条目映射为适配层摘要。
func cosObjectInfo(o cos.Object) ObjectInfo {
	return ObjectInfo{
		Key:          o.Key,
		Size:         o.Size,
		LastModified: parseCosTime(o.LastModified),
		ETag:         trimCosETag(o.ETag),
		VersionID:    o.VersionId,
		StorageClass: o.StorageClass,
	}
}

// —— D. 对象读写 ——

func (a *cosAdapter) StatObject(ctx context.Context, bucket, key, versionID string) (*ObjectStat, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	var resp *cos.Response
	if versionID != "" {
		resp, err = c.Object.Head(ctx, key, &cos.ObjectHeadOptions{}, versionID)
	} else {
		resp, err = c.Object.Head(ctx, key, &cos.ObjectHeadOptions{})
	}
	if err != nil {
		return nil, wrapCosError(err)
	}
	h := resp.Header
	size, _ := strconv.ParseInt(h.Get("Content-Length"), 10, 64)
	storedVersion := h.Get("X-Cos-Version-Id")
	if versionID != "" {
		storedVersion = versionID
	}
	return &ObjectStat{
		Key:          key,
		VersionID:    storedVersion,
		Size:         size,
		ETag:         trimCosETag(h.Get("ETag")),
		ContentType:  h.Get("Content-Type"),
		LastModified: parseCosHTTPTime(h.Get("Last-Modified")),
		Expires:      parseCosHTTPTime(h.Get("Expires")),
		StorageClass: h.Get("X-Cos-Storage-Class"),
		Metadata:     h,
		UserMetadata: cosUserMetadata(h),
	}, nil
}

func (a *cosAdapter) GetObject(ctx context.Context, bucket, key string) (*ObjectReader, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	// disableCloseBody 语义：resp.Body 归调用方所有（与 minio 惰性句柄一致，首个 Read 才拉流）
	resp, err := c.Object.Get(ctx, key, &cos.ObjectGetOptions{})
	if err != nil {
		return nil, wrapCosError(err)
	}
	return &ObjectReader{Body: resp.Body}, nil
}

func (a *cosAdapter) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: contentType},
	}
	// 长度未知（size<=0）时不下发 Content-Length，由 SDK 走 chunked 传输
	if size > 0 {
		opt.ContentLength = size
	}
	if _, err := c.Object.Put(ctx, key, reader, opt); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) CopyObject(ctx context.Context, src CopySrc, dest CopyDest) error {
	c, err := a.clientForBucket(dest.Bucket)
	if err != nil {
		return err
	}
	// x-cos-copy-source 使用完整 bucket 域名形式：<bucket>.<endpoint-host>/<key>
	sourceURL := src.Bucket + "." + a.host + "/" + src.Object

	var opt *cos.ObjectCopyOptions
	if dest.ReplaceMetadata {
		opt = &cos.ObjectCopyOptions{ObjectCopyHeaderOptions: cosCopyHeader(dest)}
	}

	var copyErr error
	if src.VersionID != "" {
		_, _, copyErr = c.Object.Copy(ctx, dest.Object, sourceURL, opt, src.VersionID)
	} else {
		_, _, copyErr = c.Object.Copy(ctx, dest.Object, sourceURL, opt)
	}
	if copyErr != nil {
		return wrapCosError(copyErr)
	}
	return nil
}

// cosCopyHeader 构造元数据整体替换（Replaced）的复制头。
// 控制层传入的 UserMetadata 里混有系统项（X-Amz-Storage-Class / SSE），按 COS 对应头部拆分承载。
func cosCopyHeader(dest CopyDest) *cos.ObjectCopyHeaderOptions {
	header := &cos.ObjectCopyHeaderOptions{
		ContentType:        dest.ContentType,
		ContentEncoding:    dest.ContentEncoding,
		ContentDisposition: dest.ContentDisposition,
		ContentLanguage:    dest.ContentLanguage,
		CacheControl:       dest.CacheControl,
		// COS 的元数据替换指令枚举是 Copy / Replaced（与 S3 的 COPY/REPLACE 措辞不同）
		XCosMetadataDirective: "Replaced",
	}
	if !dest.Expires.IsZero() {
		header.Expires = dest.Expires.UTC().Format(http.TimeFormat)
	}
	meta := http.Header{}
	extra := http.Header{}
	for k, v := range dest.UserMetadata {
		switch {
		case strings.EqualFold(k, "X-Amz-Storage-Class"):
			header.XCosStorageClass = v
		case strings.EqualFold(k, "X-Amz-Server-Side-Encryption"):
			header.XCosServerSideEncryption = v
		case strings.EqualFold(k, "X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id"):
			// COS 的 KMS 主密钥走独立头（x-cos-server-side-encryption-cos-kms-key-id）
			extra.Set("X-Cos-Server-Side-Encryption-Cos-Kms-Key-Id", v)
		default:
			meta.Set("X-Cos-Meta-"+k, v)
		}
	}
	if len(meta) > 0 {
		header.XCosMetaXXX = &meta
	}
	if len(extra) > 0 {
		header.XOptionHeader = &extra
	}
	return header
}

func (a *cosAdapter) RemoveObject(ctx context.Context, bucket, key, versionID string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	// VersionId 为空时被 omitempty 省略，等价于删除当前版本
	if _, err := c.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: versionID}); err != nil {
		return wrapCosError(err)
	}
	return nil
}

// cosDeleteBatchSize 单次批量删除的对象数上限（COS/S3 一致为 1000）
const cosDeleteBatchSize = 1000

func (a *cosAdapter) DeleteObjectsBatch(ctx context.Context, bucket string, refs <-chan DeleteRef) DeleteErrorStream {
	out := make(chan DeleteError, 1)
	go func() {
		defer close(out)
		c, err := a.clientForBucket(bucket)
		if err != nil {
			sendCosDeleteError(ctx, out, DeleteError{Err: err})
			return
		}
		batch := make([]cos.Object, 0, cosDeleteBatchSize)
		flush := func() bool {
			if len(batch) == 0 {
				return true
			}
			firstKey := batch[0].Key
			res, _, err := c.Object.DeleteMulti(ctx, &cos.ObjectDeleteMultiOptions{
				// Quiet 模式只回传失败对象，成功对象不产生条目（与适配层语义一致）
				Quiet:   true,
				Objects: batch,
			})
			batch = batch[:0]
			if err != nil {
				// 批次级失败：以批次首个 key 为定位信息，继续处理后续批次
				return sendCosDeleteError(ctx, out, DeleteError{Key: firstKey, Err: wrapCosError(err)})
			}
			for _, e := range res.Errors {
				entry := DeleteError{
					Key: e.Key,
					Err: &StorageError{Code: e.Code, Message: cosDeleteErrorMessage(e.Code, e.Message)},
				}
				if !sendCosDeleteError(ctx, out, entry) {
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
				batch = append(batch, cos.Object{Key: ref.Key, VersionId: ref.VersionID})
				if len(batch) >= cosDeleteBatchSize {
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

func sendCosDeleteError(ctx context.Context, out chan<- DeleteError, entry DeleteError) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

func cosDeleteErrorMessage(code, message string) string {
	if code == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", code, message)
}

func (a *cosAdapter) PresignGetObject(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return "", err
	}
	// GetPresignedURL3 从 AuthorizationTransport 取凭据签名
	u, err := c.Object.GetPresignedURL3(ctx, http.MethodGet, key, expires, nil)
	if err != nil {
		return "", wrapCosError(err)
	}
	return u.String(), nil
}

// —— E. 对象标签与 ACL ——

func (a *cosAdapter) GetObjectTags(ctx context.Context, bucket, key, versionID string) (map[string]string, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	var res *cos.ObjectGetTaggingResult
	if versionID != "" {
		res, _, err = c.Object.GetTagging(ctx, key, versionID)
	} else {
		res, _, err = c.Object.GetTagging(ctx, key)
	}
	if err != nil {
		if cosAbsentConfig(err, "NoSuchTagSet") {
			return map[string]string{}, nil
		}
		return nil, wrapCosError(err)
	}
	return cosObjectTagsToMap(res.TagSet), nil
}

func (a *cosAdapter) SetObjectTags(ctx context.Context, bucket, key, versionID string, tagMap map[string]string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if len(tagMap) == 0 {
		if versionID != "" {
			_, err = c.Object.DeleteTagging(ctx, key, versionID)
		} else {
			_, err = c.Object.DeleteTagging(ctx, key)
		}
		if err != nil {
			return wrapCosError(err)
		}
		return nil
	}
	if err := validateCosTags(tagMap); err != nil {
		return err
	}
	tagSet := make([]cos.ObjectTaggingTag, 0, len(tagMap))
	for tagKey, tagValue := range tagMap {
		tagSet = append(tagSet, cos.ObjectTaggingTag{Key: tagKey, Value: tagValue})
	}
	opt := &cos.ObjectPutTaggingOptions{TagSet: tagSet}
	if versionID != "" {
		_, err = c.Object.PutTagging(ctx, key, opt, versionID)
	} else {
		_, err = c.Object.PutTagging(ctx, key, opt)
	}
	if err != nil {
		return wrapCosError(err)
	}
	return nil
}

// cosMaxObjectTags 对象标签数量上限（S3/COS 一致）
const cosMaxObjectTags = 10

// validateCosTags 本地校验对象标签，命中用户输入问题时返回 InvalidTag（控制层映射为 400）。
func validateCosTags(tagMap map[string]string) error {
	if len(tagMap) > cosMaxObjectTags {
		return &StorageError{
			Code: CodeInvalidTag, Status: http.StatusBadRequest,
			Message: fmt.Sprintf("too many object tags: %d, limit is %d", len(tagMap), cosMaxObjectTags),
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

func (a *cosAdapter) GetObjectACL(ctx context.Context, bucket, key string) (*ObjectStat, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	res, _, err := c.Object.GetACL(ctx, key)
	if err != nil {
		return nil, wrapCosError(err)
	}
	stat := &ObjectStat{Key: key, Metadata: http.Header{}}
	// COS 的 GET acl 返回授权列表（无 canned 头）：按「对所有人授权」的粒度反推私有/公共读写
	stat.Metadata.Set("X-Amz-Acl", classifyCosACL(res.AccessControlList))
	return stat, nil
}

// classifyCosACL 从授权列表反推三档 ACL（与 Bucket 配置同族粒度）。
func classifyCosACL(grants []cos.ACLGrant) string {
	read, write := false, false
	for _, g := range grants {
		if g.Grantee == nil {
			continue
		}
		allUsers := g.Grantee.URI == cos.CAMAllUsers || g.Grantee.ID == "qcs::cam::anyone:anyone"
		if !allUsers {
			continue
		}
		switch strings.ToUpper(g.Permission) {
		case "READ":
			read = true
		case "WRITE", "FULL_CONTROL":
			write = true
		}
	}
	switch {
	case read && write:
		return "public-read-write"
	case read:
		return "public-read"
	default:
		return "private"
	}
}

// —— F. Bucket 配置 ——

func (a *cosAdapter) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return "", err
	}
	res, _, err := c.Bucket.GetPolicy(ctx)
	if err != nil {
		if cosAbsentConfig(err, "NoSuchBucketPolicy") {
			return "", nil
		}
		return "", wrapCosError(err)
	}
	return cosPolicyToText(res)
}

// cosPolicyToText 把 SDK 结构化策略重新序列化为 JSON 原文返回。
// 结构覆盖 COS policy 全集（version / 顶层 principal / statement{sid, principal, action, effect, resource, condition}）；
// SDK 解析不了的未知扩展字段会在此环节丢失，属已知限制。
func cosPolicyToText(res *cos.BucketGetPolicyResult) (string, error) {
	if res == nil {
		return "", nil
	}
	data, err := json.Marshal(res)
	if err != nil {
		return "", wrapCosError(err)
	}
	// 空结构（无任何字段）等价于「无策略」
	if strings.TrimSpace(string(data)) == "{}" {
		return "", nil
	}
	return string(data), nil
}

func (a *cosAdapter) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if strings.TrimSpace(policy) == "" {
		if _, err := c.Bucket.DeletePolicy(ctx); err != nil {
			if cosAbsentConfig(err, "NoSuchBucketPolicy") {
				return nil
			}
			return wrapCosError(err)
		}
		return nil
	}
	// 契约是「策略原文透传」：这里把 JSON 反序列化进 SDK 的结构化模型后提交
	var opt cos.BucketPutPolicyOptions
	if err := json.Unmarshal([]byte(policy), &opt); err != nil {
		// 结构不匹配 COS 的策略模型（控制层的结构校验已先行把关，这里属兜底）
		return &StorageError{Code: "MalformedPolicy", Status: http.StatusBadRequest, Err: err}
	}
	if _, err := c.Bucket.PutPolicy(ctx, &opt); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) GetBucketVersioning(ctx context.Context, bucket string) (string, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return "", err
	}
	res, _, err := c.Bucket.GetVersioning(ctx)
	if err != nil {
		// 版本控制错误不归一化：维持「读取失败即报错」的既有语义
		return "", wrapCosError(err)
	}
	return res.Status, nil
}

func (a *cosAdapter) SetBucketVersioning(ctx context.Context, bucket, status string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if _, err := c.Bucket.PutVersioning(ctx, &cos.BucketPutVersionOptions{Status: status}); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) GetBucketEncryption(ctx context.Context, bucket string) (bool, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return false, err
	}
	res, _, err := c.Bucket.GetEncryption(ctx)
	if err != nil {
		// COS「未配置加密」的空态码未见公开文档：先按 404 规则归一化，冒烟时补录码表
		if cosAbsentConfig(err) {
			return false, nil
		}
		return false, wrapCosError(err)
	}
	return res != nil && res.Rule != nil && strings.TrimSpace(res.Rule.SSEAlgorithm) != "", nil
}

func (a *cosAdapter) SetBucketEncryption(ctx context.Context, bucket string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	// SSE-COS：COS 托管密钥的 AES256（对齐 S3 的 SSE-S3 语义）
	opt := &cos.BucketPutEncryptionOptions{
		Rule: &cos.BucketEncryptionConfiguration{SSEAlgorithm: "AES256"},
	}
	if _, err := c.Bucket.PutEncryption(ctx, opt); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) RemoveBucketEncryption(ctx context.Context, bucket string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if _, err := c.Bucket.DeleteEncryption(ctx); err != nil {
		if cosAbsentConfig(err) {
			return nil
		}
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) GetBucketTagging(ctx context.Context, bucket string) (map[string]string, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	res, _, err := c.Bucket.GetTagging(ctx)
	if err != nil {
		if cosAbsentConfig(err, "NoSuchTagSet") {
			return nil, nil
		}
		return nil, wrapCosError(err)
	}
	return cosBucketTagsToMap(res.TagSet), nil
}

func (a *cosAdapter) SetBucketTagging(ctx context.Context, bucket string, tagMap map[string]string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if len(tagMap) == 0 {
		if _, err := c.Bucket.DeleteTagging(ctx); err != nil {
			if cosAbsentConfig(err, "NoSuchTagSet") {
				return nil
			}
			return wrapCosError(err)
		}
		return nil
	}
	if err := validateCosTags(tagMap); err != nil {
		return err
	}
	tagSet := make([]cos.BucketTaggingTag, 0, len(tagMap))
	for tagKey, tagValue := range tagMap {
		tagSet = append(tagSet, cos.BucketTaggingTag{Key: tagKey, Value: tagValue})
	}
	if _, err := c.Bucket.PutTagging(ctx, &cos.BucketPutTaggingOptions{TagSet: tagSet}); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) GetBucketLifecycle(ctx context.Context, bucket string) ([]LifecycleRule, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	res, _, err := c.Bucket.GetLifecycle(ctx)
	if err != nil {
		if cosAbsentConfig(err, "NoSuchLifecycleConfiguration") {
			return nil, nil
		}
		return nil, wrapCosError(err)
	}
	rules := make([]LifecycleRule, 0, len(res.Rules))
	for _, r := range res.Rules {
		rule := LifecycleRule{ID: r.ID, Status: r.Status}
		if r.Filter != nil {
			rule.Prefix = r.Filter.Prefix
		}
		if r.Expiration != nil {
			rule.ExpirationDays = r.Expiration.Days
		}
		if r.AbortIncompleteMultipartUpload != nil {
			rule.AbortMultipartDays = r.AbortIncompleteMultipartUpload.DaysAfterInitiation
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (a *cosAdapter) SetBucketLifecycle(ctx context.Context, bucket string, rules []LifecycleRule) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		// 空规则集合 = 删除生命周期配置
		if _, err := c.Bucket.DeleteLifecycle(ctx); err != nil {
			if cosAbsentConfig(err, "NoSuchLifecycleConfiguration") {
				return nil
			}
			return wrapCosError(err)
		}
		return nil
	}
	cosRules := make([]cos.BucketLifecycleRule, 0, len(rules))
	for _, r := range rules {
		rule := cos.BucketLifecycleRule{ID: r.ID, Status: r.Status}
		if r.Prefix != "" {
			rule.Filter = &cos.BucketLifecycleFilter{Prefix: r.Prefix}
		}
		if r.ExpirationDays > 0 {
			rule.Expiration = &cos.BucketLifecycleExpiration{Days: r.ExpirationDays}
		}
		if r.AbortMultipartDays > 0 {
			rule.AbortIncompleteMultipartUpload = &cos.BucketLifecycleAbortIncompleteMultipartUpload{DaysAfterInitiation: r.AbortMultipartDays}
		}
		cosRules = append(cosRules, rule)
	}
	if _, err := c.Bucket.PutLifecycle(ctx, &cos.BucketPutLifecycleOptions{Rules: cosRules}); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) GetBucketCors(ctx context.Context, bucket string) ([]CORSRule, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	res, _, err := c.Bucket.GetCORS(ctx)
	if err != nil {
		if cosAbsentConfig(err, "NoSuchCORSConfiguration") {
			return nil, nil
		}
		return nil, wrapCosError(err)
	}
	out := make([]CORSRule, 0, len(res.Rules))
	for _, r := range res.Rules {
		out = append(out, CORSRule{
			ID:             r.ID,
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  r.MaxAgeSeconds,
		})
	}
	return out, nil
}

func (a *cosAdapter) SetBucketCors(ctx context.Context, bucket string, rules []CORSRule) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		// 空规则集合 = 删除 CORS 配置
		if _, err := c.Bucket.DeleteCORS(ctx); err != nil {
			if cosAbsentConfig(err, "NoSuchCORSConfiguration") {
				return nil
			}
			return wrapCosError(err)
		}
		return nil
	}
	cosRules := make([]cos.BucketCORSRule, 0, len(rules))
	for _, r := range rules {
		cosRules = append(cosRules, cos.BucketCORSRule{
			ID:             r.ID,
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  r.MaxAgeSeconds,
		})
	}
	if _, err := c.Bucket.PutCORS(ctx, &cos.BucketPutCORSOptions{Rules: cosRules}); err != nil {
		return wrapCosError(err)
	}
	return nil
}

// —— G. 分片上传 ——

func (a *cosAdapter) InitMultipart(ctx context.Context, bucket, key, contentType, storageClass string) (string, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return "", err
	}
	opt := &cos.InitiateMultipartUploadOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:      contentType,
			XCosStorageClass: storageClass,
		},
	}
	res, _, err := c.Object.InitiateMultipartUpload(ctx, key, opt)
	if err != nil {
		return "", wrapCosError(err)
	}
	return res.UploadID, nil
}

func (a *cosAdapter) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*PartInfo, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	opt := &cos.ObjectUploadPartOptions{ContentLength: size}
	resp, err := c.Object.UploadPart(ctx, key, uploadID, partNumber, reader, opt)
	if err != nil {
		return nil, wrapCosError(err)
	}
	// ETag 在响应头（CompleteMultipartUpload 必须提交真实 ETag）；LastModified 响应不提供，保持零值
	return &PartInfo{
		PartNumber: partNumber,
		ETag:       trimCosETag(resp.Header.Get("ETag")),
		Size:       size,
	}, nil
}

func (a *cosAdapter) ListParts(ctx context.Context, bucket, key, uploadID string, partNumberMarker, maxParts int) (*PartsPage, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	if maxParts <= 0 {
		maxParts = cosListPageSize
	}
	res, _, err := c.Object.ListParts(ctx, key, uploadID, &cos.ObjectListPartsOptions{
		MaxParts:         strconv.Itoa(maxParts),
		PartNumberMarker: strconv.Itoa(partNumberMarker),
	})
	if err != nil {
		return nil, wrapCosError(err)
	}
	page := &PartsPage{IsTruncated: res.IsTruncated}
	page.Parts = make([]PartInfo, 0, len(res.Parts))
	for _, p := range res.Parts {
		page.Parts = append(page.Parts, PartInfo{
			PartNumber:   p.PartNumber,
			ETag:         trimCosETag(p.ETag),
			Size:         p.Size,
			LastModified: parseCosTime(p.LastModified),
		})
	}
	page.NextPartNumberMarker, _ = strconv.Atoi(res.NextPartNumberMarker)
	return page, nil
}

func (a *cosAdapter) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompletedObject, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	cosParts := make([]cos.Object, 0, len(parts))
	for _, p := range parts {
		// ETag 需带引号提交（S3 语义）；控制层传入的是去引号值
		cosParts = append(cosParts, cos.Object{PartNumber: p.PartNumber, ETag: p.ETag})
	}
	res, _, err := c.Object.CompleteMultipartUpload(ctx, key, uploadID, &cos.CompleteMultipartUploadOptions{Parts: cosParts})
	if err != nil {
		return nil, wrapCosError(err)
	}
	// COS/S3 的 Complete 响应不含对象大小（与 minio 路径的既有语义一致，Size 恒 0）
	return &CompletedObject{Key: res.Key, ETag: trimCosETag(res.ETag)}, nil
}

func (a *cosAdapter) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return err
	}
	if _, err := c.Object.AbortMultipartUpload(ctx, key, uploadID); err != nil {
		return wrapCosError(err)
	}
	return nil
}

func (a *cosAdapter) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (*UploadsPage, error) {
	c, err := a.clientForBucket(bucket)
	if err != nil {
		return nil, err
	}
	if maxUploads <= 0 {
		maxUploads = cosListPageSize
	}
	res, _, err := c.Bucket.ListMultipartUploads(ctx, &cos.ListMultipartUploadsOptions{
		Prefix:         prefix,
		KeyMarker:      keyMarker,
		UploadIDMarker: uploadIDMarker,
		MaxUploads:     maxUploads,
	})
	if err != nil {
		return nil, wrapCosError(err)
	}
	page := &UploadsPage{
		IsTruncated:        res.IsTruncated,
		NextKeyMarker:      res.NextKeyMarker,
		NextUploadIDMarker: res.NextUploadIDMarker,
	}
	page.Uploads = make([]MultipartUploadInfo, 0, len(res.Uploads))
	for _, u := range res.Uploads {
		page.Uploads = append(page.Uploads, MultipartUploadInfo{
			Key:       u.Key,
			UploadID:  u.UploadID,
			Initiated: parseCosTime(u.Initiated),
			// COS 的 ListMultipartUploads 不返回已上传大小（与 minio 路径一致，保持 0）
		})
	}
	return page, nil
}

// —— 工具函数 ——

// parseCosTime 解析 COS 的 RFC3339 时间串；解析失败返回零值。
func parseCosTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

// parseCosHTTPTime 解析 HTTP 头时间（RFC1123 GMT）；解析失败返回零值。
func parseCosHTTPTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if t, err := http.ParseTime(value); err == nil {
		return t
	}
	return time.Time{}
}

// trimCosETag 去掉 ETag 的引号（与 minio 路径的规整行为一致）。
func trimCosETag(etag string) string {
	return strings.Trim(strings.TrimSpace(etag), `"`)
}

// cosMetaHeaderPrefix 用户元数据头前缀（x-cos-meta-*）
const cosMetaHeaderPrefix = "X-Cos-Meta-"

// cosUserMetadata 提取用户元数据：键为前缀后的部分（与 minio 的去前缀行为一致）。
func cosUserMetadata(h http.Header) map[string]string {
	out := make(map[string]string)
	for k, v := range h {
		if len(v) == 0 || !strings.HasPrefix(k, cosMetaHeaderPrefix) {
			continue
		}
		out[strings.TrimPrefix(k, cosMetaHeaderPrefix)] = v[0]
	}
	return out
}

// cosObjectTagsToMap 把对象标签集合摊平为 map。
func cosObjectTagsToMap(tags []cos.ObjectTaggingTag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		out[t.Key] = t.Value
	}
	return out
}

// cosBucketTagsToMap 把桶标签集合摊平为 map。
func cosBucketTagsToMap(tags []cos.BucketTaggingTag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		out[t.Key] = t.Value
	}
	return out
}
