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
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// awsAdapter 是 AWS S3 的原生适配器（aws-sdk-go-v2）。
//
// 语义要点：
//   - 端点支持自定义（BaseEndpoint）：既覆盖 AWS 各区默认端点，也是「本地 MinIO 接线验证」的通道；
//     IP / localhost 端点强制 path-style 寻址（virtual-host 子域形态对它们不成立）。
//   - 版本列举的 XML 解码把 Version 与 DeleteMarker 拆成两个数组（文档序丢失）：
//     与 COS/OBS 同口径，按「key 升序、同 key 内修改时间倒序」合并近似还原。
//   - 刻意不引入 aws-sdk-go-v2/config 模块：静态凭据 + 显式 Options 已足够，少一套加载链。
type awsAdapter struct {
	client *s3.Client
}

// NewAWS 构造 AWS S3 原生适配器。
func NewAWS(cfg Config) (Adapter, error) {
	schema := "https"
	if !cfg.Secure {
		schema = "http"
	}
	options := s3.Options{
		Region: cfg.Region,
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)),
		BaseEndpoint: aws.String(schema + "://" + cfg.Endpoint),
		UsePathStyle: awsNeedsPathStyle(cfg.Endpoint),
	}
	if cfg.Transport != nil {
		options.HTTPClient = &http.Client{Transport: cfg.Transport}
	}
	return &awsAdapter{client: s3.New(options)}, nil
}

// awsNeedsPathStyle 判断端点是否只能走 path-style：IP 与 localhost 域名无法承载 virtual-host 子域。
func awsNeedsPathStyle(endpoint string) bool {
	host := strings.ToLower(strings.TrimSpace(endpoint))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	if net.ParseIP(host) != nil {
		return true
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// wrapAWSError 把 SDK 错误包装为 *StorageError：错误码走 smithy APIError，HTTP 状态走 ResponseError。
func wrapAWSError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	status := 0
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		status = respErr.HTTPStatusCode()
	}
	return &StorageError{Code: apiErr.ErrorCode(), Status: status, Err: err}
}

// awsAbsentConfig 判断错误是否属于「配置未设置」：历史空态码，或 HTTP 404 且非 NoSuchBucket。
func awsAbsentConfig(err error, codes ...string) bool {
	return absentCodes(wrapAWSError(err), codes...)
}

// —— A. Bucket 基础 ——

func (a *awsAdapter) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	res, err := a.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, wrapAWSError(err)
	}
	out := make([]BucketInfo, 0, len(res.Buckets))
	for _, b := range res.Buckets {
		info := BucketInfo{Name: awsString(b.Name)}
		if b.CreationDate != nil {
			info.CreationDate = *b.CreationDate
		}
		out = append(out, info)
	}
	return out, nil
}

func (a *awsAdapter) CreateBucket(ctx context.Context, bucket, region string) error {
	in := &s3.CreateBucketInput{Bucket: aws.String(bucket)}
	// AWS 规则：us-east-1 不允许在请求体里带区域配置（MinIO 兼容两种形态）
	if region != "" && region != "us-east-1" {
		in.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}
	if _, err := a.client.CreateBucket(ctx, in); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) DeleteBucket(ctx context.Context, bucket string) error {
	if _, err := a.client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

// —— B. 流式列举 ——

// awsListPageSize 单页列举条数（S3 上限 1000）
const awsListPageSize = 1000

func (a *awsAdapter) ListObjectsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		delimiter := ""
		if !opts.Recursive {
			delimiter = "/"
		}
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = awsListPageSize
		}
		token := ""
		for {
			in := &s3.ListObjectsV2Input{
				Bucket:  aws.String(bucket),
				MaxKeys: aws.Int32(int32(maxKeys)),
			}
			if opts.Prefix != "" {
				in.Prefix = aws.String(opts.Prefix)
			}
			if delimiter != "" {
				in.Delimiter = aws.String(delimiter)
			}
			if token != "" {
				in.ContinuationToken = aws.String(token)
			}
			res, err := a.client.ListObjectsV2(ctx, in)
			if err != nil {
				sendAWSEntry(ctx, out, ObjectEntry{Err: wrapAWSError(err)})
				return
			}
			// 与 minio 流一致：先对象、后目录前缀
			for _, o := range res.Contents {
				if !sendAWSEntry(ctx, out, ObjectEntry{ObjectInfo: awsObjectInfo(o)}) {
					return
				}
			}
			for _, cp := range res.CommonPrefixes {
				if !sendAWSEntry(ctx, out, ObjectEntry{ObjectInfo: ObjectInfo{Key: awsString(cp.Prefix)}}) {
					return
				}
			}
			if !awsBool(res.IsTruncated) {
				return
			}
			next := awsString(res.NextContinuationToken)
			if next == "" || next == token {
				return
			}
			token = next
		}
	}()
	return out
}

