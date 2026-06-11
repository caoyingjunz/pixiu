package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildLokiServiceProxyPath(t *testing.T) {
	tests := []struct {
		name      string
		act       string
		namespace string
		service   string
		port      int
		scheme    string
		want      string
	}{
		{
			name:      "base path",
			act:       "",
			namespace: "loki",
			service:   "loki",
			port:      3100,
			scheme:    "http",
			want:      "/api/v1/namespaces/loki/services/http:loki:3100/proxy/loki/api/v1/query_range",
		},
		{
			name:      "query endpoint",
			act:       "/api/v1/query_range",
			namespace: "monitoring",
			service:   "loki-gateway",
			port:      80,
			scheme:    "http",
			want:      "/api/v1/namespaces/monitoring/services/http:loki-gateway:80/proxy/loki/api/v1/query_range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLokiServiceProxyPath(tt.act, tt.namespace, tt.service, tt.port, tt.scheme)
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

func TestEnsureDefaultLokiOrgID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets default header when absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pixiu/kubeproxy/clusters/test/loki", nil)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		ensureDefaultLokiOrgID(c)

		if got := c.Request.Header.Get("X-Scope-OrgID"); got != defaultLokiOrgID {
			t.Fatalf("X-Scope-OrgID = %q, want %q", got, defaultLokiOrgID)
		}
	})

	t.Run("keeps caller header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pixiu/kubeproxy/clusters/test/loki", nil)
		req.Header.Set("X-Scope-OrgID", "tenant-a")
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		ensureDefaultLokiOrgID(c)

		if got := c.Request.Header.Get("X-Scope-OrgID"); got != "tenant-a" {
			t.Fatalf("X-Scope-OrgID = %q, want %q", got, "tenant-a")
		}
	})
}

func TestSelectLokiNamespace(t *testing.T) {
	namespaces := []v1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "monitoring"}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:   "loki",
			Labels: map[string]string{lokiNamespaceLabelKey: lokiNamespaceLabelValue},
		}},
	}

	got := selectLokiNamespace(namespaces)
	if got != "loki" {
		t.Fatalf("selectLokiNamespace() = %q, want %q", got, "loki")
	}
}

func TestSelectLokiService(t *testing.T) {
	services := []v1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "loki"},
			Spec:       v1.ServiceSpec{Ports: []v1.ServicePort{{Port: 3100}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "loki-distributed-gateway",
				Labels: map[string]string{lokiGatewayLabelKey: lokiGatewayLabelValue},
			},
			Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{Name: "http", Port: 80}}},
		},
	}

	got := selectLokiService(services)
	if got == nil || got.Name != "loki-distributed-gateway" {
		t.Fatalf("selectLokiService() = %v, want %q", got, "loki-distributed-gateway")
	}
	if port := selectLokiServicePort(got); port != 80 {
		t.Fatalf("selectLokiServicePort() = %d, want %d", port, 80)
	}
}
