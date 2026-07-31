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

package deployagent

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 为 deploy-agent 的配置文件结构。
// 配置文件优先级高于环境变量。
type Config struct {
	Default DefaultConfig `yaml:"default"`
}

// DefaultConfig 为配置文件中的 default 段。
type DefaultConfig struct {
	Server  string `yaml:"server"`
	Token   string `yaml:"token"`
	WorkDir string `yaml:"work_dir"`
}

// LoadConfig 从 YAML 文件读取配置。文件不存在时返回空配置。
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Resolve 返回最终配置，配置文件优先，环境变量作为回退。
func (c Config) Resolve() (server, token, workDir string) {
	server = firstNonEmpty(c.Default.Server, os.Getenv("PIXIU_SERVER"))
	token = firstNonEmpty(c.Default.Token, os.Getenv("PIXIU_DEPLOY_TOKEN"))
	workDir = firstNonEmpty(c.Default.WorkDir, os.Getenv("PIXIU_AGENT_WORKDIR"))
	if workDir == "" {
		workDir = "/etc/pixiu"
	}
	return
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
