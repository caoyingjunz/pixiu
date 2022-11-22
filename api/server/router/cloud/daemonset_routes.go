package cloud

import (
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/apps/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err       error
		opts      meta.CreateOptions
		daemonset v1.DaemonSet
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&daemonset); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	daemonset.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().DaemonSets(opts.Cloud).Create(c, &daemonset); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err       error
		opts      meta.UpdateOptions
		daemonset v1.DaemonSet
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	daemonset.Name = opts.ObjectName
	daemonset.Namespace = opts.Namespace
	err = pixiu.CoreV1.Cloud().DaemonSets(opts.Cloud).Update(c, &daemonset)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var opts meta.DeleteOptions
	if err := c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().DaemonSets(opts.Cloud).Delete(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.GetOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().DaemonSets(opts.Cloud).Get(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listDaemonSets API: clouds/<cloud_name>/namespaces/<ns>/daemonsets
func (s *cloudRouter) listDaemonsets(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.ListOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().DaemonSets(opts.Cloud).List(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
