package cloud

import (
	"context"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/apps/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		createOptions meta.CreateOptions
		daemonset     v1.DaemonSet
	)
	if err = c.ShouldBindUri(&createOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&daemonset); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	daemonset.Namespace = createOptions.Namespace
	if err = pixiu.CoreV1.Cloud().DaemonSets(createOptions.Cloud).Create(context.TODO(), &daemonset); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		updateOptions meta.UpdateOptions
		daemonset     v1.DaemonSet
	)
	if err = c.ShouldBindUri(&updateOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	daemonset.Name = updateOptions.ObjectName
	daemonset.Namespace = updateOptions.Namespace
	err = pixiu.CoreV1.Cloud().DaemonSets(updateOptions.Cloud).Update(context.TODO(), &daemonset)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var deleteOptions meta.DeleteOptions
	if err := c.ShouldBindUri(&deleteOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().DaemonSets(deleteOptions.Cloud).Delete(context.TODO(), deleteOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getDaemonSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions meta.GetOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().DaemonSets(getOptions.Cloud).Get(context.TODO(), getOptions)
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
		err         error
		listOptions meta.ListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().DaemonSets(listOptions.Cloud).List(context.TODO(), listOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
