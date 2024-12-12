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

package static

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ServeFileSystem interface {
	http.FileSystem
	Exists(prefix string, path string) bool
}

func ServeRoot(urlPrefix, root string) gin.HandlerFunc {
	return Serve(urlPrefix, LocalFile(root, false))
}

// Serve returns a middleware handler that serves static files in the given directory.
func Serve(urlPrefix string, fs ServeFileSystem) gin.HandlerFunc {
	return ServeCached(urlPrefix, fs, 0)
}

// ServeCached returns a middleware handler that similar as Serve
// but with the Cache-Control Header set as passed in the cacheAge parameter
func ServeCached(urlPrefix string, fs ServeFileSystem, cacheAge uint) gin.HandlerFunc {
	fileserver := http.FileServer(fs)
	if urlPrefix != "" {
		fileserver = http.StripPrefix(urlPrefix, fileserver)
	}
	return func(c *gin.Context) {
		if fs.Exists(urlPrefix, c.Request.URL.Path) {
			if cacheAge != 0 {
				c.Writer.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d", cacheAge))
			}
			fileserver.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		}
	}
}
