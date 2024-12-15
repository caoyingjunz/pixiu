/*
* Copyright 2024 Pixiu. All rights reserved.
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*     http:*www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package static

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type embedFileSystem struct {
	http.FileSystem
}

func (e embedFileSystem) Exists(prefix string, path string) bool {
	_, err := e.Open(path)
	return err == nil
}

func EmbedFolder(fsEmbed embed.FS, reqPath string) (ServeFileSystem, error) {
	targetPath := strings.TrimSpace(reqPath)
	if targetPath == "" {
		return embedFileSystem{
			FileSystem: http.FS(fsEmbed),
		}, nil
	}

	fsys, _ := fs.Sub(fsEmbed, targetPath)
	_, err := fsEmbed.Open(targetPath)
	if err != nil {
		return nil, err
	}

	return embedFileSystem{
		FileSystem: http.FS(fsys),
	}, nil
}

func ServeEmbed(reqPath string, fsEmbed embed.FS) gin.HandlerFunc {
	embedFS, err := EmbedFolder(fsEmbed, reqPath)
	if err != nil {
		return func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "initialization of embed folder failed",
				"error":   err.Error(),
			})
		}
	}
	return gin.WrapH(http.FileServer(embedFS))
}
