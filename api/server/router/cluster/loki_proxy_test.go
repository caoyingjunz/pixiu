package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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
			want:      "/api/v1/namespaces/loki/services/http:loki:3100/proxy",
		},
		{
			name:      "query endpoint",
			act:       "/loki/api/v1/query_range",
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
