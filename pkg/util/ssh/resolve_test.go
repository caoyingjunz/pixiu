package ssh

import (
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func TestResolveKeyAuthUser(t *testing.T) {
	for _, tc := range []struct {
		name string
		user string
		want string
	}{
		{"non-root", "ubuntu", "ubuntu"},
		{"legacy", "", "root"},
		{"trimmed", " deploy ", "deploy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := types.PlanNodeAuth{Type: types.KeyAuth, Port: 2222, Key: &types.KeySpec{User: tc.user, Data: "private-key"}}
			encoded, err := auth.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			var saved types.PlanNodeAuth
			if err := saved.Unmarshal(encoded); err != nil {
				t.Fatal(err)
			}
			req, err := ResolveAuth(&saved)
			if err != nil {
				t.Fatal(err)
			}
			if req.User != tc.want || req.Port != 2222 || req.PrivateKey != "private-key" {
				t.Fatalf("unexpected SSH parameters: user=%q port=%d", req.User, req.Port)
			}
		})
	}
}
