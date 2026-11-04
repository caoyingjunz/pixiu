package template

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestKeyInventoryUser(t *testing.T) {
	tpl, err := template.New("inventory").Parse(MultiModeTemplate)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"ubuntu", ""} {
		t.Run("user="+user, func(t *testing.T) {
			node := struct {
				Name string
				Auth types.PlanNodeAuth
			}{Name: "node1", Auth: types.PlanNodeAuth{Type: types.KeyAuth, Key: &types.KeySpec{User: user, File: "/key"}}}
			data := map[string]any{}
			for _, group := range []string{"DockerMaster", "DockerNode", "ContainerdMaster", "ContainerdNode", "StorageNode"} {
				data[group] = []any{node}
			}
			var out bytes.Buffer
			if err := tpl.Execute(&out, data); err != nil {
				t.Fatal(err)
			}
			want := "root"
			if user != "" {
				want = user
			}
			if strings.Count(out.String(), "ansible_ssh_user="+want+" ") != 5 {
				t.Fatalf("incorrect inventory login user: %s", out.String())
			}
			becomeCount := strings.Count(out.String(), "ansible_become=true")
			if (user == "" && becomeCount != 0) || (user != "" && becomeCount != 5) {
				t.Fatalf("unexpected sudo configuration: %s", out.String())
			}
		})
	}
}
