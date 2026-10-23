package backup

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type idMeta struct {
	PlanID    int64 `uri:"planId"`
	BackupID  int64 `uri:"backupId"`
	RestoreID int64 `uri:"restoreId"`
	ClusterID int64 `uri:"clusterId"`
}

func (r *backupRouter) testStorage(c *gin.Context) {
	result := httputils.NewResponse()
	var req types.TestBackupStorageRequest
	err := c.ShouldBindJSON(&req)
	if err == nil {
		err = r.c.Backup().TestStorage(c, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	result.Result = map[string]string{"message": "连接成功"}
	httputils.SetSuccess(c, result)
}

func (r *backupRouter) getVeleroStatus(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.Backup().GetVeleroStatus(c, meta.ClusterID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) createPlan(c *gin.Context) {
	result := httputils.NewResponse()
	var req types.CreateBackupPlanRequest
	var err error
	if err = httputils.BindCreateRequest(c, &req); err == nil {
		result.Result, err = r.c.Backup().CreatePlan(c, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) updatePlan(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	var req types.UpdateBackupPlanRequest
	err := httputils.ShouldBindAny(c, &req, &meta, nil)
	if err == nil {
		err = r.c.Backup().UpdatePlan(c, meta.PlanID, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) deletePlan(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		err = r.c.Backup().DeletePlan(c, meta.PlanID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) listPlans(c *gin.Context) {
	result := httputils.NewResponse()
	value, err := r.c.Backup().ListPlans(c)
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	result.Result = value
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) getPlan(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.Backup().GetPlan(c, meta.PlanID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) runPlan(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	var req types.RunBackupPlanRequest
	err := httputils.ShouldBindAny(c, &req, &meta, nil)
	if err == nil {
		result.Result, err = r.c.Backup().RunPlan(c, meta.PlanID, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) listBackups(c *gin.Context) {
	result := httputils.NewResponse()
	value, err := r.c.Backup().ListBackups(c)
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	result.Result = value
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) getBackup(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.Backup().GetBackup(c, meta.BackupID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) deleteBackup(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		err = r.c.Backup().DeleteBackup(c, meta.BackupID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) createRestore(c *gin.Context) {
	result := httputils.NewResponse()
	var req types.CreateRestoreRequest
	var err error
	if err = c.ShouldBindJSON(&req); err == nil {
		result.Result, err = r.c.Backup().CreateRestore(c, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) updateRestore(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	var req types.UpdateRestoreRequest
	err := httputils.ShouldBindAny(c, &req, &meta, nil)
	if err == nil {
		err = r.c.Backup().UpdateRestore(c, meta.RestoreID, &req)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) deleteRestore(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		err = r.c.Backup().DeleteRestore(c, meta.RestoreID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) listRestores(c *gin.Context) {
	result := httputils.NewResponse()
	value, err := r.c.Backup().ListRestores(c)
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	result.Result = value
	httputils.SetSuccess(c, result)
}
func (r *backupRouter) getRestore(c *gin.Context) {
	result := httputils.NewResponse()
	var meta idMeta
	err := c.ShouldBindUri(&meta)
	if err == nil {
		result.Result, err = r.c.Backup().GetRestore(c, meta.RestoreID)
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	httputils.SetSuccess(c, result)
}
