package cluster

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestFindLokiNamespace(t *testing.T) {
	namespaces := []v1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{
			Name:   "loki",
			Labels: map[string]string{lokiNamespaceLabelKey: lokiNamespaceLabelValue},
		}},
	}

	got, err := findLokiNamespace(namespaces)
	if err != nil {
		t.Fatalf("findLokiNamespace() unexpected error = %v", err)
	}
	if got != "loki" {
		t.Fatalf("findLokiNamespace() = %q, want %q", got, "loki")
	}
}

func TestFindLokiService(t *testing.T) {
	services := []v1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "loki-distributed-gateway",
				Labels: map[string]string{lokiGatewayLabelKey: lokiGatewayLabelValue},
			},
			Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{Name: "http", Port: 80}}},
		},
	}

	got, err := findLokiService(services)
	if err != nil {
		t.Fatalf("findLokiService() unexpected error = %v", err)
	}
	if got == nil || got.Name != "loki-distributed-gateway" {
		t.Fatalf("findLokiService() = %v, want %q", got, "loki-distributed-gateway")
	}
	if port := findLokiServicePort(got); port != 80 {
		t.Fatalf("findLokiServicePort() = %d, want %d", port, 80)
	}
}
