package cloud

import (
	"context"

	v1 "k8s.io/api/networking/v1"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/gin-gonic/gin"
)

func (s *cloudRouter) listIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		listOptions types.ListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Ingress(listOptions.CloudName).List(context.TODO(), listOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) createIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions types.GetOrCreateOptions
		ingress    v1.Ingress
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&ingress); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	ingress.Name = getOptions.ObjectName
	ingress.Namespace = getOptions.Namespace
	if err = pixiu.CoreV1.Cloud().Ingress(getOptions.CloudName).Create(context.TODO(), &ingress); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions types.GetOrDeleteOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Ingress(getOptions.CloudName).Get(context.TODO(), getOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var deleteOptions types.GetOrDeleteOptions
	if err := c.ShouldBindUri(&deleteOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().Ingress(deleteOptions.CloudName).Delete(context.TODO(), deleteOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
