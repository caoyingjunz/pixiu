package cluster

import (
	"net/url"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestBuildLokiServiceProxyPath(t *testing.T) {
	tests := []struct {
		name     string
		act      string
		endpoint lokiEndpoint
		want     string
	}{
		{
			name: "base path",
			act:  "",
			endpoint: lokiEndpoint{
				Namespace: "loki",
				Service:   "loki",
				Port:      3100,
				Scheme:    "http",
			},
			want: "/api/v1/namespaces/loki/services/http:loki:3100/proxy/loki/api/v1/query_range",
		},
		{
			name: "query endpoint",
			act:  "/api/v1/query_range",
			endpoint: lokiEndpoint{
				Namespace: "monitoring",
				Service:   "loki-gateway",
				Port:      80,
				Scheme:    "http",
			},
			want: "/api/v1/namespaces/monitoring/services/http:loki-gateway:80/proxy/loki/api/v1/query_range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLokiServiceProxyPath(tt.act, tt.endpoint)
			if got != tt.want {
				t.Fatalf("buildLokiServiceProxyPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeLokiAPIPath(t *testing.T) {
	tests := []struct {
		name string
		act  string
		want string
	}{
		{
			name: "default query path",
			act:  "",
			want: "/loki/api/v1/query_range",
		},
		{
			name: "prefix plain api path",
			act:  "/api/v1/query_range",
			want: "/loki/api/v1/query_range",
		},
		{
			name: "prefix path without leading slash",
			act:  "api/v1/query_range",
			want: "/loki/api/v1/query_range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLokiAPIPath(tt.act)
			if got != tt.want {
				t.Fatalf("normalizeLokiAPIPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveServicePortFromURL(t *testing.T) {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Name: "http", Port: 80},
				{Name: "grpc", Port: 9095},
			},
		},
	}

	targetURL, err := url.Parse("http://loki.loki.svc.cluster.local:80")
	if err != nil {
		t.Fatalf("url.Parse() unexpected error = %v", err)
	}

	port := resolveServicePortFromURL(service, targetURL)
	if port != 80 {
		t.Fatalf("resolveServicePortFromURL() = %d, want %d", port, 80)
	}
}

func TestJoinURLPath(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		suffix string
		want   string
	}{
		{
			name:   "empty base",
			base:   "",
			suffix: "/loki/api/v1/query_range",
			want:   "/loki/api/v1/query_range",
		},
		{
			name:   "joins paths",
			base:   "/proxy",
			suffix: "/loki/api/v1/query_range",
			want:   "/proxy/loki/api/v1/query_range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinURLPath(tt.base, tt.suffix)
			if got != tt.want {
				t.Fatalf("joinURLPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
