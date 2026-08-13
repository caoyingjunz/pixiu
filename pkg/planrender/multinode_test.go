package planrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestRenderMultinodePasswordAuth(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		port     int
		password string
		want     string
		unwanted string
	}{
		{
			name:     "sudo user",
			user:     "deploy",
			port:     2222,
			password: "sudo-password",
			want: "kube230 ansible_ssh_user=deploy ansible_ssh_pass=sudo-password " +
				"ansible_ssh_port=2222 " +
				"ansible_become=true ansible_become_method=sudo ansible_become_user=root " +
				"ansible_become_password=sudo-password",
		},
		{
			name:     "root user remains unchanged",
			user:     "root",
			password: "root-password",
			want:     "kube230 ansible_ssh_user=root ansible_ssh_pass=root-password",
			unwanted: "ansible_become=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir := t.TempDir()
			plan := &types.Plan{
				PixiuMeta: types.PixiuMeta{Id: 1},
				Config: types.PlanConfig{
					Runtime: types.RuntimeSpec{Runtime: string(model.ContainerdCRI)},
				},
				Nodes: []types.PlanNode{
					{
						Name: "kube230",
						Role: []string{model.MasterRole},
						Auth: types.PlanNodeAuth{
							Type: types.PasswordAuth,
							Port: tt.port,
							Password: &types.PasswordSpec{
								User:     tt.user,
								Password: tt.password,
							},
						},
					},
				},
			}

			if err := Render(workDir, plan); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(workDir, "1", "multinode"))
			if err != nil {
				t.Fatal(err)
			}
			got := string(content)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("expected inventory line %q, got:\n%s", tt.want, got)
			}
			if tt.unwanted != "" && strings.Contains(got, tt.unwanted) {
				t.Fatalf("inventory unexpectedly contains %q:\n%s", tt.unwanted, got)
			}
		})
	}
}