func (a *awsAdapter) ListVersionsStream(ctx context.Context, bucket string, opts ListOptions) ObjectStream {
	out := make(chan ObjectEntry, 1)
	go func() {
		defer close(out)
		maxKeys := opts.MaxKeys
		if maxKeys <= 0 {
			maxKeys = awsListPageSize
		}
		keyMarker, versionIDMarker := "", ""
		for {
			in := &s3.ListObjectVersionsInput{
				Bucket:  aws.String(bucket),
				MaxKeys: aws.Int32(int32(maxKeys)),
			}
			if opts.Prefix != "" {
				in.Prefix = aws.String(opts.Prefix)
			}
			if keyMarker != "" {
				in.KeyMarker = aws.String(keyMarker)
			}
			if versionIDMarker != "" {
				in.VersionIdMarker = aws.String(versionIDMarker)
			}
			res, err := a.client.ListObjectVersions(ctx, in)
			if err != nil {
				sendAWSEntry(ctx, out, ObjectEntry{Err: wrapAWSError(err)})
				return
			}
			for _, info := range mergeAWSVersions(res.Versions, res.DeleteMarkers) {
				if !sendAWSEntry(ctx, out, ObjectEntry{ObjectInfo: info}) {
					return
				}
			}
			if !awsBool(res.IsTruncated) {
				return
			}
			nextKey, nextVersion := awsString(res.NextKeyMarker), awsString(res.NextVersionIdMarker)
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

func sendAWSEntry(ctx context.Context, out chan<- ObjectEntry, entry ObjectEntry) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

// awsObjectInfo 映射 V2 列举的对象条目（ETag 去引号，与 minio 路径的规整行为一致）。
func awsObjectInfo(o types.Object) ObjectInfo {
	info := ObjectInfo{
		Key:          awsString(o.Key),
		ETag:         trimAWSEtag(awsString(o.ETag)),
		StorageClass: string(o.StorageClass),
	}
	if o.Size != nil {
		info.Size = *o.Size
	}
	if o.LastModified != nil {
		info.LastModified = *o.LastModified
	}
	return info
}

// mergeAWSVersions 把 AWS 的「版本 / 删除标记」两个数组合并为单流。
// XML 解码丢失了服务端的交错顺序，这里按 S3 语义近似：key 升序，同 key 内按修改时间倒序。
func mergeAWSVersions(versions []types.ObjectVersion, deleteMarkers []types.DeleteMarkerEntry) []ObjectInfo {
	items := make([]ObjectInfo, 0, len(versions)+len(deleteMarkers))
	for _, v := range versions {
		info := ObjectInfo{
			Key:          awsString(v.Key),
			VersionID:    awsString(v.VersionId),
			IsLatest:     awsBool(v.IsLatest),
			ETag:         trimAWSEtag(awsString(v.ETag)),
			StorageClass: string(v.StorageClass),
		}
		if v.Size != nil {
			info.Size = *v.Size
		}
		if v.LastModified != nil {
			info.LastModified = *v.LastModified
		}
		items = append(items, info)
	}
	for _, d := range deleteMarkers {
		info := ObjectInfo{
			Key:            awsString(d.Key),
			VersionID:      awsString(d.VersionId),
			IsLatest:       awsBool(d.IsLatest),
			IsDeleteMarker: true,
		}
		if d.LastModified != nil {
			info.LastModified = *d.LastModified
		}
		items = append(items, info)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Key != items[j].Key {
			return items[i].Key < items[j].Key
		}
		return items[i].LastModified.After(items[j].LastModified)
	})
	return items
}

// —— C. 分页列举 ——

func (a *awsAdapter) ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken, delimiter string, pageSize int) (*ObjectPage, error) {
	if pageSize <= 0 {
		pageSize = awsListPageSize
	}
	in := &s3.ListObjectsV2Input{Bucket: aws.String(bucket), MaxKeys: aws.Int32(int32(pageSize))}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	if continuationToken != "" {
		in.ContinuationToken = aws.String(continuationToken)
	}
	if delimiter != "" {
		in.Delimiter = aws.String(delimiter)
	}
	res, err := a.client.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, wrapAWSError(err)
	}
	page := &ObjectPage{
		Contents:    make([]ObjectInfo, 0, len(res.Contents)),
		IsTruncated: awsBool(res.IsTruncated),
		NextToken:   awsString(res.NextContinuationToken),
	}
	for _, o := range res.Contents {
		page.Contents = append(page.Contents, awsObjectInfo(o))
	}
	for _, cp := range res.CommonPrefixes {
		page.CommonPrefixes = append(page.CommonPrefixes, awsString(cp.Prefix))
	}
	return page, nil
}

