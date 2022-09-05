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

package app

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/gopixiu/api/server/middleware"
	"github.com/caoyingjunz/gopixiu/api/server/router/cicd"
	"github.com/caoyingjunz/gopixiu/api/server/router/cloud"
	"github.com/caoyingjunz/gopixiu/api/server/router/demo"
	"github.com/caoyingjunz/gopixiu/api/server/router/user"
	"github.com/caoyingjunz/gopixiu/cmd/app/options"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func NewServerCommand() *cobra.Command {
	opts, err := options.NewOptions()
	if err != nil {
		log.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use:  "gopixiu-server",
		Long: "The gopixiu server controller is a daemon than embeds the core control loops.",
		Run: func(cmd *cobra.Command, args []string) {
			if err = opts.Complete(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			if err = opts.Validate(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			if err = Run(opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	// 绑定命令行参数
	opts.BindFlags(cmd)
	return cmd
}

func InitRouters(opt *options.Options) {
	middleware.InitMiddlewares(opt.GinEngine) // 注册中间件

	demo.NewRouter(opt.GinEngine)  // 注册 demo 路由
	cicd.NewRouter(opt.GinEngine)  // 注册 cicd 路由
	cloud.NewRouter(opt.GinEngine) // 注册 cloud 路由
	user.NewRouter(opt.GinEngine)  // 注册 user 路由
}

func Run(opt *options.Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置核心应用接口
	pixiu.Setup(opt)
	// 初始化已存在 cloud clients
	if err := pixiu.CoreV1.Cloud().InitCloudClients(); err != nil {
		return err
	}

	// 初始化 api 路由
	InitRouters(opt)

	klog.Infof("starting pixiu server")
	// 启动主进程
	opt.Run(ctx.Done())

	select {}
}
