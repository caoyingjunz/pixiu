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

type Multinode struct {
	DockerMaster     []types.PlanNode
	DockerNode       []types.PlanNode
	ContainerdMaster []types.PlanNode
	ContainerdNode   []types.PlanNode
	StorageNode      []types.PlanNode
}

// Render 将 hosts / multinode / globals.yml / ssh key 渲染到 workDir/<planId>/。
func Render(workDir string, plan *types.Plan) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}
	planDir := filepath.Join(workDir, fmt.Sprintf("%d", plan.Id))
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}

	if err := writeTemplate(filepath.Join(planDir, "hosts"), pixiutpl.HostTemplate, plan); err != nil {
		return err
	}

	nodes, err := buildMultinode(plan, workDir)
	if err != nil {
		return err
	}
	if err = writeTemplate(filepath.Join(planDir, "multinode"), pixiutpl.MultiModeTemplate, nodes); err != nil {
		return err
	}

	// 启用自定义源时，将内容直接写入 pixiu 文件（容器内路径 /configs/pixiu）
	cfg := plan.Config
	if cfg.Component.CustomRepo != nil && cfg.Component.CustomRepo.Enable {
		pixiuFile := filepath.Join(planDir, "pixiu")
		if err = os.WriteFile(pixiuFile, []byte(cfg.Component.CustomRepo.Content), 0o600); err != nil {
			return fmt.Errorf("write custom repo file %s: %w", pixiuFile, err)
		}
	}

	return writeTemplate(filepath.Join(planDir, "globals.yml"), pixiutpl.GlobalsTemplate, &plan.Config)
}

func writeTemplate(filename, text string, data interface{}) error {
	tpl := template.Must(template.New(filepath.Base(filename)).Parse(text))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0o600)
}

func buildMultinode(plan *types.Plan, workDir string) (Multinode, error) {
	multinode := Multinode{
		DockerMaster:     make([]types.PlanNode, 0),
		DockerNode:       make([]types.PlanNode, 0),
		ContainerdMaster: make([]types.PlanNode, 0),
		ContainerdNode:   make([]types.PlanNode, 0),
		StorageNode:      make([]types.PlanNode, 0),
	}
	if plan == nil {
		return multinode, fmt.Errorf("plan is nil")
	}

	runtime := plan.Config.Runtime
	for _, node := range plan.Nodes {
		nodeAuth := node.Auth
		if _, err := writeRSA(plan.Id, node.Name, workDir, nodeAuth); err != nil {
			return multinode, err
		}

		if nodeAuth.Type == types.KeyAuth {
			if nodeAuth.Key == nil {
				return multinode, fmt.Errorf("node(%s) key auth config is empty", node.Name)
			}
			// 拷贝避免改写调用方数据
			key := *nodeAuth.Key
			key.File = fmt.Sprintf("/configs/ssh/%s/id_rsa", node.Name)
			nodeAuth.Key = &key
		}
		if nodeAuth.Type == types.PasswordAuth && nodeAuth.Password == nil {
			return multinode, fmt.Errorf("node(%s) password auth config is empty", node.Name)
		}
		planNode := types.PlanNode{Name: node.Name, Auth: nodeAuth}
		roles := node.Role
		if len(roles) == 0 {
			continue
		}

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

// RenderExpand 渲染集群扩容所需的 hosts / multinode / globals.yml / ssh key。
// 与部署渲染隔离：使用独立目录，只包含本次扩容的目标节点，不依赖部署计划。
func RenderExpand(planDir string, nodes []types.PlanNode, globals pixiutpl.ExpandGlobals, customRepo *types.CustomRepo) error {
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}

	if err := writeTemplate(filepath.Join(planDir, "hosts"), pixiutpl.HostTemplate, struct {
		Nodes []types.PlanNode
	}{Nodes: nodes}); err != nil {
		return err
	}

	prepared := make([]types.PlanNode, 0, len(nodes))
	for i := range nodes {
		node := nodes[i]
		if node.Auth.Type == types.KeyAuth {
			if node.Auth.Key == nil {
				return fmt.Errorf("node(%s) key auth config is empty", node.Name)
			}
			if err := writeExpandRSA(planDir, node.Name, node.Auth.Key.Data); err != nil {
				return err
			}
			key := *node.Auth.Key
			key.File = fmt.Sprintf("/configs/ssh/%s/id_rsa", node.Name)
			node.Auth.Key = &key
		}
		if node.Auth.Type == types.PasswordAuth && node.Auth.Password == nil {
			return fmt.Errorf("node(%s) password auth config is empty", node.Name)
		}
		prepared = append(prepared, node)
	}

	multinode := Multinode{
		DockerMaster:     make([]types.PlanNode, 0),
		DockerNode:       make([]types.PlanNode, 0),
		ContainerdMaster: make([]types.PlanNode, 0),
		ContainerdNode:   prepared,
		StorageNode:      make([]types.PlanNode, 0),
	}
	if err := writeTemplate(filepath.Join(planDir, "multinode"), pixiutpl.MultiModeTemplate, multinode); err != nil {
		return err
	}

	if customRepo != nil && customRepo.Enable {
		if err := os.WriteFile(filepath.Join(planDir, "pixiu"), []byte(customRepo.Content), 0o600); err != nil {
			return fmt.Errorf("write custom repo file: %w", err)
		}
	}

	return writeTemplate(filepath.Join(planDir, "globals.yml"), pixiutpl.ExpandGlobalsTemplate, globals)
}

func writeExpandRSA(planDir, name, data string) error {
	rsaDir := filepath.Join(planDir, "ssh", name)
	if err := os.MkdirAll(rsaDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rsaDir, "id_rsa"), []byte(data), 0o600)
}

// NewPlan 将 DB 模型转为 types.Plan，供 local 模式控制面渲染使用。
func NewPlan(planId int64, cfg *model.Config, nodes []model.Node) (*types.Plan, error) {
	plan := &types.Plan{
		PixiuMeta: types.PixiuMeta{Id: planId},
		Nodes:     make([]types.PlanNode, 0, len(nodes)),
	}
	if cfg != nil {
		ks := types.KubernetesSpec{}
		if err := ks.Unmarshal(cfg.Kubernetes); err != nil {
			return nil, err
		}
		ns := types.NetworkSpec{}
		if err := ns.Unmarshal(cfg.Network); err != nil {
			return nil, err
		}
		rs := types.RuntimeSpec{}
		if err := rs.Unmarshal(cfg.Runtime); err != nil {
			return nil, err
		}
		cs := types.ComponentSpec{}
		if err := cs.Unmarshal(cfg.Component); err != nil {
			return nil, err
		}
		plan.Config = types.PlanConfig{
			PlanId:     planId,
			Region:     cfg.Region,
			OSImage:    cfg.OSImage,
			Kubernetes: ks,
			Network:    ns,
			Runtime:    rs,
			Component:  cs,
		}
	}
	for _, n := range nodes {
		auth := types.PlanNodeAuth{}
		if err := auth.Unmarshal(n.Auth); err != nil {
			return nil, err
		}
		var roles []string
		if n.Role != "" {
			roles = strings.Split(n.Role, ",")
		}
		plan.Nodes = append(plan.Nodes, types.PlanNode{
			Name:   n.Name,
			PlanId: planId,
			Role:   roles,
			CRI:    n.CRI,
			Ip:     n.Ip,
			Auth:   auth,
		})
	}
	return plan, nil
}
