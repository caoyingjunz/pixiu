/*
Copyright 2021 The Pixiu Authors.

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

package proxy

import "github.com/gin-gonic/gin"

// challengeStrippingResponseWriter 剥离上游 401 认证挑战头（WWW-Authenticate / Proxy-Authenticate），
// 防止浏览器对 /pixiu/proxy 的响应弹出原生 Basic Auth 登录框。
// 内嵌 gin.ResponseWriter，自动获得 Hijack/Flush/WriteString/Status/Size/Pusher 等全部方法，
// 保证 websocket/SPDY 升级与现有行为不变。
type challengeStrippingResponseWriter struct {
	gin.ResponseWriter
}

// WriteHeader 在写出状态码之前剥离认证挑战头。
func (w *challengeStrippingResponseWriter) WriteHeader(code int) {
	w.Header().Del("WWW-Authenticate")
	w.Header().Del("Proxy-Authenticate")
	w.ResponseWriter.WriteHeader(code)
}
