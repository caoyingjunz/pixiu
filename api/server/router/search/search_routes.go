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

package search

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// searchRequest 全局搜索入参。
// q 与 keyword 等价：q 供前端搜索框直接使用，keyword 为显式命名，二者都传时以 q 为准。
// cluster 与 clusters 等价，clusters 支持逗号分隔的多个集群（「全部集群」场景），都传时以 clusters 为准。
type searchRequest struct {
	Cluster       string `form:"cluster"`  // 集群名（非 id），与代理路径一致
	Clusters      string `form:"clusters"` // 逗号分隔的集群名，如 dev,prod
	Q             string `form:"q"`
	Keyword       string `form:"keyword"`
	Namespace     string `form:"namespace"`     // 为空或 all_namespaces 表示全部命名空间
	Kinds         string `form:"kinds"`         // 逗号分隔，如 pods,deployments
	LabelSelector string `form:"labelSelector"` // 标签子串匹配，如 app=nginx
	Limit         int    `form:"limit"`         // 单个类型的返回上限
}

func (s *searchRouter) search(c *gin.Context) {
	r := httputils.NewResponse()

	var req searchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	opt := types.SearchOptions{
		Keyword:       firstNonEmpty(req.Q, req.Keyword),
		Namespace:     req.Namespace,
		Kinds:         splitList(req.Kinds),
		LabelSelector: req.LabelSelector,
		Limit:         req.Limit,
	}
	clusters := splitList(firstNonEmpty(req.Clusters, req.Cluster))
	if r.Result, err = s.c.Search().Search(c, user, clusters, opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listSearchKinds 返回可搜索的资源类型清单，供前端渲染「资源类型」下拉。
func (s *searchRouter) listSearchKinds(c *gin.Context) {
	r := httputils.NewResponse()
	r.Result = s.c.Search().Kinds()
	httputils.SetSuccess(c, r)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// splitList 解析逗号分隔的列表参数（集群名、资源类型共用）。
func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
