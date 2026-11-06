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

package updater

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

// Config 是 pixiu-updater 的独立配置。
type Config struct {
	Enabled         bool
	VersionURL      string
	ImageRepository string
	Interval        time.Duration
	ContainerName   string
	// PollInterval 轮询数据库中升级请求的间隔。
	PollInterval time.Duration

	HealthCheckURL     string
	HealthCheckTimeout time.Duration
	HealthCheckRetries int

	Runtime config.RuntimeOptions
	Mysql   config.MysqlOptions
}

type rawConfig struct {
	Upgrade struct {
		Enabled            bool   `yaml:"enabled"`
		VersionURL         string `yaml:"version_url"`
		ImageRepository    string `yaml:"image_repository"`
		Interval           string `yaml:"interval"`
		ContainerName      string `yaml:"container_name"`
		PollInterval       string `yaml:"poll_interval"`
		HealthCheckURL     string `yaml:"health_check_url"`
		HealthCheckTimeout string `yaml:"health_check_timeout"`
		HealthCheckRetries int    `yaml:"health_check_retries"`
	} `yaml:"upgrade"`
	Runtime config.RuntimeOptions `yaml:"runtime"`
	Mysql   config.MysqlOptions   `yaml:"mysql"`
}

// Load 读取并校验 updater 配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置(%s)失败: %v", path, err)
	}

	var raw rawConfig
	if err = yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析配置(%s)失败: %v", path, err)
	}

	cfg := &Config{
		Enabled:            raw.Upgrade.Enabled,
		VersionURL:         raw.Upgrade.VersionURL,
		ImageRepository:    raw.Upgrade.ImageRepository,
		ContainerName:      raw.Upgrade.ContainerName,
		HealthCheckURL:     raw.Upgrade.HealthCheckURL,
		HealthCheckTimeout: defaultHealthTimeout,
		HealthCheckRetries: raw.Upgrade.HealthCheckRetries,
		Runtime:            raw.Runtime,
		Mysql:              raw.Mysql,
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
	cfg.PollInterval = defaultPollInterval
	if raw.Upgrade.PollInterval != "" {
		interval, parseErr := time.ParseDuration(raw.Upgrade.PollInterval)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.poll_interval 非法(%s): %v", raw.Upgrade.PollInterval, parseErr)
		}
		cfg.PollInterval = interval
	}
	cfg.Interval = defaultInterval
	if raw.Upgrade.Interval != "" {
		interval, parseErr := time.ParseDuration(raw.Upgrade.Interval)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.interval 非法(%s): %v", raw.Upgrade.Interval, parseErr)
		}
		cfg.Interval = interval
	}
	if raw.Upgrade.HealthCheckTimeout != "" {
		timeout, parseErr := time.ParseDuration(raw.Upgrade.HealthCheckTimeout)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.health_check_timeout 非法(%s): %v", raw.Upgrade.HealthCheckTimeout, parseErr)
		}
		cfg.HealthCheckTimeout = timeout
	}
	if cfg.Interval < time.Minute {
		return nil, fmt.Errorf("upgrade.interval 不能小于 1 分钟")
	}
	if cfg.PollInterval < time.Second {
		return nil, fmt.Errorf("upgrade.poll_interval 不能小于 1 秒")
	}
	if cfg.Mysql.Host == "" || cfg.Mysql.Name == "" || cfg.Mysql.User == "" {
		return nil, fmt.Errorf("mysql 配置不完整（host/name/user 必填）")
	}

	cfg.Runtime.SetDefaults()
	if err = cfg.Runtime.Valid(); err != nil {
		return nil, err
	}
	return cfg, nil
}
