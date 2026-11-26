/*
Copyright 2026 The Pixiu Authors.

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

package httputils

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/util/sqlaudit"
)

const (
	auditSkipKey      = "audit_skip"
	auditDetailKey    = "audit_detail"
	auditSQLObjectKey = "audit_sql_object"
)

func ShouldSkipAudit(c *gin.Context) bool {
	return c != nil && c.GetBool(auditSkipKey)
}

func GetAuditDetail(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString(auditDetailKey)
}

func GetAuditSQLObject(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString(auditSQLObjectKey)
}

// ApplySQLAudit 复用控制器的只读判定和语句拆分结果。
func ApplySQLAudit(ctx context.Context, readOnly bool, database string, statements ...string) {
	c, ok := ctx.(*gin.Context)
	if !ok || c == nil {
		return
	}
	if readOnly {
		c.Set(auditSkipKey, true)
		return
	}
	c.Set(auditDetailKey, sqlaudit.Summary(statements...))
	c.Set(auditSQLObjectKey, sqlaudit.Tables(database, statements...))
}
