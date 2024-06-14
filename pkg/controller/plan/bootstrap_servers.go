/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/util/container"
)

type BootStrap struct {
	handlerTask

	dir    string
	runner string
}

func (b BootStrap) Name() string { return "初始化部署环境" }

// Run 以容器的形式执行 BootStrap 任务，如果存在旧的容器，则先删除在执行
func (b BootStrap) Run() error {
	cli, err := container.NewContainer("bootstrap-servers", b.GetPlanId(), b.dir)
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	// 启动执行容器
	if err = cli.StartAndWaitForContainer(ctx, b.runner); err != nil {
		return err
	}

	return nil
}
