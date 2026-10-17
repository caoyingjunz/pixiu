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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

// obsAdapter 是华为云 OBS 的原生适配器（huaweicloud-sdk-go-obs）。
//
// 语义要点：
//   - OBS 的列举接口只有 V1（marker）语义：分页原语一律以 marker 实现，
//     ListObjectsV2Page 的 continuationToken 与 NextToken 均为 marker 透传（对调用方不透明）。
//   - 版本列举的 XML 解码把 Version 与 DeleteMarker 拆成两个数组（文档序丢失）：
//     与 COS 同口径，按「key 升序、同 key 内修改时间倒序」合并近似还原。
//   - 复制替换元数据的指令是 OBS 自己的 REPLACE_NEW（不是 S3 的 REPLACE）。
//   - 「未设置」态：错误码命中历史空态表，或 HTTP 404 且非 NoSuchBucket。
type obsAdapter struct {
	client *obs.ObsClient
}

func newOBS(cfg Config) (Adapter, error) {
	schema := "https"
	if !cfg.Secure {
		schema = "http"
	}
	endpoint := schema + "://" + cfg.Endpoint
	// obs.New 的 configurer 参数是未导出类型（func(*config)），无法组装切片；
	// 以显式分支覆盖「有/无临时令牌 × 有/无自定义传输层」四种组合。
	// 传输层只接受 *http.Transport 具体类型（SSRF 探测场景由控制层传入）。
	var (
		client *obs.ObsClient
		err    error
	)
	switch {
	case cfg.SessionToken != "" && cfg.Transport != nil:
		client, err = obs.New(cfg.AccessKeyID, cfg.SecretAccessKey, endpoint,
			obs.WithRegion(cfg.Region), obs.WithSecurityToken(cfg.SessionToken), obs.WithHttpTransport(cfg.Transport))
	case cfg.SessionToken != "":
		client, err = obs.New(cfg.AccessKeyID, cfg.SecretAccessKey, endpoint,
			obs.WithRegion(cfg.Region), obs.WithSecurityToken(cfg.SessionToken))
	case cfg.Transport != nil:
		client, err = obs.New(cfg.AccessKeyID, cfg.SecretAccessKey, endpoint,
			obs.WithRegion(cfg.Region), obs.WithHttpTransport(cfg.Transport))
	default:
		client, err = obs.New(cfg.AccessKeyID, cfg.SecretAccessKey, endpoint,
			obs.WithRegion(cfg.Region))
	}
	if err != nil {
		return nil, wrapOBSError(err)
	}
	return &obsAdapter{client: client}, nil
}

// wrapOBSError 把 SDK 错误包装为 *StorageError。
// SDK 既可能以 *ObsError 也可能以值类型 ObsError 返回（Error() 是值接收者），两种都要识别。
// ObsError 的 HTTP 状态码优先取 StatusCode 字段，缺失时从 Status 文本（如 "404 Not Found"）前缀解析。
func wrapOBSError(err error) error {
	if err == nil {
		return nil
	}
	var oePtr *obs.ObsError
	if errors.As(err, &oePtr) {
		return &StorageError{Code: oePtr.Code, Status: obsErrorStatus(oePtr), Err: err}
	}
	var oe obs.ObsError
	if errors.As(err, &oe) {
		return &StorageError{Code: oe.Code, Status: obsErrorStatus(&oe), Err: err}
	}
	return err
}

func obsErrorStatus(oe *obs.ObsError) int {
	if oe.StatusCode != 0 {
		return oe.StatusCode
	}
	status := strings.TrimSpace(oe.Status)
	if idx := strings.IndexByte(status, ' '); idx > 0 {
		status = status[:idx]
	}
	if code, err := strconv.Atoi(status); err == nil {
		return code
	}
	return 0
}

// obsAbsentConfig 判断错误是否属于「配置未设置」：历史空态码，或 HTTP 404 且非 NoSuchBucket。
func obsAbsentConfig(err error, codes ...string) bool {
	return absentCodes(wrapOBSError(err), codes...)
}

