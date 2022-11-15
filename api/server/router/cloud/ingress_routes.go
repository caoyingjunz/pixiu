package cloud

import (
	"context"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/networking/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) listIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		listOptions meta.ListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Ingress(listOptions.Cloud).List(context.TODO(), listOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) createIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		createOptions meta.CreateOptions
		ingress       v1.Ingress
	)
	if err = c.ShouldBindUri(&createOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&ingress); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	ingress.Namespace = createOptions.Namespace
	if err = pixiu.CoreV1.Cloud().Ingress(createOptions.Cloud).Create(context.TODO(), &ingress); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions meta.GetOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Ingress(getOptions.Cloud).Get(context.TODO(), getOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var deleteOptions meta.DeleteOptions
	if err := c.ShouldBindUri(&deleteOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().Ingress(deleteOptions.Cloud).Delete(context.TODO(), deleteOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
