/*
Copyright 2024 The Pixiu Authors.

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

package cluster

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (cr *clusterRouter) listPodFiles(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opts    types.PodResourceMeta
		fileOpt types.PodFileOptions
		err     error
	)
	if err = httputils.ShouldBindAny(c, nil, &opts, &fileOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.authorizeClusterAccessByName(c, opts.Cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.Cluster().ListPodFiles(c, opts.Cluster, opts.Namespace, opts.Pod, fileOpt.Container, fileOpt.Path); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) downloadPodFile(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opts    types.PodResourceMeta
		fileOpt types.PodFileOptions
		err     error
	)
	if err = httputils.ShouldBindAny(c, nil, &opts, &fileOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.authorizeClusterAccessByName(c, opts.Cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.c.Cluster().DownloadPodFile(c, opts.Cluster, opts.Namespace, opts.Pod, fileOpt.Container, fileOpt.Path, c.Writer); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}

func (cr *clusterRouter) uploadPodFile(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opts    types.PodResourceMeta
		fileOpt types.PodFileOptions
		err     error
	)
	if err = httputils.ShouldBindAny(c, nil, &opts, &fileOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// 集群归属校验放在读取上传文件之前，避免未授权用户触发 multipart 解析
	if err = cr.authorizeClusterAccessByName(c, opts.Cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// Cap request body to upload limit + multipart overhead.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, types.PodFileMaxBytes+1024*1024)
	fh, err := c.FormFile("file")
	if err != nil {
		httputils.SetFailed(c, r, fmt.Errorf("file is required"))
		return
	}
	if fh.Size > types.PodFileMaxBytes {
		httputils.SetFailed(c, r, fmt.Errorf("file exceeds size limit (%d bytes)", types.PodFileMaxBytes))
		return
	}

	f, err := fh.Open()
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	defer f.Close()

	if err = cr.c.Cluster().UploadPodFile(c, opts.Cluster, opts.Namespace, opts.Pod, fileOpt.Container, fileOpt.Path, fh.Filename, f, fh.Size); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	r.Result = map[string]string{"path": fileOpt.Path, "name": fh.Filename}
	httputils.SetSuccess(c, r)
}
