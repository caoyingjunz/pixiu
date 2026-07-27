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

package tunnel

import (
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	pixtunnel "github.com/caoyingjunz/pixiu/pkg/tunnel"
)

// NewRouter registers the agent reverse-tunnel websocket endpoint.
// Registered without gin.WrapH response wrapping issues by using the raw writer/request.
func NewRouter(o *options.Options) {
	m := pixtunnel.Default()
	if m == nil {
		klog.Error("tunnel manager is not initialized; skip registering /pixiu/connect")
		return
	}
	o.HttpEngine.GET(pixtunnel.ConnectPath, func(c *gin.Context) {
		// RemoteDialer needs the underlying Hijacker; pass through gin writer/request.
		m.ServeHTTP(c.Writer, c.Request)
	})
	klog.Infof("tunnel connect endpoint registered: GET %s", pixtunnel.ConnectPath)
}
