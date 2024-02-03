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

package main

import (
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app"
)

var version string

// @title           Pixiu API Documentation
// @version         1.0
// @termsOfService  https://github.com/caoyingjunz/pixiu

// @contact.name   API Support
// @contact.url    https://github.com/caoyingjunz/pixiu
// @contact.email  support@pixiu.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
// @schemes       http https
// @host          localhost:8090

// @securityDefinitions.apikey  Bearer
// @in                          header
// @name                        Authorization
// @description                 Use the Pixiu APIs to your cloud
// @description                 Type "Bearer" followed by a space and JWT token
func main() {
	klog.InitFlags(nil)
	rand.Seed(time.Now().UnixNano())

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	cmd := app.NewServerCommand(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
