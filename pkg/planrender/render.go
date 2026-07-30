/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package planrender

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

// Data 部署计划渲染输入（控制面 local 模式与边缘 agent 共用）。
type Data struct {
	PlanId int64
	Nodes  []Node
	Config Config
}

// Node 渲染所需的节点信息。
type Node struct {
	Name string
	Role string
	CRI  string
	Ip   string
	Auth string // PlanNodeAuth JSON
}

// Config 渲染所需的配置（字段为 DB 中原始 JSON 字符串）。
type Config struct {
	OSImage    string
	Region     string
	Kubernetes string
	Network    string
	Runtime    string
	Component  string
}

type Multinode struct {
	DockerMaster     []types.PlanNode
	DockerNode       []types.PlanNode
	ContainerdMaster []types.PlanNode
	ContainerdNode   []types.PlanNode
	StorageNode      []types.PlanNode
}

// RenderToDir 将 hosts / multinode / globals.yml / ssh key 渲染到 workDir/<planId>/。
func RenderToDir(workDir string, data Data) error {
	planDir := filepath.Join(workDir, fmt.Sprintf("%d", data.PlanId))
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}

	if err := writeTemplate(filepath.Join(planDir, "hosts"), pixiutpl.HostTemplate, data); err != nil {
		return err
	}

	nodes, err := ParseMultinode(data, workDir)
	if err != nil {
		return err
	}
	if err = writeTemplate(filepath.Join(planDir, "multinode"), pixiutpl.MultiModeTemplate, nodes); err != nil {
		return err
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		return err
	}
	return writeTemplate(filepath.Join(planDir, "globals.yml"), pixiutpl.GlobalsTemplate, cfg)
}

func writeTemplate(filename, text string, data interface{}) error {
	tpl := template.Must(template.New(filepath.Base(filename)).Parse(text))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0o600)
}

func ParseMultinode(data Data, workDir string) (Multinode, error) {
	multinode := Multinode{
		DockerMaster:     make([]types.PlanNode, 0),
		DockerNode:       make([]types.PlanNode, 0),
		ContainerdMaster: make([]types.PlanNode, 0),
		ContainerdNode:   make([]types.PlanNode, 0),
		StorageNode:      make([]types.PlanNode, 0),
	}

	runtime := types.RuntimeSpec{}
	if err := runtime.Unmarshal(data.Config.Runtime); err != nil {
		return multinode, err
	}

	for _, node := range data.Nodes {
		nodeAuth := types.PlanNodeAuth{}
		if err := nodeAuth.Unmarshal(node.Auth); err != nil {
			return multinode, err
		}
		if _, err := writeRSA(data.PlanId, node.Name, workDir, nodeAuth); err != nil {
			return multinode, err
		}

		if nodeAuth.Type == types.KeyAuth {
			if nodeAuth.Key == nil {
				return multinode, fmt.Errorf("node(%s) key auth config is empty", node.Name)
			}
			nodeAuth.Key.File = fmt.Sprintf("/configs/ssh/%s/id_rsa", node.Name)
		}
		if nodeAuth.Type == types.PasswordAuth && nodeAuth.Password == nil {
			return multinode, fmt.Errorf("node(%s) password auth config is empty", node.Name)
		}
		planNode := types.PlanNode{Name: node.Name, Auth: nodeAuth}
		roles := strings.Split(node.Role, ",")

		if runtime.IsDocker() {
			for _, role := range roles {
				if role == model.MasterRole {
					multinode.DockerMaster = append(multinode.DockerMaster, planNode)
				}
				if role == model.NodeRole {
					multinode.DockerNode = append(multinode.DockerNode, planNode)
				}
			}
		}
		if runtime.IsContainerd() {
			for _, role := range roles {
				if role == model.MasterRole {
					multinode.ContainerdMaster = append(multinode.ContainerdMaster, planNode)
				}
				if role == model.NodeRole {
					multinode.ContainerdNode = append(multinode.ContainerdNode, planNode)
				}
			}
		}
		for _, role := range roles {
			if role == model.StorageRole {
				multinode.StorageNode = append(multinode.StorageNode, planNode)
			}
		}
	}

	return multinode, nil
}

func writeRSA(planId int64, name, workDir string, auth types.PlanNodeAuth) (string, error) {
	if auth.Type != types.KeyAuth {
		return "", nil
	}
	if auth.Key == nil {
		return "", fmt.Errorf("node(%s) key auth config is empty", name)
	}
	rsaDir := filepath.Join(workDir, fmt.Sprintf("%d", planId), "ssh", name)
	if err := os.MkdirAll(rsaDir, 0o755); err != nil {
		return "", err
	}
	f := filepath.Join(rsaDir, "id_rsa")
	if err := os.WriteFile(f, []byte(auth.Key.Data), 0o600); err != nil {
		return "", err
	}
	return f, nil
}

func ParseConfig(data Data) (*types.PlanConfig, error) {
	network := types.NetworkSpec{}
	if err := network.Unmarshal(data.Config.Network); err != nil {
		return nil, err
	}
	kubernetes := types.KubernetesSpec{}
	if err := kubernetes.Unmarshal(data.Config.Kubernetes); err != nil {
		return nil, err
	}
	component := types.ComponentSpec{}
	if err := component.Unmarshal(data.Config.Component); err != nil {
		return nil, err
	}
	runtimeSpec := types.RuntimeSpec{}
	if err := runtimeSpec.Unmarshal(data.Config.Runtime); err != nil {
		return nil, err
	}
	return &types.PlanConfig{
		Kubernetes: kubernetes,
		Network:    network,
		Component:  component,
		Runtime:    runtimeSpec,
	}, nil
}

// FromModels 从 DB 模型构造渲染输入。
func FromModels(planId int64, cfg *model.Config, nodes []model.Node) Data {
	out := Data{PlanId: planId, Nodes: make([]Node, 0, len(nodes))}
	if cfg != nil {
		out.Config = Config{
			OSImage:    cfg.OSImage,
			Region:     cfg.Region,
			Kubernetes: cfg.Kubernetes,
			Network:    cfg.Network,
			Runtime:    cfg.Runtime,
			Component:  cfg.Component,
		}
	}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, Node{
			Name: n.Name,
			Role: n.Role,
			CRI:  string(n.CRI),
			Ip:   n.Ip,
			Auth: n.Auth,
		})
	}
	return out
}