func (a *awsAdapter) ListObjectsByMarker(ctx context.Context, bucket, prefix, marker, delimiter string, pageSize int) (*ObjectPage, error) {
	if pageSize <= 0 {
		pageSize = awsListPageSize
	}
	in := &s3.ListObjectsInput{Bucket: aws.String(bucket), MaxKeys: aws.Int32(int32(pageSize))}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	if marker != "" {
		in.Marker = aws.String(marker)
	}
	if delimiter != "" {
		in.Delimiter = aws.String(delimiter)
	}
	res, err := a.client.ListObjects(ctx, in)
	if err != nil {
		return nil, wrapAWSError(err)
	}
	page := &ObjectPage{
		Contents:    make([]ObjectInfo, 0, len(res.Contents)),
		IsTruncated: awsBool(res.IsTruncated),
		NextToken:   awsString(res.NextMarker),
	}
	contents := res.Contents
	for _, o := range contents {
		page.Contents = append(page.Contents, awsObjectInfo(o))
	}
	for _, cp := range res.CommonPrefixes {
		page.CommonPrefixes = append(page.CommonPrefixes, awsString(cp.Prefix))
	}
	if page.IsTruncated && page.NextToken == "" && len(page.Contents) > 0 {
		// 平铺模式服务端不返回 NextMarker：用本页最后 key 作为续读游标
		page.NextToken = page.Contents[len(page.Contents)-1].Key
	}
	return page, nil
}

// —— D. 对象读写 ——

func (a *awsAdapter) StatObject(ctx context.Context, bucket, key, versionID string) (*ObjectStat, error) {
	in := &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	res, err := a.client.HeadObject(ctx, in)
	if err != nil {
		return nil, wrapAWSError(err)
	}
	stat := &ObjectStat{
		Key:          key,
		VersionID:    awsString(res.VersionId),
		ETag:         trimAWSEtag(awsString(res.ETag)),
		ContentType:  awsString(res.ContentType),
		StorageClass: string(res.StorageClass),
		Metadata:     awsHeaderFromHead(res),
		UserMetadata: res.Metadata,
	}
	if res.ContentLength != nil {
		stat.Size = *res.ContentLength
	}
	if res.LastModified != nil {
		stat.LastModified = *res.LastModified
	}
	if res.Expires != nil {
		stat.Expires = *res.Expires
	}
	if stat.VersionID == "" {
		stat.VersionID = versionID
	}
	if stat.UserMetadata == nil {
		stat.UserMetadata = map[string]string{}
	}
	return stat, nil
}

// awsHeaderFromHead 把 HeadObject 的结构化头字段回填为 http.Header（控制器按原始头读取）。
func awsHeaderFromHead(res *s3.HeadObjectOutput) http.Header {
	h := http.Header{}
	set := func(name, value string) {
		if strings.TrimSpace(value) != "" {
			h.Set(name, value)
		}
	}
	set("Content-Type", awsString(res.ContentType))
	set("Content-Encoding", awsString(res.ContentEncoding))
	set("Content-Disposition", awsString(res.ContentDisposition))
	set("Content-Language", awsString(res.ContentLanguage))
	set("Cache-Control", awsString(res.CacheControl))
	set("ETag", awsString(res.ETag))
	if res.ContentLength != nil {
		h.Set("Content-Length", strconv.FormatInt(*res.ContentLength, 10))
	}
	if res.LastModified != nil {
		h.Set("Last-Modified", res.LastModified.UTC().Format(http.TimeFormat))
	}
	return h
}

