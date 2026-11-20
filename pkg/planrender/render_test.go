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

package planrender

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/caoyingjunz/pixiu/pkg/types"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

// genTestPrivateKey 动态生成测试用 OpenSSH 私钥 PEM（末尾带 LF）。
// 不硬编码任何真实私钥，也不在日志中输出私钥正文。
func genTestPrivateKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "pixiu-test")
	if err != nil {
		t.Fatalf("marshal test key: %v", err)
	}
	return string(pem.EncodeToMemory(block))
}

// assertReadableKeyFile 断言落盘文件以 LF 结尾、PEM 结构完整且可被 ssh 解析。
func assertReadableKeyFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("key file %s must end with a trailing LF (OpenSSH/libcrypto requirement)", path)
	}
	if !strings.Contains(string(data), "-----END OPENSSH PRIVATE KEY-----") {
		t.Fatalf("key file %s missing PEM end marker", path)
	}
	if _, err := sshutil.ParsePrivateKeySigner(string(data)); err != nil {
		t.Fatalf("key file %s cannot be parsed by ssh: %v", path, err)
	}
}

func TestNormalizeNodePrivateKeyTrailingNewline(t *testing.T) {
	valid := genTestPrivateKey(t)

	cases := []struct {
		name  string
		input string
	}{
		{"with trailing LF", valid},
		{"trailing LF stripped", strings.TrimRight(valid, "\n")},
		{"trailing whitespace", valid + "\n\n  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeNodePrivateKey("node01", tc.input)
			if err != nil {
				t.Fatalf("normalizeNodePrivateKey: %v", err)
			}
			if !strings.HasSuffix(got, "\n") {
				t.Fatalf("normalized key must end with a trailing LF")
			}
			signer, err := ssh.ParsePrivateKey([]byte(got))
			if err != nil {
				t.Fatalf("normalized key must be parseable by ssh: %v", err)
			}
			if signer == nil {
				t.Fatalf("expected non-nil signer")
			}
		})
	}
}

func TestNormalizeNodePrivateKeyInvalidKeyReportsNode(t *testing.T) {
	_, err := normalizeNodePrivateKey("k8s-worker01", "this is not a private key")
	if err == nil {
		t.Fatal("expected error for invalid private key")
	}
	if !strings.Contains(err.Error(), "k8s-worker01") {
		t.Fatalf("error should name the node, got: %v", err)
	}
	if strings.Contains(err.Error(), "this is not a private key") {
		t.Fatalf("error must not echo private key material, got: %v", err)
	}
}

func TestWriteExpandRSAKeepsTrailingNewline(t *testing.T) {
	planDir := t.TempDir()
	// 模拟 issue 场景：凭据往返后私钥末尾 LF 丢失
	keyNoLF := strings.TrimRight(genTestPrivateKey(t), "\n")

	if err := writeExpandRSA(planDir, "k8s-worker01", keyNoLF); err != nil {
		t.Fatalf("writeExpandRSA: %v", err)
	}
	assertReadableKeyFile(t, filepath.Join(planDir, "ssh", "k8s-worker01", "id_rsa"))
}

func TestWriteRSAKeepsTrailingNewline(t *testing.T) {
	workDir := t.TempDir()
	keyNoLF := strings.TrimRight(genTestPrivateKey(t), "\n")
	auth := types.PlanNodeAuth{
		Type: types.KeyAuth,
		Key:  &types.KeySpec{User: "root", Data: keyNoLF},
	}

	path, err := writeRSA(42, "master01", workDir, auth)
	if err != nil {
		t.Fatalf("writeRSA: %v", err)
	}
	assertReadableKeyFile(t, path)
}
