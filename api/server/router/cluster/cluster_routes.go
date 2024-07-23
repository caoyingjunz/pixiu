/*
Copyright 2021 The Pixiu Authors.

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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"time"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
)

type IdMeta struct {
	ClusterId int64 `uri:"clusterId" binding:"required"`
}

// CreateCluster godoc
//
//	@Summary      Create a cluster
//	@Description  Create by a json cluster
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        cluster  body      types.Cluster  true  "Create cluster"
//	@Success      200      {object}  httputils.Response
//	@Failure      400      {object}  httputils.Response
//	@Failure      404      {object}  httputils.Response
//	@Failure      500      {object}  httputils.Response
//	@Router       /pixiu/clusters/ [post]
//	@Security     Bearer
func (cr *clusterRouter) createCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var req types.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := cr.c.Cluster().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// UpdateCluster godoc
//
//	@Summary      Update an cluster
//	@Description  Update by json cluster
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int            true  "Cluster ID"
//	@Param        cluster    body      types.Cluster  true  "Update cluster"
//	@Success      200        {object}  httputils.Response
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId} [put]
//	@Security     Bearer
func (cr *clusterRouter) updateCluster(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	var req types.UpdateClusterRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.c.Cluster().Update(c, idMeta.ClusterId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// DeleteCluster godoc
//
//	@Summary      Delete cluster by clusterId
//	@Description  Delete by cloud cluster ID
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int  true  "Cluster ID"
//	@Success      200        {object}  httputils.Response
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId} [delete]
//	@Security     Bearer
func (cr *clusterRouter) deleteCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.c.Cluster().Delete(c, idMeta.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// GetCluster godoc
//
//	@Summary      Get Cluster by clusterId
//	@Description  Get by cloud cluster ID
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int  true  "Cluster ID"
//	@Success      200        {object}  httputils.Response{result=types.Cluster}
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId} [get]
//	@Security     Bearer
func (cr *clusterRouter) getCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Get(c, idMeta.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// ListClusters godoc
//
//	@Summary      List clusters
//	@Description  List clusters
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Success      200  {array}   httputils.Response{result=[]types.Cluster}
//	@Failure      400  {object}  httputils.Response
//	@Failure      404  {object}  httputils.Response
//	@Failure      500  {object}  httputils.Response
//	@Router       /pixiu/clusters [get]
//	@Security     Bearer
func (cr *clusterRouter) listClusters(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = cr.c.Cluster().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// PingCluster godoc
//
//	@Summary      Ping cluster
//	@Description  Do ping
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int  true  "Cluster ID"
//	@Success      200        {array}   httputils.Response
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId}/ping [get]
//	@Security     Bearer
func (cr *clusterRouter) pingCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		cluster types.Cluster
		err     error
	)
	if err = c.ShouldBindJSON(&cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.Cluster().Ping(c, cluster.KubeConfig); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) protectCluster(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		idMeta IdMeta
		req    types.ProtectClusterRequest
		err    error
	)
	if err = httputils.ShouldBindAny(c, &req, &idMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.Cluster().Protect(c, idMeta.ClusterId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) aggregateEvents(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		optMeta struct {
			Cluster   string `uri:"cluster" binding:"required"`
			Namespace string `uri:"namespace" binding:"required"`
			Name      string `uri:"name" binding:"required"`
			Kind      string `uri:"kind" binding:"required"`
		}
		err error
	)

	if err = c.ShouldBindUri(&optMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.Cluster().AggregateEvents(c, optMeta.Cluster, optMeta.Namespace, optMeta.Name, optMeta.Kind); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getEventList(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		opts struct {
			Cluster string `uri:"cluster" binding:"required"`
		}
		eventOpt types.EventOptions
		err      error
	)
	if err = httputils.ShouldBindAny(c, nil, &opts, &eventOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.Cluster().GetEventList(c, opts.Cluster, eventOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) watchPodLog(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		opts struct {
			Cluster   string `uri:"cluster" binding:"required"`
			Namespace string `uri:"namespace" binding:"required"`
			Name      string `uri:"name" binding:"required"` //pod name
		}
		logOpt types.PodLogOptions
		err    error
	)
	if err = httputils.ShouldBindAny(c, nil, &opts, &logOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	req := cr.c.Cluster().WatchPodLog(c, opts.Cluster, opts.Namespace, opts.Name, logOpt.Container, logOpt.TailLines)
	withTimeout, cancelFunc := context.WithTimeout(c, time.Minute*10)
	defer cancelFunc()

	reader, err := req.Stream(withTimeout)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	defer reader.Close()

	flush, _ := c.Writer.(http.Flusher)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	for {
		select {
		case <-c.Request.Context().Done():
			return
		default:
			buf := make([]byte, 1024)  // 定义缓冲区大小为 1024 字节
			n, err := reader.Read(buf) // 读取数据到缓冲区
			if err != nil {
				if err != io.EOF {
					klog.Errorf("read error: %v", err)
					return
				}
				// 这里是 EOF，没有更多的数据可读
			}

			// 查找缓冲区中的第一个换行符
			newlineIndex := bytes.IndexByte(buf[:n], '\n')
			for newlineIndex != -1 {
				// 将缓冲区中从开始到换行符之前的数据写入
				if newlineIndex > 0 {
					klog.Errorf("-2-------------: %v", string(buf[:newlineIndex]))
					if err := json.NewEncoder(c.Writer).Encode(string(buf[:newlineIndex-2])); err != nil {
						klog.Errorf("failed to encode: %v", err)
						return
					}
					flush.Flush()
				}

				// 如果换行符之后还有数据，继续处理剩余的数据
				if newlineIndex < n-1 {
					// 移动未处理的数据到缓冲区的开始位置
					copy(buf, buf[newlineIndex+1:])
					n = n - newlineIndex - 1 // 更新剩余数据的长度
					newlineIndex = bytes.IndexByte(buf[:n], '\n')
				} else {
					// 没有更多的数据了，退出循环
					newlineIndex = -1
				}
			}

			// 如果没有找到换行符，检查缓冲区末尾是否有不完整的行
			if newlineIndex == -1 && n > 0 {
				klog.Errorf("-3-------------: %v", string(buf[:n]))
				if err := json.NewEncoder(c.Writer).Encode(string(buf[:n])); err != nil {
					klog.Errorf("failed to encode: %v", err)
					return
				}
				flush.Flush()
			}
		}
	}
	//for {
	//	select {
	//	case <-c.Request.Context().Done():
	//		return
	//	default:
	//		buf := make([]byte, 1024)
	//		n, err := reader.Read(buf)
	//		if err != nil && err != io.EOF {
	//			return
	//		}
	//		klog.Errorf("-2-------------", string(buf[0:n]))
	//		if err := json.NewEncoder(c.Writer).Encode(string(buf[0:n])); err != nil {
	//			klog.Errorf("failed to encode : %v", err)
	//			return
	//		}
	//		flush.Flush()
	//	}
	//}

	//conn, err := client.WebsocketUpgrade.Upgrade(c.Writer, c.Request, nil)
	//if err != nil {
	//	httputils.SetFailed(c, r, err)
	//	return
	//}
	//defer conn.Close()
	//
	//client.WebsocketUpgrade.Subprotocols = []string{c.Request.Header.Get("Sec-WebSocket-Protocol")}
	//wsClient := client.NewWsClient(conn, opts.Cluster, "log")
	//for {
	//	buf := make([]byte, 1024)
	//	n, err := reader.Read(buf)
	//	if err != nil && err != io.EOF {
	//		break
	//	}
	//
	//	err = wsClient.Conn.WriteMessage(websocket.TextMessage, buf[0:n])
	//	if err != nil {
	//		break
	//	}
	//}

}
