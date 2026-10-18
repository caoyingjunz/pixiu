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

package storage

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	DatasourceID int64 `uri:"datasourceId"`
}

type pingStorageRequest struct {
	DatasourceID int64 `json:"datasource_id"`
	types.StorageSourceConfig
}

type bucketMeta struct {
	DatasourceID int64  `uri:"datasourceId"`
	Bucket       string `uri:"bucket" binding:"required"`
}

type createBucketOptions struct {
	Name string `json:"name" binding:"required"`
}

type objectPageOptions struct {
	Bucket    string `form:"bucket" binding:"required"`
	Prefix    string `form:"prefix"`
	Token     string `form:"token"`
	PageSize  int    `form:"page_size"`
	Keyword   string `form:"keyword"`
	Recursive bool   `form:"recursive"`
}

type uploadOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Key    string `form:"key" binding:"required"`
}

type deleteObjectsOptions struct {
	Bucket string   `json:"bucket" binding:"required"`
	Keys   []string `json:"keys" binding:"required"`
}

type renameObjectOptions struct {
	Bucket string `json:"bucket" binding:"required"`
	Src    string `json:"src" binding:"required"`
	Dst    string `json:"dst" binding:"required"`
}

type presignOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Key    string `form:"key" binding:"required"`
	Expiry int    `form:"expiry"`
}

type objectContentOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Key    string `form:"key" binding:"required"`
}

type versionListOptions struct {
	Bucket  string `form:"bucket" binding:"required"`
	Prefix  string `form:"prefix"`
	Recycle bool   `form:"recycle"`
	Limit   int    `form:"limit"`
}

type bucketOnlyOptions struct {
	Bucket string `form:"bucket" binding:"required"`
}

type versionTargetOptions struct {
	Bucket    string `json:"bucket" binding:"required"`
	Key       string `json:"key" binding:"required"`
	VersionID string `json:"version_id" binding:"required"`
}

type incompleteUploadOptions struct {
	Bucket string `form:"bucket" binding:"required"`
	Prefix string `form:"prefix"`
}

type abortUploadOptions struct {
	Bucket string   `json:"bucket" binding:"required"`
	Keys   []string `json:"keys" binding:"required"`
}

type objectTagsOptions struct {
	Bucket    string `form:"bucket" binding:"required"`
	Key       string `form:"key" binding:"required"`
	VersionID string `form:"version_id"`
}

type objectTagsUpdateOptions struct {
	Bucket    string                   `json:"bucket" binding:"required"`
	Key       string                   `json:"key" binding:"required"`
	VersionID string                   `json:"version_id"`
	Tags      []types.StorageObjectTag `json:"tags"`
}

type objectDetailOptions struct {
	Bucket    string `form:"bucket" binding:"required"`
	Key       string `form:"key" binding:"required"`
	VersionID string `form:"version_id"`
}

type objectMetaUpdateOptions struct {
	Bucket string                        `json:"bucket" binding:"required"`
	Key    string                        `json:"key" binding:"required"`
	Meta   types.StorageObjectMetaUpdate `json:"meta"`
}

// multipartPartOptions 分片上传：分片本体走请求体，其余信息走 query，避免 multipart/form-data 开销
type multipartPartOptions struct {
	Bucket     string `form:"bucket" binding:"required"`
	Key        string `form:"key" binding:"required"`
	UploadID   string `form:"upload_id" binding:"required"`
	PartNumber int    `form:"part_number" binding:"required"`
}

type multipartPartsOptions struct {
	Bucket   string `form:"bucket" binding:"required"`
	Key      string `form:"key" binding:"required"`
	UploadID string `form:"upload_id" binding:"required"`
}

type bucketPolicyOptions struct {
	Policy string `json:"policy"`
}

type transferObjectsOptions struct {
	Bucket    string   `json:"bucket" binding:"required"`
	Keys      []string `json:"keys" binding:"required"`
	DstBucket string   `json:"dst_bucket" binding:"required"`
	DstPrefix string   `json:"dst_prefix"`
	Move      bool     `json:"move"`
}

// batchDownloadOptions 批量/目录打包下载：keys 用重复参数传递（keys=a&keys=b）
type batchDownloadOptions struct {
	Bucket string   `form:"bucket" binding:"required"`
	Keys   []string `form:"keys" binding:"required"`
}

