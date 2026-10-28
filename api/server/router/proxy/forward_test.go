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

package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	k8sproxy "k8s.io/apimachinery/pkg/util/proxy"
)

func TestApplyClusterHost(t *testing.T) {
	tests := []struct {
		name        string
		kubePath    string
		targetPath  string
		targetRaw   string
		targetQuery string
		wantPath    string
		wantRawPath string
		wantQuery   string
	}{
		{
			name:        "kubeconfig host without path leaves target unchanged",
			kubePath:    "",
			targetPath:  "/api/v1/namespaces",
			targetQuery: "limit=1",
			wantPath:    "/api/v1/namespaces",
			wantRawPath: "",
			wantQuery:   "limit=1",
		},
		{
			name:        "kubeconfig host with root path leaves target and raw path unchanged",
			kubePath:    "/",
			targetPath:  "/api/v1/namespaces/a b",
			targetRaw:   "/api/v1/namespaces/a%20b",
			targetQuery: "limit=1",
			wantPath:    "/api/v1/namespaces/a b",
			wantRawPath: "/api/v1/namespaces/a%20b",
			wantQuery:   "limit=1",
		},
		{
			name:        "prefix without trailing slash",
			kubePath:    "/inCluster",
			targetPath:  "/api/v1/namespaces",
			wantPath:    "/inCluster/api/v1/namespaces",
			wantRawPath: "",
		},
		{
			name:        "prefix with trailing slash",
			kubePath:    "/inCluster/",
			targetPath:  "/api/v1/namespaces",
			wantPath:    "/inCluster/api/v1/namespaces",
			wantRawPath: "",
		},
		{
			name:        "prefix with duplicated trailing slash",
			kubePath:    "/inCluster//",
			targetPath:  "/api/v1/namespaces",
			wantPath:    "/inCluster/api/v1/namespaces",
			wantRawPath: "",
		},
		{
			name:        "empty target path becomes prefix root",
			kubePath:    "/inCluster",
			targetPath:  "",
			wantPath:    "/inCluster/",
			wantRawPath: "",
		},
		{
			name:        "root target path becomes prefix root",
			kubePath:    "/inCluster",
			targetPath:  "/",
			wantPath:    "/inCluster/",
			wantRawPath: "",
		},
		{
			name:        "target trailing slash preserved",
			kubePath:    "/inCluster",
			targetPath:  "/api/v1/namespaces/",
			wantPath:    "/inCluster/api/v1/namespaces/",
			wantRawPath: "",
		},
		{
			name:        "escaped target path keeps decoded path and clears raw path",
			kubePath:    "/inCluster",
			targetPath:  "/api/v1/namespaces/a b",
			targetRaw:   "/api/v1/namespaces/a%20b",
			wantPath:    "/inCluster/api/v1/namespaces/a b",
			wantRawPath: "",
		},
		{
			name:        "raw query untouched",
			kubePath:    "/inCluster",
			targetPath:  "/api/v1/namespaces",
			targetQuery: "limit=1&watch=true",
			wantPath:    "/inCluster/api/v1/namespaces",
			wantRawPath: "",
			wantQuery:   "limit=1&watch=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeURL, err := url.Parse("https://cluster.example.com:6443" + tt.kubePath)
			if err != nil {
				t.Fatalf("parse kubeconfig host: %v", err)
			}
			target := &url.URL{
				Path:     tt.targetPath,
				RawPath:  tt.targetRaw,
				RawQuery: tt.targetQuery,
			}

			applyClusterHost(target, kubeURL)

			if target.Path != tt.wantPath {
				t.Errorf("target.Path = %q, want %q", target.Path, tt.wantPath)
			}
			if target.RawPath != tt.wantRawPath {
				t.Errorf("target.RawPath = %q, want %q", target.RawPath, tt.wantRawPath)
			}
			if target.RawQuery != tt.wantQuery {
				t.Errorf("target.RawQuery = %q, want %q", target.RawQuery, tt.wantQuery)
			}
			if target.Host != kubeURL.Host {
				t.Errorf("target.Host = %q, want %q", target.Host, kubeURL.Host)
			}
			if target.Scheme != kubeURL.Scheme {
				t.Errorf("target.Scheme = %q, want %q", target.Scheme, kubeURL.Scheme)
			}
		})
	}
}

func TestParseTargetsWithClusterPrefix(t *testing.T) {
	kubeURL, err := url.Parse("https://cluster.example.com:6443/inCluster")
	if err != nil {
		t.Fatalf("parse kubeconfig host: %v", err)
	}
	router := &proxyRouter{}

	const wantPath = "/inCluster/api/v1/namespaces"

	proxyTarget, err := router.parseProxyTarget(url.URL{Path: "/pixiu/proxy/demo/api/v1/namespaces"}, "demo")
	if err != nil {
		t.Fatalf("parseProxyTarget: %v", err)
	}
	applyClusterHost(proxyTarget, kubeURL)
	if proxyTarget.Path != wantPath {
		t.Errorf("proxy target path = %q, want %q", proxyTarget.Path, wantPath)
	}

	gatewayTarget, err := router.parseKubeGatewayTarget(url.URL{Path: "/k8s/demo/api/v1/namespaces"}, "demo")
	if err != nil {
		t.Fatalf("parseKubeGatewayTarget: %v", err)
	}
	applyClusterHost(gatewayTarget, kubeURL)
	if gatewayTarget.Path != wantPath {
		t.Errorf("gateway target path = %q, want %q", gatewayTarget.Path, wantPath)
	}
}

func TestForwardedPathKeepsKubeconfigPrefix(t *testing.T) {
	tests := []struct {
		name     string
		kubePath string
		wantPath string
	}{
		{
			name:     "with kubeconfig prefix",
			kubePath: "/inCluster/",
			wantPath: "/inCluster/api/v1/namespaces",
		},
		{
			name:     "without kubeconfig prefix",
			kubePath: "",
			wantPath: "/api/v1/namespaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotQuery string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
			}))
			defer upstream.Close()

			kubeURL, err := url.Parse(upstream.URL + tt.kubePath)
			if err != nil {
				t.Fatalf("parse kubeconfig host: %v", err)
			}
			target := &url.URL{Path: "/api/v1/namespaces"}
			applyClusterHost(target, kubeURL)

			handler := k8sproxy.NewUpgradeAwareHandler(target, http.DefaultTransport, false, false, nil)
			req := httptest.NewRequest(http.MethodPost, "/pixiu/proxy/demo/api/v1/namespaces?limit=1", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("upstream status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
			}
			if gotPath != tt.wantPath {
				t.Errorf("upstream path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotQuery != "limit=1" {
				t.Errorf("upstream raw query = %q, want %q", gotQuery, "limit=1")
			}
		})
	}
}
