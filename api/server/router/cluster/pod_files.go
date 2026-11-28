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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

const (
	PodFileDownloadRoute  = "/clusters/:cluster/namespaces/:namespace/pods/:pod/files/download"
	PodFileDownloadPath   = kubeProxyBaseURL + PodFileDownloadRoute
	PodFileDownloadCookie = "pixiu-pod-file-download"
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
	credName, err := cr.resolveCredClusterName(c, opts.Cluster)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	opts.Cluster = credName
	if r.Result, err = cr.c.Cluster().ListPodFiles(c.Request.Context(), opts.Cluster, opts.Namespace, opts.Pod, fileOpt.Container, fileOpt.Path); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) preparePodFileDownload(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		opts    types.PodResourceMeta
		fileOpt types.PodFileOptions
	)
	if err := httputils.ShouldBindAny(c, nil, &opts, &fileOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := cr.resolveCredClusterName(c, opts.Cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if strings.TrimSpace(fileOpt.Container) == "" {
		httputils.SetFailed(c, r, fmt.Errorf("container is required"))
		return
	}
	downloadPath := c.Request.URL.EscapedPath()
	// Browser downloads cannot send the Authorization header; scope the cookie to this endpoint.
	if c.GetHeader("Authorization") != "" {
		token, err := tokenutil.ExtractToken(c, false)
		if err != nil {
			httputils.SetFailed(c, r, err)
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name: PodFileDownloadCookie, Value: token, Path: downloadPath, MaxAge: 60,
			HttpOnly: true, SameSite: http.SameSiteStrictMode,
			Secure: c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"),
		})
	}
	c.Header("Cache-Control", "no-store")
	query := url.Values{"container": {fileOpt.Container}, "path": {fileOpt.Path}}
	r.Result = map[string]string{"download_url": downloadPath + "?" + query.Encode()}
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) downloadPodFile(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Accel-Buffering", "no")
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

	credName, err := cr.resolveCredClusterName(c, opts.Cluster)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	opts.Cluster = credName

	if err = cr.c.Cluster().DownloadPodFile(c.Request.Context(), opts.Cluster, opts.Namespace, opts.Pod, fileOpt.Container, fileOpt.Path, c.Writer); err != nil {
		if !c.Writer.Written() {
			httputils.SetFailed(c, r, err)
		} else {
			_ = c.Error(err)
		}
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
	credName, err := cr.resolveCredClusterName(c, opts.Cluster)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	opts.Cluster = credName

	// Allow multipart overhead in addition to the file size limit.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, types.PodFileMaxBytes+1024*1024)
	fh, err := c.FormFile("file")
	if err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			httputils.SetFailed(c, r, fmt.Errorf("file exceeds size limit (%d bytes)", types.PodFileMaxBytes))
			return
		}
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

	if err = cr.c.Cluster().UploadPodFile(c.Request.Context(), opts.Cluster, opts.Namespace, opts.Pod, fileOpt.Container, fileOpt.Path, fh.Filename, f, fh.Size); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	r.Result = map[string]string{"path": fileOpt.Path, "name": fh.Filename}
	httputils.SetSuccess(c, r)
}
