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

package options

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/db"
	pixiuConfig "github.com/caoyingjunz/pixiulib/config"
)

const (
	maxIdleConns = 10
	maxOpenConns = 100

	defaultListen     = 8080
	defaultTokenKey   = "pixiu"
	defaultConfigFile = "/etc/pixiu/config.yaml"
	defaultLogFormat  = config.LogFormatJson
	defaultWorkDir    = "/etc/pixiu"

	defaultSlowSQLDuration = 1 * time.Second
)

// Options has all the params needed to run a pixiu
type Options struct {
	// The default values.
	ComponentConfig config.Config
	HttpEngine      *gin.Engine

	// 数据库接口
	Factory db.ShareDaoFactory
	// 貔貅主控制接口
	Controller controller.PixiuInterface

	// ConfigFile is the location of the pixiu server's configuration file.
	ConfigFile string
}

func NewOptions() (*Options, error) {
	return &Options{
		HttpEngine: gin.Default(), // 初始化默认 api 路由
		ConfigFile: defaultConfigFile,
	}, nil
}

// Complete completes all the required options
func (o *Options) Complete() error {
	// 配置文件优先级: 默认配置，环境变量，命令行
	if len(o.ConfigFile) == 0 {
		// Try to read config file path from env.
		if cfgFile := os.Getenv("ConfigFile"); cfgFile != "" {
			o.ConfigFile = cfgFile
		} else {
			o.ConfigFile = defaultConfigFile
		}
	}

	c := pixiuConfig.New()
	c.SetConfigFile(o.ConfigFile)
	c.SetConfigType("yaml")
	if err := c.Binding(&o.ComponentConfig); err != nil {
		return err
	}

	// TODO: move to config initialization?
	if o.ComponentConfig.Default.Listen == 0 {
		o.ComponentConfig.Default.Listen = defaultListen
	}
	if len(o.ComponentConfig.Default.JWTKey) == 0 {
		o.ComponentConfig.Default.JWTKey = defaultTokenKey
	}
	if o.ComponentConfig.Default.LogFormat == "" {
		o.ComponentConfig.Default.LogFormat = defaultLogFormat
	}
	if o.ComponentConfig.Worker.WorkDir == "" {
		o.ComponentConfig.Worker.WorkDir = defaultWorkDir
	}

	if err := o.ComponentConfig.Valid(); err != nil {
		return err
	}

	// 注册依赖组件
	if err := o.register(); err != nil {
		return err
	}

	o.Controller = controller.New(o.ComponentConfig, o.Factory)
	return nil
}

// BindFlags binds the pixiu Configuration struct fields
func (o *Options) BindFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.ConfigFile, "configfile", defaultConfigFile, "The location of the pixiu configuration file")
}

func (o *Options) register() error {
	// 注册数据库
	if err := o.registerDatabase(); err != nil {
		return err
	}

	// TODO: 注册其他依赖
	return nil
}

func (o *Options) registerDatabase() error {
	sqlConfig := o.ComponentConfig.Mysql
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		sqlConfig.User,
		sqlConfig.Password,
		sqlConfig.Host,
		sqlConfig.Port,
		sqlConfig.Name)

	opt := &gorm.Config{
		Logger: db.NewLogger(logger.Info, defaultSlowSQLDuration),
	}
	DB, err := gorm.Open(mysql.Open(dsn), opt)
	if err != nil {
		return err
	}
	// 设置数据库连接池
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	o.Factory, err = db.NewDaoFactory(DB, o.ComponentConfig.Default.AutoMigrate)
	if err != nil {
		return err
	}
	return nil
}

// Validate validates all the required options.
func (o *Options) Validate() error {
	// TODO
	return nil
}
