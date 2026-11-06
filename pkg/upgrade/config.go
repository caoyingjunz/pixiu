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

package upgrade

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

const (
	defaultVersionURL    = "https://console.cloud.pixiuio.com"
	defaultContainerName = "pixiu"
	defaultInterval      = 6 * time.Hour
	defaultHealthTimeout = 60 * time.Second
	defaultHealthRetries = 10
	defaultPollInterval  = 5 * time.Second
)

// Config 是 pixiu-upgrade 的配置（与主服务共用 config.yaml，读取 upgrade/runtime/mysql 段）。
type Config struct {
	Enabled         bool
	VersionURL      string
	ImageRepository string
	Interval        time.Duration
	ContainerName   string
	// UpgradeID 升级执行者标识（默认主机名），用于任务认领与状态回写校验。
	UpgradeID string
	// PollInterval 轮询数据库中升级请求的间隔。
	PollInterval time.Duration

	HealthCheckURL     string
	HealthCheckTimeout time.Duration
	HealthCheckRetries int

	Runtime config.RuntimeOptions
	Mysql   config.MysqlOptions
}

type rawConfig struct {
	Upgrade upgradeSpec           `yaml:"upgrade"`
	Runtime config.RuntimeOptions `yaml:"runtime"`
	Mysql   config.MysqlOptions   `yaml:"mysql"`
}

// upgradeSpec 是 upgrade 配置段的原始形态（字符串形式的时长）。
type upgradeSpec struct {
	Enabled            bool   `yaml:"enabled"`
	VersionURL         string `yaml:"version_url"`
	ImageRepository    string `yaml:"image_repository"`
	Interval           string `yaml:"interval"`
	ContainerName      string `yaml:"container_name"`
	UpgradeID          string `yaml:"upgrade_id"`
	PollInterval       string `yaml:"poll_interval"`
	HealthCheckURL     string `yaml:"health_check_url"`
	HealthCheckTimeout string `yaml:"health_check_timeout"`
	HealthCheckRetries int    `yaml:"health_check_retries"`
}

// Load 读取并校验 pixiu 配置文件中的 upgrade/runtime/mysql 段（pixiu-upgrade 与主服务共用 config.yaml）。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置(%s)失败: %v", path, err)
	}

	var raw rawConfig
	if err = yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析配置(%s)失败: %v", path, err)
	}

	return normalize(raw.Upgrade, raw.Runtime, raw.Mysql, true)
}

// FromUpgradeOptions 由主服务配置构造 upgrade 进程配置（版本检测在主服务内运行，不需要 mysql 段）。
func FromUpgradeOptions(opt config.UpgradeOptions, rt config.RuntimeOptions) (*Config, error) {
	return normalize(upgradeSpec{
		Enabled:            opt.Enabled,
		VersionURL:         opt.VersionURL,
		ImageRepository:    opt.ImageRepository,
		Interval:           opt.Interval,
		ContainerName:      opt.ContainerName,
		PollInterval:       opt.PollInterval,
		HealthCheckURL:     opt.HealthCheckURL,
		HealthCheckTimeout: opt.HealthCheckTimeout,
		HealthCheckRetries: opt.HealthCheckRetries,
	}, rt, config.MysqlOptions{}, false)
}

// normalize 校验并补全默认值；requireMysql 为 true 时校验数据库配置（独立进程需要自行连库）。
func normalize(spec upgradeSpec, rt config.RuntimeOptions, mysql config.MysqlOptions, requireMysql bool) (*Config, error) {
	cfg := &Config{
		Enabled:            spec.Enabled,
		VersionURL:         spec.VersionURL,
		ImageRepository:    spec.ImageRepository,
		ContainerName:      spec.ContainerName,
		UpgradeID:          spec.UpgradeID,
		HealthCheckURL:     spec.HealthCheckURL,
		HealthCheckTimeout: defaultHealthTimeout,
		HealthCheckRetries: spec.HealthCheckRetries,
		Runtime:            rt,
		Mysql:              mysql,
	}

	if cfg.VersionURL == "" {
		cfg.VersionURL = defaultVersionURL
	}
	if cfg.ContainerName == "" {
		cfg.ContainerName = defaultContainerName
	}
	if cfg.HealthCheckRetries <= 0 {
		cfg.HealthCheckRetries = defaultHealthRetries
	}
	// 主服务检查器不传 upgrade_id，这里统一补默认值（主机名）
	if cfg.UpgradeID == "" {
		if hostname, hostErr := os.Hostname(); hostErr == nil && hostname != "" {
			cfg.UpgradeID = hostname
		} else {
			cfg.UpgradeID = "pixiu-upgrade"
		}
	}
	cfg.PollInterval = defaultPollInterval
	if spec.PollInterval != "" {
		interval, parseErr := time.ParseDuration(spec.PollInterval)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.poll_interval 非法(%s): %v", spec.PollInterval, parseErr)
		}
		cfg.PollInterval = interval
	}
	cfg.Interval = defaultInterval
	if spec.Interval != "" {
		interval, parseErr := time.ParseDuration(spec.Interval)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.interval 非法(%s): %v", spec.Interval, parseErr)
		}
		cfg.Interval = interval
	}
	if spec.HealthCheckTimeout != "" {
		timeout, parseErr := time.ParseDuration(spec.HealthCheckTimeout)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.health_check_timeout 非法(%s): %v", spec.HealthCheckTimeout, parseErr)
		}
		cfg.HealthCheckTimeout = timeout
	}
	if cfg.Interval < time.Minute {
		return nil, fmt.Errorf("upgrade.interval 不能小于 1 分钟")
	}
	if cfg.PollInterval < time.Second {
		return nil, fmt.Errorf("upgrade.poll_interval 不能小于 1 秒")
	}
	if requireMysql && (cfg.Mysql.Host == "" || cfg.Mysql.Name == "" || cfg.Mysql.User == "") {
		return nil, fmt.Errorf("mysql 配置不完整（host/name/user 必填）")
	}

	cfg.Runtime.SetDefaults()
	if err := cfg.Runtime.Valid(); err != nil {
		return nil, err
	}
	return cfg, nil
}
