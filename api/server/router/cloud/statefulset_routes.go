package cloud

import (
	"context"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/apps/v1"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		createOptions types.GetOrCreateOptions
		statefulset   v1.StatefulSet
	)
	if err = c.ShouldBindUri(&createOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	statefulset.Name = createOptions.ObjectName
	statefulset.Namespace = createOptions.Namespace
	err = pixiu.CoreV1.Cloud().StatefulSets(createOptions.CloudName).Create(context.TODO(), &statefulset)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err           error
		createOptions types.GetOrCreateOptions
		statefulset   v1.StatefulSet
	)
	if err = c.ShouldBindUri(&createOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	statefulset.Name = createOptions.ObjectName
	statefulset.Namespace = createOptions.Namespace
	err = pixiu.CoreV1.Cloud().StatefulSets(createOptions.CloudName).Update(context.TODO(), &statefulset)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		delOptions types.GetOrDeleteOptions
	)
	if err = c.ShouldBindUri(&delOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	err = pixiu.CoreV1.Cloud().StatefulSets(delOptions.CloudName).Delete(context.TODO(), delOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getStatefulSet(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		getOptions types.GetOrDeleteOptions
	)
	if err = c.ShouldBindUri(&getOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().StatefulSets(getOptions.CloudName).Get(context.TODO(), getOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listStatefulSets(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		listOptions types.ListOptions
	)
	if err = c.ShouldBindUri(&listOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().StatefulSets(listOptions.CloudName).List(context.TODO(), listOptions)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
