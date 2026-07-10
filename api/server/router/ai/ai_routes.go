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

package ai

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (r *aiRouter) respondStream(c *gin.Context) {
	var req types.AIRespondRequest
	if err := httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		c.JSON(400, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	emit := func(event *types.AIStreamEvent) error {
		if event == nil {
			return nil
		}
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err = c.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}

	if _, err := r.c.AI().RespondStream(c, &req, emit); err != nil {
		_ = emit(&types.AIStreamEvent{
			Type:    "error",
			Stage:   "failed",
			Message: err.Error(),
		})
	}
}
