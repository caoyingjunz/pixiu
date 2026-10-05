package runtime

import (
	"testing"
	"time"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

// 懒加载语义：Init 只登记配置不拨号（本机无 CRI 也必须立即返回）
func TestInitIsLazyAndFast(t *testing.T) {
	opts := config.RuntimeOptions{CRI: "containerd", Socket: "/tmp/pixiu-nonexistent-test.sock"}
	start := time.Now()
	Init(opts)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Init 疑似拨号（耗时 %v），应按懒加载仅登记配置", elapsed)
	}
}

// Default 首次使用时才连接；连接失败返回错误（containerd 客户端拨号约 10s 超时，属预期）
func TestDefaultConnectsLazilyAndFails(t *testing.T) {
	Init(config.RuntimeOptions{CRI: "containerd", Socket: "/tmp/pixiu-nonexistent-test.sock"})
	if _, err := Default(); err == nil {
		t.Fatal("socket 不存在时 Default 应返回连接错误")
	}
}

// 未知 CRI 在连接时给出明确错误（不拨号，快速返回）
func TestDefaultUnknownCRI(t *testing.T) {
	Init(config.RuntimeOptions{CRI: "bogus"})
	if _, err := Default(); err == nil {
		t.Fatal("未知 CRI 应返回错误")
	}
}
