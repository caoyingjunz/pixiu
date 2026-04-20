/*
Copyright 2024 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	defaultAuditQueueSize = 2048
	defaultAuditWorkers   = 2
	auditWriteTimeout     = 3 * time.Second
)

type auditRecorder struct {
	factory db.ShareDaoFactory
	queue   chan *model.Audit
}

var (
	recorderOnce sync.Once
	recorderInst *auditRecorder
)

func getAuditRecorder(o *options.Options) *auditRecorder {
	recorderOnce.Do(func() {
		recorderInst = &auditRecorder{
			factory: o.Factory,
			queue:   make(chan *model.Audit, defaultAuditQueueSize),
		}
		for i := 0; i < defaultAuditWorkers; i++ {
			go recorderInst.run()
		}
	})
	return recorderInst
}

func (r *auditRecorder) run() {
	for record := range r.queue {
		r.write(record)
	}
}

func (r *auditRecorder) write(record *model.Audit) {
	ctx, cancel := context.WithTimeout(context.Background(), auditWriteTimeout)
	defer cancel()

	if _, err := r.factory.Audit().Create(ctx, record); err != nil {
		klog.Errorf("failed to create audit record [%s]: %v", record.String(), err)
	}
}

func (r *auditRecorder) enqueue(record *model.Audit) {
	select {
	case r.queue <- record:
	default:
		// 队列满时降级同步写入，确保非 GET 请求都能落库
		klog.Warningf("audit queue is full, fallback to direct write: %s", record.Path)
		r.write(record)
	}
}

func Audit(o *options.Options) gin.HandlerFunc {
	recorder := getAuditRecorder(o)
	return func(c *gin.Context) {
		if !shouldAudit(c) {
			c.Next()
			return
		}

		c.Next()
		recorder.enqueue(buildAuditRecord(c))
	}
}

func buildAuditRecord(c *gin.Context) *model.Audit {
	userName := "unknown"
	if user, err := httputils.GetUserFromRequest(c); err == nil && user != nil {
		userName = user.Name
	}

	return &model.Audit{
		RequestId:  requestid.Get(c),
		Action:     c.Request.Method,
		Ip:         c.ClientIP(),
		Operator:   userName,
		Path:       c.Request.RequestURI,
		ObjectType: detectObjectType(c),
		Status:     getAuditStatus(c),
	}
}

func shouldAudit(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/pixiu")
}

func detectObjectType(c *gin.Context) model.ObjectType {
	obj, _, ok := httputils.GetObjectFromRequest(c)
	if !ok {
		return model.ObjectAll
	}
	ot := model.ObjectType(obj)
	if _, exists := model.ObjectTypeMap[ot]; exists {
		return ot
	}
	return model.ObjectAll
}

// getAuditStatus returns the status of operation.
func getAuditStatus(c *gin.Context) model.AuditOperationStatus {
	respCode := httputils.GetResponseCode(c)
	if respCode == 0 {
		respCode = c.Writer.Status()
		if respCode == 0 {
			return model.AuditOpUnknown
		}
	}

	if responseOK(respCode) {
		return model.AuditOpSuccess
	}

	return model.AuditOpFail
}

func responseOK(code int) bool {
	return code == http.StatusOK ||
		code == http.StatusCreated ||
		code == http.StatusAccepted
}
