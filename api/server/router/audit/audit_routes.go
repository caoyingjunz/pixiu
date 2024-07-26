package audit

import (
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/gin-gonic/gin"
)

type AuditMeta struct {
	AuditId int64 `uri:"auditId" binding:"required"`
}

func (a *auditRouter) getAudit(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt AuditMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = a.c.Tenant().Get(c, opt.AuditId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *auditRouter) deleteAudit(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt AuditMeta
		err error
	)
	if err = c.ShouldBindUri(&opt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = a.c.Tenant().Delete(c, opt.AuditId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (a *auditRouter) listAudits(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = a.c.Tenant().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
