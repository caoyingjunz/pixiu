package cloud

import (
	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.CreateOptions
		job  batchv1.Job
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&job); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	job.Namespace = opts.Namespace
	if err = pixiu.CoreV1.Cloud().Jobs(opts.Cloud).Create(c, &job); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.UpdateOptions
		job  batchv1.Job
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	job.Name = opts.ObjectName
	job.Namespace = opts.Namespace
	err = pixiu.CoreV1.Cloud().Jobs(opts.Cloud).Update(c, &job)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	var opts meta.DeleteOptions
	if err := c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err := pixiu.CoreV1.Cloud().Jobs(opts.Cloud).Delete(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.GetOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Jobs(opts.Cloud).Get(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listJobs(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts meta.ListOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Jobs(opts.Cloud).List(c, opts)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
