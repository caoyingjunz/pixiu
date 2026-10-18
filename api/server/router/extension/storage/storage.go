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
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

// router 是对象存储子模块的路由持有者，复用 extension 控制器入口。
type router struct{ c controller.PixiuInterface }

// RegisterStorage 将对象存储子模块路由注册到 extension 父路由组下，
// 完整路径为 /pixiu/extension/storage/...
func RegisterStorage(o *options.Options, group *apiregistry.Group) {
	r := &router{c: o.Controller}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/ping", Handler: r.pingAdhoc, Description: "对象存储临时连接测试"},
		// 前端元数据：厂商控制面能力，不建立连接
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/providers", Handler: r.providers, Description: "对象存储厂商控制面能力"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/ping", Handler: r.ping, Description: "对象存储连接测试"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets", Handler: r.listBuckets, Description: "对象存储 Bucket 列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/buckets", Handler: r.createBucket, Description: "创建对象存储 Bucket"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/storage/:datasourceId/buckets/:bucket", Handler: r.deleteBucket, Description: "删除对象存储 Bucket"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/stats", Handler: r.getBucketStats, Description: "获取对象存储 Bucket 统计"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/config", Handler: r.getBucketConfig, Description: "获取对象存储 Bucket 配置"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/config", Handler: r.setBucketConfig, Description: "设置对象存储 Bucket 配置"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/buckets/:bucket/empty", Handler: r.emptyBucket, Description: "清空对象存储 Bucket"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/lifecycle", Handler: r.getLifecycle, Description: "获取对象存储 Bucket 生命周期规则"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/lifecycle", Handler: r.setLifecycle, Description: "设置对象存储 Bucket 生命周期规则"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/cors", Handler: r.getCors, Description: "获取对象存储 Bucket 跨域规则"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/cors", Handler: r.setCors, Description: "设置对象存储 Bucket 跨域规则"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/objects", Handler: r.listObjects, Description: "对象存储对象列表"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/objects", Handler: r.uploadObject, Description: "上传对象存储对象"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/objects/delete", Handler: r.deleteObjects, Description: "删除对象存储对象"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/objects/rename", Handler: r.renameObject, Description: "重命名对象存储对象"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/objects/content", Handler: r.objectContent, Description: "读取对象存储对象内容"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/presign-download", Handler: r.presignDownload, Description: "生成对象存储下载地址"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/object-versions", Handler: r.listObjectVersions, Description: "对象存储历史版本/回收站列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/bucket-versioning", Handler: r.bucketVersioning, Description: "查询对象存储 Bucket 版本控制状态"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/object-versions/restore", Handler: r.restoreObjectVersion, Description: "恢复对象存储历史版本"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/object-versions/delete", Handler: r.deleteObjectVersion, Description: "彻底删除对象存储历史版本"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/incomplete-uploads", Handler: r.incompleteUploads, Description: "对象存储未完成分片上传（碎片）列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/incomplete-uploads/abort", Handler: r.abortIncompleteUploads, Description: "清理对象存储未完成分片上传"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/object-tags", Handler: r.objectTags, Description: "获取对象存储对象标签"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/object-tags", Handler: r.setObjectTags, Description: "设置对象存储对象标签"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/object-detail", Handler: r.objectDetail, Description: "获取对象存储对象详情与元数据"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/object-meta", Handler: r.updateObjectMeta, Description: "更新对象存储对象元数据"},
		// 大文件分片上传：单次 PUT 受反向代理请求体限制，大文件必须走分片链路
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/multipart-uploads", Handler: r.initMultipartUpload, Description: "初始化对象存储分片上传"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/multipart-uploads/part", Handler: r.uploadMultipartPart, Description: "上传对象存储分片"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/multipart-uploads/parts", Handler: r.multipartParts, Description: "列举对象存储已上传分片"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/multipart-uploads/complete", Handler: r.completeMultipartUpload, Description: "完成对象存储分片上传"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/multipart-uploads/abort", Handler: r.abortMultipartUpload, Description: "中止对象存储分片上传"},
		// Bucket 授权策略（完整 JSON，与 Bucket 配置的三档 ACL 写的是同一份数据）
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/buckets/:bucket/policy", Handler: r.getBucketPolicy, Description: "获取对象存储 Bucket 授权策略"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/storage/:datasourceId/buckets/:bucket/policy", Handler: r.setBucketPolicy, Description: "设置对象存储 Bucket 授权策略"},
		// 跨桶复制/移动与批量打包下载
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/storage/:datasourceId/objects/transfer", Handler: r.transferObjects, Description: "对象存储对象跨桶复制/移动"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/storage/:datasourceId/objects/archive", Handler: r.downloadObjectsArchive, Description: "对象存储批量打包下载"},
	)
}