// —— A. Bucket 基础 ——

func (a *obsAdapter) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	res, err := a.client.ListBuckets(&obs.ListBucketsInput{})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	out := make([]BucketInfo, 0, len(res.Buckets))
	for _, b := range res.Buckets {
		out = append(out, BucketInfo{Name: b.Name, CreationDate: b.CreationDate})
	}
	return out, nil
}

func (a *obsAdapter) CreateBucket(ctx context.Context, bucket, region string) error {
	if _, err := a.client.CreateBucket(&obs.CreateBucketInput{Bucket: bucket}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) DeleteBucket(ctx context.Context, bucket string) error {
	if _, err := a.client.DeleteBucket(bucket); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

// —— B. 流式列举 ——

// obsListPageSize 单页列举条数（OBS 上限 1000）
const obsListPageSize = 1000

func (a *obsAdapter) ListObjectsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		delimiter := ""
		if !opts.Recursive {
			delimiter = "/"
		}
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = obsListPageSize
		}
		marker := ""
		for {
			req := &obs.ListObjectsInput{
				ListObjsInput: obs.ListObjsInput{Prefix: opts.Prefix, MaxKeys: maxKeys, Delimiter: delimiter},
				Bucket:        bucket,
				Marker:        marker,
			}
			res, err := a.client.ListObjects(req)
			if err != nil {
				sendOBSEntry(ctx, out, ObjectEntry{Err: wrapOBSError(err)})
				return
			}
			// 与 minio 流一致：先对象、后目录前缀
			for _, o := range res.Contents {
				if !sendOBSEntry(ctx, out, ObjectEntry{ObjectInfo: obsContentInfo(o)}) {
					return
				}
			}
			for _, cp := range res.CommonPrefixes {
				if !sendOBSEntry(ctx, out, ObjectEntry{ObjectInfo: ObjectInfo{Key: cp}}) {
					return
				}
			}
			if !res.IsTruncated {
				return
			}
			// NextMarker 仅在 delimiter 模式由服务端返回；平铺模式用本页最后 key 续读
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

func (a *obsAdapter) ListVersionsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = obsListPageSize
		}
		keyMarker, versionIDMarker := "", ""
		for {
			req := &obs.ListVersionsInput{
				ListObjsInput:   obs.ListObjsInput{Prefix: opts.Prefix, MaxKeys: maxKeys},
				Bucket:          bucket,
				KeyMarker:       keyMarker,
				VersionIdMarker: versionIDMarker,
			}
			res, err := a.client.ListVersions(req)
			if err != nil {
				sendOBSEntry(ctx, out, ObjectEntry{Err: wrapOBSError(err)})
				return
			}
			for _, info := range mergeOBSVersions(res.Versions, res.DeleteMarkers) {
				if !sendOBSEntry(ctx, out, ObjectEntry{ObjectInfo: info}) {
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

func sendOBSEntry(ctx context.Context, out chan<- ObjectEntry, entry ObjectEntry) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

// obsContentInfo 映射 V1 列举的对象条目（ETag 去引号，与 minio 路径的规整行为一致）。
func obsContentInfo(o obs.Content) ObjectInfo {
	return ObjectInfo{
		Key:          o.Key,
		Size:         o.Size,
		LastModified: o.LastModified,
		ETag:         trimOBSEtag(o.ETag),
		StorageClass: string(o.StorageClass),
	}
}

// mergeOBSVersions 把 OBS 的「版本 / 删除标记」两个数组合并为单流。
// OBS 的 XML 解码丢失了服务端的交错顺序，这里按 S3 语义近似：key 升序，同 key 内按修改时间倒序。
func mergeOBSVersions(versions []obs.Version, deleteMarkers []obs.DeleteMarker) []ObjectInfo {
	items := make([]ObjectInfo, 0, len(versions)+len(deleteMarkers))
	for _, v := range versions {
		items = append(items, ObjectInfo{
			Key:          v.Key,
			VersionID:    v.VersionId,
			IsLatest:     v.IsLatest,
			Size:         v.Size,
			LastModified: v.LastModified,
			ETag:         trimOBSEtag(v.ETag),
			StorageClass: string(v.StorageClass),
		})
	}
	for _, d := range deleteMarkers {
		items = append(items, ObjectInfo{
			Key:            d.Key,
			VersionID:      d.VersionId,
			IsLatest:       d.IsLatest,
			IsDeleteMarker: true,
			LastModified:   d.LastModified,
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

// —— C. 分页列举（OBS 仅 V1 marker 语义，token 即 marker 透传）——

func (a *obsAdapter) ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken, delimiter string, pageSize int) (*ObjectPage, error) {
	return a.listObjectsPage(ctx, bucket, prefix, continuationToken, delimiter, pageSize)
}

func (a *obsAdapter) ListObjectsByMarker(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	return a.listObjectsPage(ctx, bucket, prefix, marker, delimiter, pageSize)
}

func (a *obsAdapter) listObjectsPage(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	if pageSize <= 0 {
		pageSize = obsListPageSize
	}
	res, err := a.client.ListObjects(&obs.ListObjectsInput{
		ListObjsInput: obs.ListObjsInput{Prefix: prefix, MaxKeys: pageSize, Delimiter: delimiter},
		Bucket:        bucket,
		Marker:        marker,
	})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	page := &ObjectPage{
		Contents:    make([]ObjectInfo, 0, len(res.Contents)),
		IsTruncated: res.IsTruncated,
		NextToken:   res.NextMarker,
	}
	for _, o := range res.Contents {
		page.Contents = append(page.Contents, obsContentInfo(o))
	}
	page.CommonPrefixes = append(page.CommonPrefixes, res.CommonPrefixes...)
	if page.IsTruncated && page.NextToken == "" && len(res.Contents) > 0 {
		// 平铺模式服务端不返回 NextMarker：用本页最后 key 作为续读游标
		page.NextToken = res.Contents[len(res.Contents)-1].Key
	}
	return page, nil
}

// —— D. 对象读写 ——

func (a *obsAdapter) StatObject(ctx context.Context, bucket, key, versionID string) (*ObjectStat, error) {
	res, err := a.client.GetObjectMetadata(&obs.GetObjectMetadataInput{Bucket: bucket, Key: key, VersionId: versionID})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	stat := &ObjectStat{
		Key:          key,
		VersionID:    res.VersionId,
		Size:         res.ContentLength,
		ETag:         trimOBSEtag(res.ETag),
		ContentType:  res.ContentType,
		LastModified: res.LastModified,
		StorageClass: string(res.StorageClass),
		// 控制器按原始头读取 Content-Encoding / Cache-Control 等：由 OBS 的结构化头字段回填
		Metadata:     obsHeaderToHTTPHeader(res),
		UserMetadata: res.Metadata,
	}
	if stat.VersionID == "" {
		stat.VersionID = versionID
	}
	if stat.UserMetadata == nil {
		stat.UserMetadata = map[string]string{}
	}
	return stat, nil
}

// obsHeaderToHTTPHeader 把 OBS 的结构化响应头回填为 http.Header（键名用 S3 惯用形态）。
func obsHeaderToHTTPHeader(out *obs.GetObjectMetadataOutput) http.Header {
	h := http.Header{}
	set := func(name, value string) {
		if strings.TrimSpace(value) != "" {
			h.Set(name, value)
		}
	}
	set("Content-Type", out.ContentType)
	set("Content-Encoding", out.ContentEncoding)
	set("Content-Disposition", out.ContentDisposition)
	set("Content-Language", out.ContentLanguage)
	set("Cache-Control", out.CacheControl)
	set("Expires", out.HttpExpires)
	set("ETag", out.ETag)
	if !out.LastModified.IsZero() {
		h.Set("Last-Modified", out.LastModified.UTC().Format(http.TimeFormat))
	}
	if out.ContentLength >= 0 {
		h.Set("Content-Length", strconv.FormatInt(out.ContentLength, 10))
	}
	return h
}

func (a *obsAdapter) GetObject(ctx context.Context, bucket, key string) (*ObjectReader, error) {
	res, err := a.client.GetObject(&obs.GetObjectInput{GetObjectMetadataInput: obs.GetObjectMetadataInput{Bucket: bucket, Key: key}})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	return &ObjectReader{Body: res.Body}, nil
}

func (a *obsAdapter) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	input := &obs.PutObjectInput{
		PutObjectBasicInput: obs.PutObjectBasicInput{ObjectOperationInput: obs.ObjectOperationInput{Bucket: bucket, Key: key}},
		Body:                reader,
	}
	// 与其他适配器同口径：空值不下发——长度未知（size<=0，如 chunked 请求）时不带 Content-Length，
	// 避免负值/零值干扰 SDK 的传输判定
	if strings.TrimSpace(contentType) != "" {
		input.ContentType = contentType
	}
	if size > 0 {
		input.ContentLength = size
	}
	if _, err := a.client.PutObject(input); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) CopyObject(ctx context.Context, src CopySrc, dest CopyDest) error {
	input := &obs.CopyObjectInput{
		ObjectOperationInput: obs.ObjectOperationInput{Bucket: dest.Bucket, Key: dest.Object},
		CopySourceBucket:     src.Bucket,
		CopySourceKey:        src.Object,
		CopySourceVersionId:  src.VersionID,
	}
	if dest.ReplaceMetadata {
		// OBS 的元数据替换指令是 REPLACE_NEW（S3 的 REPLACE 用词不同）
		input.MetadataDirective = obs.ReplaceNew
		input.ContentType = dest.ContentType
		input.ContentEncoding = dest.ContentEncoding
		input.ContentDisposition = dest.ContentDisposition
		input.CacheControl = dest.CacheControl
		if !dest.Expires.IsZero() {
			input.Expires = dest.Expires.UTC().Format(http.TimeFormat)
		}
		meta := make(map[string]string, len(dest.UserMetadata))
		for k, v := range dest.UserMetadata {
			switch {
			case strings.EqualFold(k, "X-Amz-Storage-Class"):
				input.StorageClass = obs.StorageClassType(v)
			case strings.EqualFold(k, "X-Amz-Server-Side-Encryption"), strings.EqualFold(k, "X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id"):
				// OBS 的复制不提供按请求指定 SSE-KMS 主密钥的通道：默认加密由桶级配置承接，这里忽略
			default:
				meta[k] = v
			}
		}
		if len(meta) > 0 {
			input.Metadata = meta
		}
	}
	if _, err := a.client.CopyObject(input); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) RemoveObject(ctx context.Context, bucket, key, versionID string) error {
	if _, err := a.client.DeleteObject(&obs.DeleteObjectInput{Bucket: bucket, Key: key, VersionId: versionID}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

// obsDeleteBatchSize 单次批量删除的对象数上限（OBS/S3 一致为 1000）
const obsDeleteBatchSize = 1000

func (a *obsAdapter) DeleteObjectsBatch(ctx context.Context, bucket string, refs <-chan DeleteRef) DeleteErrorStream {
	out := make(chan DeleteError, 1)
	go func() {
		defer close(out)
		batch := make([]obs.ObjectToDelete, 0, obsDeleteBatchSize)
		flush := func() bool {
			if len(batch) == 0 {
				return true
			}
			res, err := a.client.DeleteObjects(&obs.DeleteObjectsInput{Bucket: bucket, Objects: batch})
			firstKey := batch[0].Key
			batch = batch[:0]
			if err != nil {
				return sendOBSDeleteError(ctx, out, DeleteError{Key: firstKey, Err: wrapOBSError(err)})
			}
			for _, e := range res.Errors {
				entry := DeleteError{
					Key: e.Key,
					Err: &StorageError{Code: e.Code, Message: obsDeleteErrorMessage(e.Code, e.Message)},
				}
				if !sendOBSDeleteError(ctx, out, entry) {
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
				batch = append(batch, obs.ObjectToDelete{Key: ref.Key, VersionId: ref.VersionID})
				if len(batch) >= obsDeleteBatchSize {
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

func sendOBSDeleteError(ctx context.Context, out chan<- DeleteError, entry DeleteError) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

func obsDeleteErrorMessage(code, message string) string {
	if code == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", code, message)
}

func (a *obsAdapter) PresignGetObject(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	res, err := a.client.CreateSignedUrl(&obs.CreateSignedUrlInput{
		Method:  obs.HttpMethodGet,
		Bucket:  bucket,
		Key:     key,
		Expires: int(expires.Seconds()),
	})
	if err != nil {
		return "", wrapOBSError(err)
	}
	return res.SignedUrl, nil
}

// —— E. 对象标签与 ACL ——

func (a *obsAdapter) GetObjectTags(ctx context.Context, bucket, key, versionID string) (map[string]string, error) {
	res, err := a.client.GetObjectTagging(&obs.GetObjectTaggingInput{
		ObjectTaggingInput: obs.ObjectTaggingInput{Bucket: bucket, Key: key, VersionId: versionID},
	})
	if err != nil {
		if obsAbsentConfig(err, "NoSuchTagSet") {
			return map[string]string{}, nil
		}
		return nil, wrapOBSError(err)
	}
	return obsTagListToMap(res.Tags), nil
}

func (a *obsAdapter) SetObjectTags(ctx context.Context, bucket, key, versionID string, tagMap map[string]string) error {
	taggingInput := obs.ObjectTaggingInput{Bucket: bucket, Key: key, VersionId: versionID}
	if len(tagMap) == 0 {
		if _, err := a.client.DeleteObjectTagging(&obs.DeleteObjectTaggingInput{ObjectTaggingInput: taggingInput}); err != nil {
			if obsAbsentConfig(err, "NoSuchTagSet") {
				return nil
			}
			return wrapOBSError(err)
		}
		return nil
	}
	if err := validateOBSTags(tagMap); err != nil {
		return err
	}
	if _, err := a.client.SetObjectTagging(&obs.SetObjectTaggingInput{
		ObjectTaggingInput: taggingInput,
		Tags:               obsTagsOf(tagMap),
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

// obsMaxObjectTags 对象标签数量上限（S3/OBS 一致）
const obsMaxObjectTags = 10

// validateOBSTags 本地校验对象标签，命中用户输入问题时返回 InvalidTag（控制层映射为 400）。
func validateOBSTags(tagMap map[string]string) error {
	if len(tagMap) > obsMaxObjectTags {
		return &StorageError{
			Code: CodeInvalidTag, Status: http.StatusBadRequest,
			Message: fmt.Sprintf("too many object tags: %d, limit is %d", len(tagMap), obsMaxObjectTags),
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

func (a *obsAdapter) GetObjectACL(ctx context.Context, bucket, key string) (*ObjectStat, error) {
	res, err := a.client.GetObjectAcl(&obs.GetObjectAclInput{Bucket: bucket, Key: key})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	stat := &ObjectStat{Key: key, Metadata: http.Header{}}
	// OBS 的 GET acl 返回授权列表（无 canned 头）：按「对所有人授权」的粒度反推三档 ACL
	stat.Metadata.Set("X-Amz-Acl", classifyOBSACL(res.Grants))
	return stat, nil
}

// classifyOBSACL 从授权列表反推三档 ACL（与 Bucket 配置同族粒度）。
func classifyOBSACL(grants []obs.Grant) string {
	read, write := false, false
	for _, g := range grants {
		uri := string(g.Grantee.URI)
		if !strings.HasSuffix(uri, "AllUsers") {
			continue
		}
		switch strings.ToUpper(string(g.Permission)) {
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

func (a *obsAdapter) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	res, err := a.client.GetBucketPolicy(bucket)
	if err != nil {
		if obsAbsentConfig(err, "NoSuchBucketPolicy") {
			return "", nil
		}
		return "", wrapOBSError(err)
	}
	if strings.TrimSpace(res.Policy) == "" {
		return "", nil
	}
	return res.Policy, nil
}

func (a *obsAdapter) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	if strings.TrimSpace(policy) == "" {
		if _, err := a.client.DeleteBucketPolicy(bucket); err != nil {
			if obsAbsentConfig(err, "NoSuchBucketPolicy") {
				return nil
			}
			return wrapOBSError(err)
		}
		return nil
	}
	// 契约是「策略原文透传」：OBS 的 put 收 Policy 字符串，天然无结构损失
	if _, err := a.client.SetBucketPolicy(&obs.SetBucketPolicyInput{Bucket: bucket, Policy: policy}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) GetBucketVersioning(ctx context.Context, bucket string) (string, error) {
	res, err := a.client.GetBucketVersioning(bucket)
	if err != nil {
		// 版本控制错误不归一化：维持「读取失败即报错」的既有语义
		return "", wrapOBSError(err)
	}
	return string(res.Status), nil
}

func (a *obsAdapter) SetBucketVersioning(ctx context.Context, bucket, status string) error {
	if _, err := a.client.SetBucketVersioning(&obs.SetBucketVersioningInput{
		Bucket:                        bucket,
		BucketVersioningConfiguration: obs.BucketVersioningConfiguration{Status: obs.VersioningStatusType(status)},
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) GetBucketEncryption(ctx context.Context, bucket string) (bool, error) {
	res, err := a.client.GetBucketEncryption(bucket)
	if err != nil {
		// OBS「未配置加密」的空态码未见公开文档：先按 404 规则归一化，云上冒烟时补录码表
		if obsAbsentConfig(err) {
			return false, nil
		}
		return false, wrapOBSError(err)
	}
	return strings.TrimSpace(res.SSEAlgorithm) != "", nil
}

func (a *obsAdapter) SetBucketEncryption(ctx context.Context, bucket string) error {
	if _, err := a.client.SetBucketEncryption(&obs.SetBucketEncryptionInput{
		Bucket: bucket,
		BucketEncryptionConfiguration: obs.BucketEncryptionConfiguration{
			// SSE-OBS：OBS 托管密钥的 AES256（对齐 S3 的 SSE-S3 语义）
			SSEAlgorithm: "AES256",
		},
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) RemoveBucketEncryption(ctx context.Context, bucket string) error {
	if _, err := a.client.DeleteBucketEncryption(bucket); err != nil {
		if obsAbsentConfig(err) {
			return nil
		}
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) GetBucketTagging(ctx context.Context, bucket string) (map[string]string, error) {
	res, err := a.client.GetBucketTagging(bucket)
	if err != nil {
		if obsAbsentConfig(err, "NoSuchTagSet") {
			return nil, nil
		}
		return nil, wrapOBSError(err)
	}
	if len(res.Tags) == 0 {
		return nil, nil
	}
	return obsTagListToMap(res.Tags), nil
}

func (a *obsAdapter) SetBucketTagging(ctx context.Context, bucket string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		if _, err := a.client.DeleteBucketTagging(bucket); err != nil {
			if obsAbsentConfig(err, "NoSuchTagSet") {
				return nil
			}
			return wrapOBSError(err)
		}
		return nil
	}
	if err := validateOBSTags(tagMap); err != nil {
		return err
	}
	if _, err := a.client.SetBucketTagging(&obs.SetBucketTaggingInput{
		Bucket:        bucket,
		BucketTagging: obs.BucketTagging{Tags: obsTagsOf(tagMap)},
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) GetBucketLifecycle(ctx context.Context, bucket string) ([]LifecycleRule, error) {
	res, err := a.client.GetBucketLifecycleConfiguration(bucket)
	if err != nil {
		if obsAbsentConfig(err, "NoSuchLifecycleConfiguration") {
			return nil, nil
		}
		return nil, wrapOBSError(err)
	}
	rules := make([]LifecycleRule, 0, len(res.LifecycleRules))
	for _, r := range res.LifecycleRules {
		rules = append(rules, LifecycleRule{
			ID:                 r.ID,
			Status:             string(r.Status),
			Prefix:             r.Prefix,
			ExpirationDays:     r.Expiration.Days,
			AbortMultipartDays: r.AbortIncompleteMultipartUpload.DaysAfterInitiation,
		})
	}
	return rules, nil
}

func (a *obsAdapter) SetBucketLifecycle(ctx context.Context, bucket string, rules []LifecycleRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除生命周期配置
		if _, err := a.client.DeleteBucketLifecycleConfiguration(bucket); err != nil {
			if obsAbsentConfig(err, "NoSuchLifecycleConfiguration") {
				return nil
			}
			return wrapOBSError(err)
		}
		return nil
	}
	obsRules := make([]obs.LifecycleRule, 0, len(rules))
	for _, r := range rules {
		rule := obs.LifecycleRule{
			ID:     r.ID,
			Status: obs.RuleStatusType(r.Status),
			Prefix: r.Prefix,
		}
		if r.ExpirationDays > 0 {
			rule.Expiration = obs.Expiration{Days: r.ExpirationDays}
		}
		if r.AbortMultipartDays > 0 {
			rule.AbortIncompleteMultipartUpload = obs.AbortIncompleteMultipartUpload{DaysAfterInitiation: r.AbortMultipartDays}
		}
		obsRules = append(obsRules, rule)
	}
	if _, err := a.client.SetBucketLifecycleConfiguration(&obs.SetBucketLifecycleConfigurationInput{
		Bucket:                       bucket,
		BucketLifecycleConfiguration: obs.BucketLifecycleConfiguration{LifecycleRules: obsRules},
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) GetBucketCors(ctx context.Context, bucket string) ([]CORSRule, error) {
	res, err := a.client.GetBucketCors(bucket)
	if err != nil {
		if obsAbsentConfig(err, "NoSuchCORSConfiguration") {
			return nil, nil
		}
		return nil, wrapOBSError(err)
	}
	out := make([]CORSRule, 0, len(res.CorsRules))
	for _, r := range res.CorsRules {
		out = append(out, CORSRule{
			ID:             r.ID,
			AllowedOrigins: r.AllowedOrigin,
			AllowedMethods: r.AllowedMethod,
			AllowedHeaders: r.AllowedHeader,
			ExposeHeaders:  r.ExposeHeader,
			MaxAgeSeconds:  r.MaxAgeSeconds,
		})
	}
	return out, nil
}

func (a *obsAdapter) SetBucketCors(ctx context.Context, bucket string, rules []CORSRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除 CORS 配置
		if _, err := a.client.DeleteBucketCors(bucket); err != nil {
			if obsAbsentConfig(err, "NoSuchCORSConfiguration") {
				return nil
			}
			return wrapOBSError(err)
		}
		return nil
	}
	obsRules := make([]obs.CorsRule, 0, len(rules))
	for _, r := range rules {
		obsRules = append(obsRules, obs.CorsRule{
			ID:            r.ID,
			AllowedOrigin: r.AllowedOrigins,
			AllowedMethod: r.AllowedMethods,
			AllowedHeader: r.AllowedHeaders,
			ExposeHeader:  r.ExposeHeaders,
			MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	if _, err := a.client.SetBucketCors(&obs.SetBucketCorsInput{
		Bucket:     bucket,
		BucketCors: obs.BucketCors{CorsRules: obsRules},
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

// —— G. 分片上传 ——

func (a *obsAdapter) InitMultipart(ctx context.Context, bucket, key, contentType, storageClass string) (string, error) {
	input := &obs.InitiateMultipartUploadInput{
		ObjectOperationInput: obs.ObjectOperationInput{Bucket: bucket, Key: key, StorageClass: obs.StorageClassType(storageClass)},
	}
	input.ContentType = contentType
	res, err := a.client.InitiateMultipartUpload(input)
	if err != nil {
		return "", wrapOBSError(err)
	}
	return res.UploadId, nil
}

func (a *obsAdapter) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*PartInfo, error) {
	// OBS 的分片上传以 PartSize 承载长度（readerWrapper 按此计数），不是 ContentLength
	res, err := a.client.UploadPart(&obs.UploadPartInput{
		Bucket:     bucket,
		Key:        key,
		UploadId:   uploadID,
		PartNumber: partNumber,
		Body:       reader,
		PartSize:   size,
	})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	return &PartInfo{PartNumber: partNumber, ETag: trimOBSEtag(res.ETag), Size: size}, nil
}

func (a *obsAdapter) ListParts(ctx context.Context, bucket, key, uploadID string, partNumberMarker, maxParts int) (*PartsPage, error) {
	if maxParts <= 0 {
		maxParts = obsListPageSize
	}
	res, err := a.client.ListParts(&obs.ListPartsInput{
		Bucket:           bucket,
		Key:              key,
		UploadId:         uploadID,
		MaxParts:         maxParts,
		PartNumberMarker: partNumberMarker,
	})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	page := &PartsPage{
		IsTruncated:          res.IsTruncated,
		NextPartNumberMarker: res.NextPartNumberMarker,
	}
	page.Parts = make([]PartInfo, 0, len(res.Parts))
	for _, p := range res.Parts {
		page.Parts = append(page.Parts, PartInfo{
			PartNumber:   p.PartNumber,
			ETag:         trimOBSEtag(p.ETag),
			Size:         p.Size,
			LastModified: p.LastModified,
		})
	}
	return page, nil
}

func (a *obsAdapter) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompletedObject, error) {
	obsParts := make([]obs.Part, 0, len(parts))
	for _, p := range parts {
		obsParts = append(obsParts, obs.Part{PartNumber: p.PartNumber, ETag: p.ETag})
	}
	res, err := a.client.CompleteMultipartUpload(&obs.CompleteMultipartUploadInput{
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadID,
		Parts:    obsParts,
	})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	// OBS/S3 的 Complete 响应不含对象大小（与 minio 路径的既有语义一致，Size 恒 0）
	return &CompletedObject{Key: res.Key, ETag: trimOBSEtag(res.ETag)}, nil
}

func (a *obsAdapter) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	if _, err := a.client.AbortMultipartUpload(&obs.AbortMultipartUploadInput{
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadID,
	}); err != nil {
		return wrapOBSError(err)
	}
	return nil
}

func (a *obsAdapter) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (*UploadsPage, error) {
	if maxUploads <= 0 {
		maxUploads = obsListPageSize
	}
	res, err := a.client.ListMultipartUploads(&obs.ListMultipartUploadsInput{
		Bucket:         bucket,
		Prefix:         prefix,
		MaxUploads:     maxUploads,
		KeyMarker:      keyMarker,
		UploadIdMarker: uploadIDMarker,
	})
	if err != nil {
		return nil, wrapOBSError(err)
	}
	page := &UploadsPage{
		IsTruncated:        res.IsTruncated,
		NextKeyMarker:      res.NextKeyMarker,
		NextUploadIDMarker: res.NextUploadIdMarker,
	}
	page.Uploads = make([]MultipartUploadInfo, 0, len(res.Uploads))
	for _, u := range res.Uploads {
		page.Uploads = append(page.Uploads, MultipartUploadInfo{
			Key:       u.Key,
			UploadID:  u.UploadId,
			Initiated: u.Initiated,
			// OBS 的 ListMultipartUploads 不返回已上传大小（与 minio 路径一致，保持 0）
		})
	}
	return page, nil
}

// —— 工具函数 ——

// trimOBSEtag 去掉 ETag 的引号（与 minio 路径的规整行为一致）。
func trimOBSEtag(etag string) string {
	return strings.Trim(strings.TrimSpace(etag), `"`)
}

// obsTagsOf 构造 OBS 的标签列表。
func obsTagsOf(tagMap map[string]string) []obs.Tag {
	tags := make([]obs.Tag, 0, len(tagMap))
	for k, v := range tagMap {
		tags = append(tags, obs.Tag{Key: k, Value: v})
	}
	return tags
}

// obsTagListToMap 摊平 OBS 的标签列表。
func obsTagListToMap(tags []obs.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		out[t.Key] = t.Value
	}
	return out
}
