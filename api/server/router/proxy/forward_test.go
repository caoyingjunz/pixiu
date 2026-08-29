package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestStripWWWAuthenticateWriter 验证代理透传响应会剥离 WWW-Authenticate（大小写不敏感），
// 且不误伤其他响应头与状态码。
func TestStripWWWAuthenticateWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	w := &stripWWWAuthenticateWriter{ResponseWriter: c.Writer}

	// 模拟上游响应：小写变体的 WWW-Authenticate + 正常业务头
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Www-Authenticate", `Basic realm="es"`)
	w.WriteHeader(http.StatusUnauthorized)
	// gin 的 WriteHeader 仅记录状态码，WriteHeaderNow 才写入底层 writer
	w.WriteHeaderNow()

	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Fatalf("WWW-Authenticate should be stripped, got %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type should be preserved, got %q", got)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestStripWWWAuthenticateWriterKeepsAuthHeader 验证仅剥离 WWW-Authenticate，
// 其它认证相关头（如 Authorization）不受影响。
func TestStripWWWAuthenticateWriterKeepsAuthHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	w := &stripWWWAuthenticateWriter{ResponseWriter: c.Writer}

	w.Header().Set("Authorization", "Bearer abc")
	w.Header().Set("WWW-Authenticate", `Basic realm="es"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.WriteHeaderNow()

	if got := rec.Header().Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("Authorization should be preserved, got %q", got)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Fatalf("WWW-Authenticate should be stripped, got %q", got)
	}
}
