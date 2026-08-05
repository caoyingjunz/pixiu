/*
Copyright 2024 The Pixiu Authors.

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

package user

import (
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func TestParseK8sProxyPath(t *testing.T) {
	cases := []struct {
		act      string
		wantNs   string
		wantName string
	}{
		{"/apis/apps/v1/namespaces/default/deployments/nginx", "default", "nginx"},
		{"/api/v1/namespaces/kube-system/pods", "kube-system", ""},
		{"api/v1/namespaces/ns1/services/svc1/status", "ns1", "svc1"},
		{"/api/v1/nodes/node-1", "", ""},
		{"/apis/apps/v1/deployments", "", ""},
	}
	for _, c := range cases {
		ns, name := parseK8sProxyPath(c.act)
		if ns != c.wantNs || name != c.wantName {
			t.Fatalf("parseK8sProxyPath(%q)=(%q,%q), want (%q,%q)", c.act, ns, name, c.wantNs, c.wantName)
		}
	}
}

func TestMatchRoleAPIScope(t *testing.T) {
	scopes := []model.RoleAPIScope{
		{Cluster: "c1", Namespace: "default", ResourceName: "*"},
		{Cluster: "c1", Namespace: "prod", ResourceName: "web"},
	}
	if !matchRoleAPIScope(scopes, "c1", "default", "any") {
		t.Fatal("expected default/* match")
	}
	if !matchRoleAPIScope(scopes, "c1", "prod", "web") {
		t.Fatal("expected prod/web match")
	}
	if matchRoleAPIScope(scopes, "c1", "prod", "other") {
		t.Fatal("expected prod/other deny")
	}
	if matchRoleAPIScope(scopes, "c2", "default", "x") {
		t.Fatal("expected other cluster deny")
	}
	if !matchRoleAPIScope(scopes, "c1", "", "") {
		t.Fatal("expected cluster-only request match any scope on cluster")
	}
}