func (a *awsAdapter) GetObject(ctx context.Context, bucket, key string) (*ObjectReader, error) {
	res, err := a.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, wrapAWSError(err)
	}
	return &ObjectReader{Body: res.Body}, nil
}

func (a *awsAdapter) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	in := &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: reader}
	if size > 0 {
		in.ContentLength = aws.Int64(size)
	}
	if strings.TrimSpace(contentType) != "" {
		in.ContentType = aws.String(contentType)
	}
	if _, err := a.client.PutObject(ctx, in); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) CopyObject(ctx context.Context, src CopySrc, dest CopyDest) error {
	in := &s3.CopyObjectInput{
		Bucket:     aws.String(dest.Bucket),
		Key:        aws.String(dest.Object),
		CopySource: aws.String(awsCopySource(src)),
	}
	if dest.ReplaceMetadata {
		in.MetadataDirective = types.MetadataDirectiveReplace
		if dest.ContentType != "" {
			in.ContentType = aws.String(dest.ContentType)
		}
		if dest.ContentEncoding != "" {
			in.ContentEncoding = aws.String(dest.ContentEncoding)
		}
		if dest.ContentDisposition != "" {
			in.ContentDisposition = aws.String(dest.ContentDisposition)
		}
		if dest.ContentLanguage != "" {
			in.ContentLanguage = aws.String(dest.ContentLanguage)
		}
		if dest.CacheControl != "" {
			in.CacheControl = aws.String(dest.CacheControl)
		}
		if !dest.Expires.IsZero() {
			in.Expires = aws.Time(dest.Expires)
		}
		// 系统元数据由 CopyDest 显式字段承载，UserMetadata 只放用户自定义项
		if dest.StorageClass != "" {
			in.StorageClass = types.StorageClass(dest.StorageClass)
		}
		if dest.SSEAlgorithm != "" {
			in.ServerSideEncryption = types.ServerSideEncryption(dest.SSEAlgorithm)
		}
		if dest.SSEKMSKeyID != "" {
			in.SSEKMSKeyId = aws.String(dest.SSEKMSKeyID)
		}
		if len(dest.UserMetadata) > 0 {
			in.Metadata = dest.UserMetadata
		}
	}
	if _, err := a.client.CopyObject(ctx, in); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

// awsCopySource 构造 CopySource（bucket/key[?versionId=]，key 按段 URL 编码）。
func awsCopySource(src CopySrc) string {
	segments := strings.Split(src.Object, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	source := src.Bucket + "/" + strings.Join(segments, "/")
	if src.VersionID != "" {
		source += "?versionId=" + url.QueryEscape(src.VersionID)
	}
	return source
}

func (a *awsAdapter) RemoveObject(ctx context.Context, bucket, key, versionID string) error {
	in := &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	if _, err := a.client.DeleteObject(ctx, in); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

// awsDeleteBatchSize 单次批量删除的对象数上限（S3 规定 1000）
const awsDeleteBatchSize = 1000

func (a *awsAdapter) DeleteObjectsBatch(ctx context.Context, bucket string, refs <-chan DeleteRef) DeleteErrorStream {
	out := make(chan DeleteError, 1)
	go func() {
		defer close(out)
		batch := make([]types.ObjectIdentifier, 0, awsDeleteBatchSize)
		flush := func() bool {
			if len(batch) == 0 {
				return true
			}
			requested := batch
			batch = batch[:0]
			res, err := a.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket),
				Delete: &types.Delete{Objects: requested, Quiet: aws.Bool(false)},
			})
			if err != nil {
				return sendAWSDeleteError(ctx, out, DeleteError{Key: awsString(requested[0].Key), Err: wrapAWSError(err)})
			}
			for _, e := range res.Errors {
				entry := DeleteError{
					Key: awsString(e.Key),
					Err: &StorageError{Code: awsString(e.Code), Message: awsDeleteErrorMessage(awsString(e.Code), awsString(e.Message))},
				}
				if !sendAWSDeleteError(ctx, out, entry) {
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
				item := types.ObjectIdentifier{Key: aws.String(ref.Key)}
				if ref.VersionID != "" {
					item.VersionId = aws.String(ref.VersionID)
				}
				batch = append(batch, item)
				if len(batch) >= awsDeleteBatchSize {
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

func sendAWSDeleteError(ctx context.Context, out chan<- DeleteError, entry DeleteError) bool {
	select {
	case out <- entry:
		return true
	case <-ctx.Done():
		return false
	}
}

func awsDeleteErrorMessage(code, message string) string {
	if code == "" {
		return message
	}
	return code + ": " + message
}

func (a *awsAdapter) PresignGetObject(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	presigner := s3.NewPresignClient(a.client)
	res, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
		func(o *s3.PresignOptions) { o.Expires = expires })
	if err != nil {
		return "", wrapAWSError(err)
	}
	return res.URL, nil
}

// —— E. 对象标签与 ACL ——

func (a *awsAdapter) GetObjectTags(ctx context.Context, bucket, key, versionID string) (map[string]string, error) {
	in := &s3.GetObjectTaggingInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	res, err := a.client.GetObjectTagging(ctx, in)
	if err != nil {
		if awsAbsentConfig(err, "NoSuchTagSet") {
			return map[string]string{}, nil
		}
		return nil, wrapAWSError(err)
	}
	return awsTagListToMap(res.TagSet), nil
}

func (a *awsAdapter) SetObjectTags(ctx context.Context, bucket, key, versionID string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		in := &s3.DeleteObjectTaggingInput{Bucket: aws.String(bucket), Key: aws.String(key)}
		if versionID != "" {
			in.VersionId = aws.String(versionID)
		}
		if _, err := a.client.DeleteObjectTagging(ctx, in); err != nil {
			return wrapAWSError(err)
		}
		return nil
	}
	if err := validateAWSTags(tagMap); err != nil {
		return err
	}
	in := &s3.PutObjectTaggingInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		Tagging: &types.Tagging{TagSet: awsTagsOf(tagMap)},
	}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	if _, err := a.client.PutObjectTagging(ctx, in); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

// awsMaxObjectTags 对象标签数量上限（S3 规定 10）
const awsMaxObjectTags = 10

// validateAWSTags 本地校验对象标签，命中用户输入问题时返回 InvalidTag（控制层映射为 400）。
func validateAWSTags(tagMap map[string]string) error {
	if len(tagMap) > awsMaxObjectTags {
		return &StorageError{
			Code: CodeInvalidTag, Status: http.StatusBadRequest,
			Message: "too many object tags: " + strconv.Itoa(len(tagMap)) + ", limit is " + strconv.Itoa(awsMaxObjectTags),
		}
	}
	for k, v := range tagMap {
		if strings.TrimSpace(k) == "" {
			return &StorageError{Code: CodeInvalidTag, Status: http.StatusBadRequest, Message: "object tag key is required"}
		}
		if len(k) > 128 || len(v) > 256 {
			return &StorageError{
				Code: CodeInvalidTag, Status: http.StatusBadRequest,
				Message: "object tag " + k + " exceeds the length limit",
			}
		}
	}
	return nil
}

func (a *awsAdapter) GetObjectACL(ctx context.Context, bucket, key string) (*ObjectStat, error) {
	res, err := a.client.GetObjectAcl(ctx, &s3.GetObjectAclInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, wrapAWSError(err)
	}
	stat := &ObjectStat{Key: key, Metadata: http.Header{}}
	// AWS 的 GET acl 返回授权列表（无 canned 头）：按「对所有人授权」的粒度反推三档 ACL
	stat.CannedACL = classifyAWSACL(res.Grants)
	return stat, nil
}

// classifyAWSACL 从授权列表反推三档 ACL（与 Bucket 配置同族粒度）。
func classifyAWSACL(grants []types.Grant) string {
	read, write := false, false
	for _, g := range grants {
		if g.Grantee == nil {
			continue
		}
		uri := awsString(g.Grantee.URI)
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

func (a *awsAdapter) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	res, err := a.client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	if err != nil {
		if awsAbsentConfig(err, "NoSuchBucketPolicy") {
			return "", nil
		}
		return "", wrapAWSError(err)
	}
	return awsString(res.Policy), nil
}

func (a *awsAdapter) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	if strings.TrimSpace(policy) == "" {
		if _, err := a.client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{Bucket: aws.String(bucket)}); err != nil {
			if awsAbsentConfig(err, "NoSuchBucketPolicy") {
				return nil
			}
			return wrapAWSError(err)
		}
		return nil
	}
	// 契约是「策略原文透传」：S3 的 put 收策略字符串，天然无结构损失
	if _, err := a.client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(policy),
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) GetBucketVersioning(ctx context.Context, bucket string) (string, error) {
	res, err := a.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		// 版本控制错误不归一化：维持「读取失败即报错」的既有语义
		return "", wrapAWSError(err)
	}
	return string(res.Status), nil
}

func (a *awsAdapter) SetBucketVersioning(ctx context.Context, bucket, status string) error {
	if _, err := a.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket:                  aws.String(bucket),
		VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatus(status)},
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) GetBucketEncryption(ctx context.Context, bucket string) (bool, error) {
	res, err := a.client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	if err != nil {
		if awsAbsentConfig(err, "ServerSideEncryptionConfigurationNotFoundError") {
			return false, nil
		}
		return false, wrapAWSError(err)
	}
	if res.ServerSideEncryptionConfiguration == nil {
		return false, nil
	}
	for _, rule := range res.ServerSideEncryptionConfiguration.Rules {
		if rule.ApplyServerSideEncryptionByDefault != nil &&
			strings.TrimSpace(string(rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm)) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (a *awsAdapter) SetBucketEncryption(ctx context.Context, bucket string) error {
	if _, err := a.client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					// SSE-S3：S3 托管密钥的 AES256
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm: types.ServerSideEncryptionAes256,
					},
				},
			},
		},
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) RemoveBucketEncryption(ctx context.Context, bucket string) error {
	if _, err := a.client.DeleteBucketEncryption(ctx, &s3.DeleteBucketEncryptionInput{Bucket: aws.String(bucket)}); err != nil {
		if awsAbsentConfig(err, "ServerSideEncryptionConfigurationNotFoundError") {
			return nil
		}
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) GetBucketTagging(ctx context.Context, bucket string) (map[string]string, error) {
	res, err := a.client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
	if err != nil {
		if awsAbsentConfig(err, "NoSuchTagSet") {
			return nil, nil
		}
		return nil, wrapAWSError(err)
	}
	if len(res.TagSet) == 0 {
		return nil, nil
	}
	return awsTagListToMap(res.TagSet), nil
}

func (a *awsAdapter) SetBucketTagging(ctx context.Context, bucket string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		if _, err := a.client.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{Bucket: aws.String(bucket)}); err != nil {
			if awsAbsentConfig(err, "NoSuchTagSet") {
				return nil
			}
			return wrapAWSError(err)
		}
		return nil
	}
	if err := validateAWSTags(tagMap); err != nil {
		return err
	}
	if _, err := a.client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket:  aws.String(bucket),
		Tagging: &types.Tagging{TagSet: awsTagsOf(tagMap)},
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) GetBucketLifecycle(ctx context.Context, bucket string) ([]LifecycleRule, error) {
	res, err := a.client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})
	if err != nil {
		if awsAbsentConfig(err, "NoSuchLifecycleConfiguration") {
			return nil, nil
		}
		return nil, wrapAWSError(err)
	}
	rules := make([]LifecycleRule, 0, len(res.Rules))
	for _, r := range res.Rules {
		rule := LifecycleRule{
			ID:     awsString(r.ID),
			Status: string(r.Status),
		}
		if r.Filter != nil && r.Filter.Prefix != nil {
			rule.Prefix = *r.Filter.Prefix
		} else if r.Prefix != nil {
			rule.Prefix = *r.Prefix
		}
		if r.Expiration != nil && r.Expiration.Days != nil {
			rule.ExpirationDays = int(*r.Expiration.Days)
		}
		if r.AbortIncompleteMultipartUpload != nil && r.AbortIncompleteMultipartUpload.DaysAfterInitiation != nil {
			rule.AbortMultipartDays = int(*r.AbortIncompleteMultipartUpload.DaysAfterInitiation)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (a *awsAdapter) SetBucketLifecycle(ctx context.Context, bucket string, rules []LifecycleRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除生命周期配置
		if _, err := a.client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{Bucket: aws.String(bucket)}); err != nil {
			if awsAbsentConfig(err, "NoSuchLifecycleConfiguration") {
				return nil
			}
			return wrapAWSError(err)
		}
		return nil
	}
	awsRules := make([]types.LifecycleRule, 0, len(rules))
	for _, r := range rules {
		rule := types.LifecycleRule{
			ID:     aws.String(r.ID),
			Status: types.ExpirationStatus(r.Status),
		}
		if r.Prefix != "" {
			rule.Filter = &types.LifecycleRuleFilter{Prefix: aws.String(r.Prefix)}
		}
		if r.ExpirationDays > 0 {
			days := int32(r.ExpirationDays)
			rule.Expiration = &types.LifecycleExpiration{Days: aws.Int32(days)}
		}
		if r.AbortMultipartDays > 0 {
			days := int32(r.AbortMultipartDays)
			rule.AbortIncompleteMultipartUpload = &types.AbortIncompleteMultipartUpload{DaysAfterInitiation: aws.Int32(days)}
		}
		awsRules = append(awsRules, rule)
	}
	if _, err := a.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{Rules: awsRules},
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) GetBucketCors(ctx context.Context, bucket string) ([]CORSRule, error) {
	res, err := a.client.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(bucket)})
	if err != nil {
		if awsAbsentConfig(err, "NoSuchCORSConfiguration") {
			return nil, nil
		}
		return nil, wrapAWSError(err)
	}
	out := make([]CORSRule, 0, len(res.CORSRules))
	for _, r := range res.CORSRules {
		rule := CORSRule{
			ID:             awsString(r.ID),
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

func (a *awsAdapter) SetBucketCors(ctx context.Context, bucket string, rules []CORSRule) error {
	if len(rules) == 0 {
		// 空规则集合 = 删除 CORS 配置
		if _, err := a.client.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{Bucket: aws.String(bucket)}); err != nil {
			if awsAbsentConfig(err, "NoSuchCORSConfiguration") {
				return nil
			}
			return wrapAWSError(err)
		}
		return nil
	}
	awsRules := make([]types.CORSRule, 0, len(rules))
	for _, r := range rules {
		rule := types.CORSRule{
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
		}
		if r.ID != "" {
			rule.ID = aws.String(r.ID)
		}
		if len(r.AllowedHeaders) > 0 {
			rule.AllowedHeaders = r.AllowedHeaders
		}
		if len(r.ExposeHeaders) > 0 {
			rule.ExposeHeaders = r.ExposeHeaders
		}
		if r.MaxAgeSeconds > 0 {
			seconds := int32(r.MaxAgeSeconds)
			rule.MaxAgeSeconds = aws.Int32(seconds)
		}
		awsRules = append(awsRules, rule)
	}
	if _, err := a.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket:            aws.String(bucket),
		CORSConfiguration: &types.CORSConfiguration{CORSRules: awsRules},
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

// —— G. 分片上传 ——

func (a *awsAdapter) InitMultipart(ctx context.Context, bucket, key, contentType, storageClass string) (string, error) {
	in := &s3.CreateMultipartUploadInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if strings.TrimSpace(contentType) != "" {
		in.ContentType = aws.String(contentType)
	}
	if strings.TrimSpace(storageClass) != "" {
		in.StorageClass = types.StorageClass(storageClass)
	}
	res, err := a.client.CreateMultipartUpload(ctx, in)
	if err != nil {
		return "", wrapAWSError(err)
	}
	return awsString(res.UploadId), nil
}

func (a *awsAdapter) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (*PartInfo, error) {
	in := &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       reader,
	}
	if size > 0 {
		in.ContentLength = aws.Int64(size)
	}
	res, err := a.client.UploadPart(ctx, in)
	if err != nil {
		return nil, wrapAWSError(err)
	}
	return &PartInfo{PartNumber: partNumber, ETag: trimAWSEtag(awsString(res.ETag)), Size: size}, nil
}

func (a *awsAdapter) ListParts(ctx context.Context, bucket, key, uploadID string, partNumberMarker, maxParts int) (*PartsPage, error) {
	if maxParts <= 0 {
		maxParts = awsListPageSize
	}
	in := &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MaxParts: aws.Int32(int32(maxParts)),
	}
	if partNumberMarker > 0 {
		in.PartNumberMarker = aws.String(strconv.Itoa(partNumberMarker))
	}
	res, err := a.client.ListParts(ctx, in)
	if err != nil {
		return nil, wrapAWSError(err)
	}
	page := &PartsPage{IsTruncated: awsBool(res.IsTruncated)}
	if res.NextPartNumberMarker != nil {
		page.NextPartNumberMarker, _ = strconv.Atoi(*res.NextPartNumberMarker)
	}
	page.Parts = make([]PartInfo, 0, len(res.Parts))
	for _, p := range res.Parts {
		info := PartInfo{ETag: trimAWSEtag(awsString(p.ETag))}
		if p.PartNumber != nil {
			info.PartNumber = int(*p.PartNumber)
		}
		if p.Size != nil {
			info.Size = *p.Size
		}
		if p.LastModified != nil {
			info.LastModified = *p.LastModified
		}
		page.Parts = append(page.Parts, info)
	}
	return page, nil
}

