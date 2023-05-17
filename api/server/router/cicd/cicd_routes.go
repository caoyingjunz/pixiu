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

package cicd

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/types"
	"github.com/caoyingjunz/pixiu/pkg/pixiu"
)

// @Summary      运行流水线
// @Description  运行流水线
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /cicd/jobs/run [post]
func (s *cicdRouter) runJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	err := pixiu.CoreV1.Cicd().RunJob(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 创建任务
// @Summary      创建流水线
// @Description  创建流水线
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs [post]
func (s *cicdRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err := pixiu.CoreV1.Cicd().CreateJob(c, cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 复制流水线
// @Summary      复制流水线
// @Description  复制流水线
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/copy [post]
func (s *cicdRouter) copyJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().CopyJob(c, cicd.OldName, cicd.NewName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 重命名任务
// @Summary      重命名流水线
// @Description  重命名流水线
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/rename [post]
func (s *cicdRouter) renameJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cicd().RenameJob(c, cicd.OldName, cicd.NewName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 获取所有的流水线
// @Summary      获取所有的流水线
// @Description  获取所有的流水线
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs [get]
func (s *cicdRouter) getAllJobs(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllJobs(c)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 获取所有的视图
// @Summary      获取所有的视图
// @Description  获取所有的视图
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/view [get]
func (s *cicdRouter) getAllViews(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllViews(c)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 获取最后一个失败的构建节点
// @Summary      获取最后一个失败的构建节点
// @Description  获取最后一个失败的构建节点
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/failed [post]
func (s *cicdRouter) getLastFailedBuild(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetLastFailedBuild(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 获取最后一个构建成功的节点
// @Summary      获取最后一个构建成功的节点
// @Description  获取最后一个构建成功的节点
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/success [post]
func (s *cicdRouter) getLastSuccessfulBuild(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetLastSuccessfulBuild(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 根据名称获取流水线细节
// @Summary      根据名称获取流水线细节
// @Description  根据名称获取流水线细节
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/details/{name} [get]
func (s *cicdRouter) details(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	r.Result = pixiu.CoreV1.Cicd().Details(c, name)
	httputils.SetSuccess(c, r)
}

// 获取所有的节点
// @Summary      获取所有的节点
// @Description  获取所有的节点
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /nodes [get]
func (s *cicdRouter) getAllNodes(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllNodes(c)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 根据名称删除流水线
// @Summary      根据名称删除流水线
// @Description  根据名称删除流水线
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        name 任务名称
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/{name} [delete]
func (s *cicdRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	if err := pixiu.CoreV1.Cicd().DeleteJob(c, name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 根据名称删除
// @Summary      根据名称删除
// @Description  根据名称删除
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/view/:name/:viewname [delete]
func (s *cicdRouter) deleteViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	parm := map[string]interface{}{"name": c.Param("name"), "viewname": c.Param("viewname")}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().DeleteViewJob(c, parm["name"].(string), parm["viewname"].(string))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 根据名称删除节点
// @Summary      根据名称删除节点
// @Description  根据名称删除节点
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/nodes/{name} [delete]
func (s *cicdRouter) deleteNode(c *gin.Context) {
	r := httputils.NewResponse()
	nodename := c.Param("nodename")
	if err := pixiu.CoreV1.Cicd().DeleteNode(c, nodename); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 创建视图任务
// @Summary      创建视图任务
// @Description  创建视图任务
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/view [post]
func (s *cicdRouter) addViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cicd().AddViewJob(c, cicd.ViewName, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// 安全重启jenkins
// @Summary      安全重启jenkins
// @Description  安全重启jenkins
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/restart [post]
func (s *cicdRouter) restart(c *gin.Context) {
	r := httputils.NewResponse()
	if err := pixiu.CoreV1.Cicd().Restart(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 禁止任务
// @Summary      禁止任务
// @Description  禁止任务
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/disable [post]
func (s *cicdRouter) disable(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().Disable(c, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 开启任务
// @Summary      开启任务
// @Description  开启任务
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/enable [post]
func (s *cicdRouter) enable(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().Enable(c, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 停止任务
// @Summary      停止任务
// @Description  停止任务
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/stop [post]
func (s *cicdRouter) stop(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().Stop(c, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 配置任务
// @Summary      配置任务
// @Description  配置任务
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/config [post]
func (s *cicdRouter) config(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().Config(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 更新流水线配置
// @Summary      更新流水线配置
// @Description  更新流水线配置
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/updateconfig [post]
func (s *cicdRouter) updateConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cicd().UpdateConfig(c, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// 获取流水线构建历史
// @Summary      获取流水线构建历史
// @Description  获取流水线构建历史
// @Tags         cicd
// @Accept       json
// @Produce      json
// @Param        null
// @Success      200  {object}  httputils.SetSuccess
// @Failure      400  {object}  httputils.SetFailed
// @Router       /cicd/jobs/history [post]
func (s *cicdRouter) history(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().History(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
