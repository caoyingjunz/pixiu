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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/router"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

func NewServerCommand(version string) *cobra.Command {
	opts, err := options.NewOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use:  "pixiu-server",
		Long: "The pixiu server controller is a daemon that embeds the core control loops.",
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

	verCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Long:  "Print version and exit.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
	cmd.AddCommand(verCmd)
	return cmd
}

// Run 优雅启动貔貅服务
func Run(opt *options.Options) error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", opt.ComponentConfig.Default.Listen),
		Handler: opt.HttpEngine,
	}

	// 启动部署计划
	// TODO: 暂未设置优雅退出
	if err := opt.Controller.Plan().Run(context.TODO(), 5); err != nil {
		klog.Fatal("failed to listen pixiu server: ", err)
	}
	// 安装 http 路由
	router.InstallRouters(opt)

	// Initializing the server in a goroutine so that it won't block the graceful shutdown handling below
	go func() {
		klog.Infof("starting pixiu server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatal("failed to listen pixiu server: ", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 5 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	klog.Infof("shutting pixiu server down ...")

	// The context is used to inform the server it has 5 seconds to finish the request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		klog.Fatalf("pixiu server forced to shutdown: %v", err)
	}

	return nil
}
