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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"k8s.io/klog/v2"

	pixiudb "github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/upgrade"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

func main() {
	var configPath string
	klog.InitFlags(nil)
	// upgrade 进程与 pixiu 主服务共用 config.yaml，读取其中的 upgrade/runtime/mysql 段
	flag.StringVar(&configPath, "config", "/etc/pixiu/config.yaml", "pixiu 配置文件路径")
	flag.Parse()
	defer klog.Flush()

	cfg, err := upgrade.Load(configPath)
	if err != nil {
		klog.Fatalf("加载配置失败: %v", err)
	}
	if !cfg.Enabled {
		klog.Info("pixiu-upgrade 未启用（upgrade.enabled=false），退出")
		return
	}

	factory, err := newFactory(cfg)
	if err != nil {
		klog.Fatalf("初始化数据库失败: %v", err)
	}

	rt, err := runtime.New(cfg.Runtime)
	if err != nil {
		klog.Fatalf("初始化容器运行时失败: %v", err)
	}
	defer rt.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 版本检测由 pixiu 主服务负责，本进程只轮询升级请求并执行容器重建
	upgrade.New(cfg, rt, factory).RunExecutor(ctx)
}

func newFactory(cfg *upgrade.Config) (pixiudb.ShareDaoFactory, error) {
	port := cfg.Mysql.Port
	if port == 0 {
		port = 3306
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		cfg.Mysql.User, cfg.Mysql.Password, cfg.Mysql.Host, port, cfg.Mysql.Name)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// 确保升级状态表存在（pixiu 侧也有 AutoMigrate，此处兜底）
	if err = db.AutoMigrate(&model.UpgradeRecord{}); err != nil {
		return nil, err
	}
	return pixiudb.NewDaoFactory(db, false)
}
