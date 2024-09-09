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
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

// Render 渲染 pixiu 部署配置
// 1. 渲染 hosts
// 2. 渲染 globals.yaml
// 3. 渲染 multinode
// 具体参考 https://github.com/pixiu-io/kubez-ansible
type Render struct {
	handlerTask

	dir string
}

func (r Render) Name() string { return "配置渲染" }
func (r Render) Run() error {
	// 渲染 hosts
	if err := r.doRender("hosts", pixiutpl.HostTemplate, r.data); err != nil {
		return err
	}
	// 渲染 multiNode
	nodes, err := ParseMultinode(r.data, r.dir)
	if err != nil {
		return err
	}
	if err := r.doRender("multinode", pixiutpl.MultiModeTemplate, nodes); err != nil {
		return err
	}
	// 渲染 globals
	cfg, err := ParseConfig(r.data)
	if err != nil {
		return err
	}
	if err := r.doRender("globals.yml", pixiutpl.GlobalsTemplate, cfg); err != nil {
		return err
	}

	return nil
}

func (r Render) doRender(name string, text string, data interface{}) error {
	tpl := template.New(name)
	tpl = template.Must(tpl.Parse(text))

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	filename, err := GetRenderFile(r.GetPlanId(), r.dir, name)
	if err != nil {
		return err
	}
	if err = util.WriteToFile(filename, buf.Bytes()); err != nil {
		return err
	}

	return nil
}

type Multinode struct {
	DockerMaster     []types.PlanNode
	DockerNode       []types.PlanNode
	ContainerdMaster []types.PlanNode
	ContainerdNode   []types.PlanNode
}

func ParseMultinode(data TaskData, workDir string) (Multinode, error) {
	multinode := Multinode{
		DockerMaster:     make([]types.PlanNode, 0),
		DockerNode:       make([]types.PlanNode, 0),
		ContainerdMaster: make([]types.PlanNode, 0),
		ContainerdNode:   make([]types.PlanNode, 0),
	}

	runtime := types.RuntimeSpec{}
	if err := runtime.Unmarshal(data.Config.Runtime); err != nil {
		return multinode, err
	}

	for _, node := range data.Nodes {
		nodeAuth := types.PlanNodeAuth{}
		err := nodeAuth.Unmarshal(node.Auth)
		if err != nil {
			return multinode, err
		}
		// 生成rsa的渲染文件
		_, err = RenderRSA(data.PlanId, node.Name, workDir, nodeAuth)
		if err != nil {
			return multinode, err
		}
		nodeAuth.Key.File = fmt.Sprintf("/configs/ssh/%s/id_rsa", node.Name)
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
	}

	return multinode, nil
}

// GetRenderFile
// TODO: 后续优化
func GetRenderFile(planId int64, workDir string, f string) (string, error) {
	planDir := filepath.Join(workDir, fmt.Sprintf("%d", planId))
	if err := util.EnsureDirectoryExists(planDir); err != nil {
		return "", err
	}

	return filepath.Join(planDir, f), nil
}

func RenderRSA(planId int64, name string, workDir string, auth types.PlanNodeAuth) (string, error) {
	if auth.Type == types.KeyAuth {
		f, err := GetRSAFile(planId, workDir, name)
		if err != nil {
			return "", err
		}
		if err = util.WriteToFile(f, []byte(auth.Key.Data)); err != nil {
			return "", err
		}
		return f, nil
	}

	return "", nil
}

func GetRSAFile(planId int64, workDir string, name string) (string, error) {
	rsaDir := filepath.Join(workDir, fmt.Sprintf("%d", planId), "ssh", name)
	if err := util.EnsureDirectoryExists(rsaDir); err != nil {
		return "", err
	}

	return filepath.Join(rsaDir, "id_rsa"), nil
}

func ParseConfig(data TaskData) (*types.PlanConfig, error) {
	config := data.Config

	network := types.NetworkSpec{}
	if err := network.Unmarshal(config.Network); err != nil {
		return nil, err
	}
	kubernetes := types.KubernetesSpec{}
	if err := kubernetes.Unmarshal(config.Kubernetes); err != nil {
		return nil, err
	}
	component := types.ComponentSpec{}
	if err := component.Unmarshal(config.Component); err != nil {
		return nil, err
	}

	return &types.PlanConfig{
		Kubernetes: kubernetes,
		Network:    network,
		Component:  component,
	}, nil
}
