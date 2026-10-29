package backup

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const backupURL = "/pixiu/backup"

type backupRouter struct{ c controller.PixiuInterface }

func NewRouter(o *options.Options) {
	router := &backupRouter{c: o.Controller}
	router.initRoutes(o.HttpEngine)
}

func (r *backupRouter) initRoutes(engine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "集群备份",
		BaseURL: backupURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/clusters/:clusterId/velero-status", Handler: r.getVeleroStatus, Description: "获取 Velero 安装状态"},
			{Method: "GET", RelativePath: "/plans", Handler: r.listPlans, Description: "备份计划列表"},
			{Method: "GET", RelativePath: "/plans/:planId", Handler: r.getPlan, Description: "备份计划详情"},
			{Method: "POST", RelativePath: "/plans", Handler: r.createPlan, Description: "创建备份计划"},
			{Method: "PUT", RelativePath: "/plans/:planId", Handler: r.updatePlan, Description: "更新备份计划"},
			{Method: "DELETE", RelativePath: "/plans/:planId", Handler: r.deletePlan, Description: "删除备份计划"},
			{Method: "POST", RelativePath: "/plans/:planId/run", Handler: r.runPlan, Description: "执行备份计划"},
			{Method: "GET", RelativePath: "/backups", Handler: r.listBackups, Description: "备份记录列表"},
			{Method: "GET", RelativePath: "/backups/:backupId", Handler: r.getBackup, Description: "备份详情"},
			{Method: "DELETE", RelativePath: "/backups/:backupId", Handler: r.deleteBackup, Description: "删除备份"},
			{Method: "GET", RelativePath: "/restores", Handler: r.listRestores, Description: "恢复任务列表"},
			{Method: "POST", RelativePath: "/restores", Handler: r.createRestore, Description: "创建恢复任务"},
			{Method: "POST", RelativePath: "/restores/:restoreId/run", Handler: r.runRestore, Description: "执行恢复任务"},
			{Method: "PUT", RelativePath: "/restores/:restoreId", Handler: r.updateRestore, Description: "更新恢复任务"},
			{Method: "DELETE", RelativePath: "/restores/:restoreId", Handler: r.deleteRestore, Description: "删除恢复任务"},
			{Method: "GET", RelativePath: "/restores/:restoreId", Handler: r.getRestore, Description: "恢复任务详情"},
		},
	}
	group.Register(engine.Group(backupURL), r.c.APIResource())
}
