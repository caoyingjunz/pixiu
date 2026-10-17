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
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/controller/extension/storage/adapter"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 对象存储碎片管理：列举与清理未完成的分片上传。

const (
	// maxIncompleteUploads 碎片单次扫描上限
	maxIncompleteUploads = 2000
)

// ListIncompleteUploads 列举未完成的分片上传（碎片）：不属任何对象也不出现在对象列表，但占用空间。
func (c *controller) ListIncompleteUploads(ctx context.Context, datasourceID int64, bucket, prefix string) (*types.StorageMultipartUploadList, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}

	items := make([]types.StorageMultipartUpload, 0, 32)
	var totalSize int64
	truncated := false
	// 与改造前 SDK 内部同构的分页列举：key-marker / upload-id-marker 续读，直到不再截断或达到上限
	keyMarker := ""
	uploadIDMarker := ""
	for {
		page, err := client.ListMultipartUploads(ctx, bucket, prefix, keyMarker, uploadIDMarker, 0)
		if err != nil {
			return nil, apierrors.NewError(fmt.Errorf("list incomplete uploads failed: %w", err), 502)
		}
		for _, info := range page.Uploads {
			if len(items) >= maxIncompleteUploads {
				truncated = true
				break
			}
			initiated := info.Initiated
			items = append(items, types.StorageMultipartUpload{
				Key:       info.Key,
				UploadID:  info.UploadID,
				Initiated: &initiated,
				Size:      info.Size,
			})
			totalSize += info.Size
		}
		if truncated || !page.IsTruncated {
			break
		}
		nextKeyMarker, nextUploadIDMarker := page.NextKeyMarker, page.NextUploadIDMarker
		// marker 未前进（或为空）即停止：防止兼容异常的服务端让循环原地打转
		if (nextKeyMarker == "" && nextUploadIDMarker == "") ||
			(nextKeyMarker == keyMarker && nextUploadIDMarker == uploadIDMarker) {
			break
		}
		keyMarker, uploadIDMarker = nextKeyMarker, nextUploadIDMarker
	}

	// 按发起时间升序，先清理最老的碎片（Initiated 由列举结果填充，恒非空）
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Initiated.Before(*items[j].Initiated)
	})

	return &types.StorageMultipartUploadList{Items: items, TotalSize: totalSize, Truncated: truncated}, nil
}

// AbortIncompleteUploads 清理指定对象（key）的全部未完成分片上传，并返回结果明细。
// 逐条失败只累计到 FailedKeys 而不整体返回 502，前端需要区分「哪些清理成功」。
func (c *controller) AbortIncompleteUploads(ctx context.Context, datasourceID int64, bucket string, keys []string) (*types.StorageAbortUploadResult, error) {
	client, _, err := c.clientFor(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	bucket, err = requireBucket(bucket)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("object keys are required"), 400)
	}

	result := &types.StorageAbortUploadResult{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if err := abortIncompleteUpload(ctx, client, bucket, key); err != nil {
			result.Failed++
			result.FailedKeys = append(result.FailedKeys, key)
			continue
		}
		result.Succeeded++
	}
	return result, nil
}

// abortIncompleteUpload 中止指定对象的全部未完成分片上传（与改造前 SDK RemoveIncompleteUpload 同构）：
// 先列举前缀为 key 的碎片、收集完全匹配 key 的全部 uploadId，再逐个中止，首个失败即返回。
func abortIncompleteUpload(ctx context.Context, client adapter.Adapter, bucket, key string) error {
	uploadIDs := make([]string, 0, 4)
	keyMarker := ""
	uploadIDMarker := ""
	for {
		page, err := client.ListMultipartUploads(ctx, bucket, key, keyMarker, uploadIDMarker, 0)
		if err != nil {
			return err
		}
		for _, upload := range page.Uploads {
			if upload.Key == key {
				uploadIDs = append(uploadIDs, upload.UploadID)
			}
		}
		if !page.IsTruncated {
			break
		}
		nextKeyMarker, nextUploadIDMarker := page.NextKeyMarker, page.NextUploadIDMarker
		// marker 未前进（或为空）即停止：防止兼容异常的服务端让循环原地打转
		if (nextKeyMarker == "" && nextUploadIDMarker == "") ||
			(nextKeyMarker == keyMarker && nextUploadIDMarker == uploadIDMarker) {
			break
		}
		keyMarker, uploadIDMarker = nextKeyMarker, nextUploadIDMarker
	}
	for _, uploadID := range uploadIDs {
		if err := client.AbortMultipart(ctx, bucket, key, uploadID); err != nil {
			return err
		}
	}
	return nil
}