func (a *awsAdapter) CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompletedObject, error) {
	completedParts := make([]types.CompletedPart, 0, len(parts))
	for _, p := range parts {
		completedParts = append(completedParts, types.CompletedPart{
			PartNumber: aws.Int32(int32(p.PartNumber)),
			ETag:       aws.String(p.ETag),
		})
	}
	res, err := a.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return nil, wrapAWSError(err)
	}
	// S3 的 Complete 响应不含对象大小（与 minio 路径的既有语义一致，Size 恒 0）
	return &CompletedObject{Key: awsString(res.Key), ETag: trimAWSEtag(awsString(res.ETag))}, nil
}

func (a *awsAdapter) AbortMultipart(ctx context.Context, bucket, key, uploadID string) error {
	if _, err := a.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}); err != nil {
		return wrapAWSError(err)
	}
	return nil
}

func (a *awsAdapter) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (*UploadsPage, error) {
	if maxUploads <= 0 {
		maxUploads = awsListPageSize
	}
	in := &s3.ListMultipartUploadsInput{Bucket: aws.String(bucket), MaxUploads: aws.Int32(int32(maxUploads))}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	if keyMarker != "" {
		in.KeyMarker = aws.String(keyMarker)
	}
	if uploadIDMarker != "" {
		in.UploadIdMarker = aws.String(uploadIDMarker)
	}
	res, err := a.client.ListMultipartUploads(ctx, in)
	if err != nil {
		return nil, wrapAWSError(err)
	}
	page := &UploadsPage{
		IsTruncated:        awsBool(res.IsTruncated),
		NextKeyMarker:      awsString(res.NextKeyMarker),
		NextUploadIDMarker: awsString(res.NextUploadIdMarker),
	}
	page.Uploads = make([]MultipartUploadInfo, 0, len(res.Uploads))
	for _, u := range res.Uploads {
		info := MultipartUploadInfo{Key: awsString(u.Key), UploadID: awsString(u.UploadId)}
		if u.Initiated != nil {
			info.Initiated = *u.Initiated
		}
		// S3 的 ListMultipartUploads 不返回已上传大小（与 minio 路径一致，保持 0）
		page.Uploads = append(page.Uploads, info)
	}
	return page, nil
}

// —— 工具函数 ——

// awsString 解引用可空字符串。
func awsString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// awsBool 解引用可空布尔（nil 视为 false）。
func awsBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

// trimAWSEtag 去掉 ETag 的引号（与 minio 路径的规整行为一致）。
func trimAWSEtag(etag string) string {
	return strings.Trim(strings.TrimSpace(etag), `"`)
}

// awsTagsOf 构造 AWS 的标签列表。
func awsTagsOf(tagMap map[string]string) []types.Tag {
	tags := make([]types.Tag, 0, len(tagMap))
	for k, v := range tagMap {
		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return tags
}

// awsTagListToMap 摊平 AWS 的标签列表。
func awsTagListToMap(tags []types.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		out[awsString(t.Key)] = awsString(t.Value)
	}
	return out
}