func (r *router) pingAdhoc(c *gin.Context) {
	resp := httputils.NewResponse()
	var req pingStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().PingAdhoc(c, req.DatasourceID, &req.StorageSourceConfig)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) ping(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().Ping(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

// providers 下发各厂商的控制面能力，前端据此决定功能入口是否展示。
func (r *router) providers(c *gin.Context) {
	resp := httputils.NewResponse()
	resp.Result = r.c.Extension().ObjectStorage().ListProviders(c)
	httputils.SetSuccess(c, resp)
}

func (r *router) buckets(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().ListBuckets(c, m.DatasourceID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) createBucket(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts createBucketOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().CreateBucket(c, m.DatasourceID, opts.Name); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteBucket(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().DeleteBucket(c, m.DatasourceID, m.Bucket); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getBucketConfig(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetBucketConfig(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) getBucketStats(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetBucketStats(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setBucketConfig(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var cfg types.StorageBucketConfigUpdate
	if err := httputils.ShouldBindAny(c, &cfg, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().SetBucketConfig(c, m.DatasourceID, m.Bucket, &cfg); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) emptyBucket(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().EmptyBucket(c, m.DatasourceID, m.Bucket); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getLifecycle(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetLifecycle(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setLifecycle(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var rules []types.StorageLifecycleRule
	if err := httputils.ShouldBindAny(c, &rules, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().SetLifecycle(c, m.DatasourceID, m.Bucket, rules); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getCors(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetCors(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setCors(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var rules []types.StorageCorsRule
	if err := httputils.ShouldBindAny(c, &rules, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().SetCors(c, m.DatasourceID, m.Bucket, rules); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) objects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectPageOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().ListObjectsPage(c, m.DatasourceID, opts.Bucket, opts.Prefix, opts.Token, opts.PageSize, opts.Keyword, opts.Recursive)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) uploadObject(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts uploadOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().PutObject(c, m.DatasourceID, opts.Bucket, opts.Key, c.Request.Body, c.Request.ContentLength, c.ContentType()); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteObjects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts deleteObjectsOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().DeleteObjects(c, m.DatasourceID, opts.Bucket, opts.Keys); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) renameObject(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts renameObjectOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().RenameObject(c, m.DatasourceID, opts.Bucket, opts.Src, opts.Dst); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) presignDownload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts presignOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().PresignedGetObject(c, m.DatasourceID, opts.Bucket, opts.Key, opts.Expiry)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// expires 是夹取后的生效值，不是请求里的 expiry
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

// objectContent streams one object through Pixiu for in-app preview.
// It deliberately keeps the browser on the Pixiu origin so Bucket CORS rules are not required.
func (r *router) objectContent(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectContentOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	reader, size, contentType, err := r.c.Extension().ObjectStorage().OpenObject(c, m.DatasourceID, opts.Bucket, opts.Key)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	defer reader.Close()
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(200, size, contentType, reader, nil)
}

func (r *router) objectVersions(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts versionListOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().ListObjectVersions(c, m.DatasourceID, opts.Bucket, opts.Prefix, opts.Recycle, opts.Limit)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) bucketVersioning(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts bucketOnlyOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	state, err := r.c.Extension().ObjectStorage().GetBucketVersioningState(c, m.DatasourceID, opts.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// 三态返回：unknown 表示状态查询失败，前端据此提示「状态获取失败」而不是断言「未开启版本控制」
	resp.Result = map[string]interface{}{
		"versioning_state":   state,
		"versioning_enabled": state == "enabled",
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) restoreObjectVersion(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts versionTargetOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().RestoreObjectVersion(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteObjectVersion(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts versionTargetOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().DeleteObjectVersion(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) incompleteUploads(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts incompleteUploadOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().ListIncompleteUploads(c, m.DatasourceID, opts.Bucket, opts.Prefix)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) abortIncompleteUploads(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts abortUploadOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().AbortIncompleteUploads(c, m.DatasourceID, opts.Bucket, opts.Keys)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) objectTags(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectTagsOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetObjectTags(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setObjectTags(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectTagsUpdateOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().SetObjectTags(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID, opts.Tags); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) objectDetail(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectDetailOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetObjectDetail(c, m.DatasourceID, opts.Bucket, opts.Key, opts.VersionID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) updateObjectMeta(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts objectMetaUpdateOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().UpdateObjectMeta(c, m.DatasourceID, opts.Bucket, opts.Key, &opts.Meta); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

// ---------- 大文件分片上传 ----------

func (r *router) initMultipartUpload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageMultipartInit
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().InitMultipartUpload(c, m.DatasourceID, &opts)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) uploadMultipartPart(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts multipartPartOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	// 分片本体直通请求体，长度取 Content-Length（浏览器上传 Blob 时会带上）
	result, err := r.c.Extension().ObjectStorage().UploadMultipartPart(
		c, m.DatasourceID, opts.Bucket, opts.Key, opts.UploadID, opts.PartNumber,
		c.Request.Body, c.Request.ContentLength,
	)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) multipartParts(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts multipartPartsOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().ListMultipartParts(c, m.DatasourceID, opts.Bucket, opts.Key, opts.UploadID)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) completeMultipartUpload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageMultipartComplete
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().CompleteMultipartUpload(c, m.DatasourceID, &opts)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) abortMultipartUpload(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts types.StorageMultipartTarget
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().AbortMultipartUpload(c, m.DatasourceID, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

// ---------- Bucket 授权策略 ----------

func (r *router) getBucketPolicy(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	if err := httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().GetBucketPolicy(c, m.DatasourceID, m.Bucket)
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

func (r *router) setBucketPolicy(c *gin.Context) {
	resp := httputils.NewResponse()
	var m bucketMeta
	var opts bucketPolicyOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Extension().ObjectStorage().SetBucketPolicy(c, m.DatasourceID, m.Bucket, opts.Policy); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

// ---------- 跨桶复制/移动与批量打包下载 ----------

func (r *router) transferObjects(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts transferObjectsOptions
	if err := httputils.ShouldBindAny(c, &opts, &m, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	result, err := r.c.Extension().ObjectStorage().TransferObjects(c, m.DatasourceID, &types.StorageTransferRequest{
		Bucket:    opts.Bucket,
		Keys:      opts.Keys,
		DstBucket: opts.DstBucket,
		DstPrefix: opts.DstPrefix,
		Move:      opts.Move,
	})
	if err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	resp.Result = result
	httputils.SetSuccess(c, resp)
}

// downloadObjectsArchive 批量/目录打包下载：响应是 zip 二进制流而非项目统一 JSON 响应。
// controller 只有在「校验与展开全部成功」之后才写出第一个字节，因此 written=false 时仍能返回 JSON 错误。
func (r *router) downloadObjectsArchive(c *gin.Context) {
	resp := httputils.NewResponse()
	var m meta
	var opts batchDownloadOptions
	if err := httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	filename := archiveFileName(opts.Bucket, opts.Keys)
	c.Header("Content-Type", "application/zip")
	c.Header("Cache-Control", "no-store")
	writeArchiveDisposition(c, filename)

	written, err := r.c.Extension().ObjectStorage().DownloadObjectsZip(c, m.DatasourceID, opts.Bucket, opts.Keys, c.Writer)
	if err != nil {
		klog.Errorf("download objects archive failed: %v", err)
		if !written {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Header("Content-Disposition", "")
			httputils.SetFailed(c, resp, err)
		}
	}
}

// archiveFileName 单目录下载时用目录名，多选时用 Bucket 名
func archiveFileName(bucket string, keys []string) string {
	name := bucket + "-objects"
	if len(keys) == 1 {
		trimmed := strings.TrimSuffix(strings.TrimSpace(keys[0]), "/")
		if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		if trimmed != "" {
			name = trimmed
		}
	}
	return name + ".zip"
}

// 文件名里除字母数字与 -_. 之外的字符（含中文）不能直接放进 header，需转义
var archiveNameUnsafe = regexp.MustCompile(`[^\w.-]+`)

// writeArchiveDisposition 同时给出 ASCII 回退名与 RFC 5987 编码名，兼容中文目录名
func writeArchiveDisposition(c *gin.Context, filename string) {
	ascii := archiveNameUnsafe.ReplaceAllString(filename, "_")
	if strings.Trim(ascii, "_.-") == "" {
		ascii = "objects.zip"
	}
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(filename)))
}
