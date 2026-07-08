/*
Copyright 2021 The Pixiu Authors.

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

package proxy

import "testing"

func TestParseServiceProxyPath(t *testing.T) {
	target, ok := parseServiceProxyPath("/api/v1/namespaces/logging/services/elasticsearch:9200/proxy/_search")
	if !ok {
		t.Fatal("expected service proxy path to parse")
	}
	if target.namespace != "logging" || target.service != "elasticsearch" || target.port != 9200 || target.path != "/_search" {
		t.Fatalf("unexpected target: %+v", target)
	}

	_, ok = parseServiceProxyPath("/api/v1/namespaces/logging/services/elasticsearch:9200/proxy")
	if !ok {
		t.Fatal("expected path without suffix to parse")
	}

	_, ok = parseServiceProxyPath("api/v1/namespaces/logging/services/elasticsearch:9200/proxy/_search")
	if ok {
		t.Fatal("service proxy path without leading slash should not parse")
	}

	_, ok = parseServiceProxyPath("/api/v1/namespaces/logging/pods/elasticsearch-0/proxy/_search")
	if ok {
		t.Fatal("pod proxy path should not parse as service proxy path")
	}
}

func TestExtractServiceProxyPathFromRequestPath(t *testing.T) {
	const clusterName = "demo"
	fullPath := "/pixiu/proxy/demo/api/v1/namespaces/logging/services/elasticsearch:9200/proxy/_search"
	k8sPath := fullPath[len(proxyBaseURL+"/"+clusterName):]

	target, ok := parseServiceProxyPath(k8sPath)
	if !ok {
		t.Fatalf("expected extracted path to parse, got %q", k8sPath)
	}
	if target.path != "/_search" {
		t.Fatalf("unexpected path suffix: %q", target.path)
	}
}
