package cloud

import (
	"context"

	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions meta.CreateOptions
		job        batchv1.Job
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&job); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	job.Namespace = getOptions.Namespace
	if err = pixiu.CoreV1.Cloud().Jobs(getOptions.Cloud).Create(context.TODO(), &job); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		createOptions meta.UpdateOptions
		job           batchv1.Job
	)
	if err = c.ShouldBindUri(&createOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	job.Name = createOptions.ObjectName
	job.Namespace = createOptions.Namespace
	err = pixiu.CoreV1.Cloud().Jobs(createOptions.Cloud).Update(context.TODO(), &job)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	var deleteOptions meta.DeleteOptions
	if err := c.ShouldBindUri(&deleteOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().Jobs(deleteOptions.Cloud).Delete(context.TODO(), deleteOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions meta.GetOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Jobs(getOptions.Cloud).Get(context.TODO(), getOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listJobs(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		listOptions meta.ListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Jobs(listOptions.Cloud).List(context.TODO(), listOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
