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
		err  error
		opts meta.ListOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Ingress(opts.Cloud).List(context.TODO(), opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) createIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err     error
		opts    meta.CreateOptions
		ingress v1.Ingress
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&ingress); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	ingress.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().Ingress(opts.Cloud).Create(context.TODO(), &ingress); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.GetOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Ingress(opts.Cloud).Get(context.TODO(), opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteIngress(c *gin.Context) {
	r := httputils.NewResponse()
	var opts meta.DeleteOptions
	if err := c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().Ingress(opts.Cloud).Delete(context.TODO(), opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
