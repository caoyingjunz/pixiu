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

package clusterexpand

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

func TestNormalizeAPIServer(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "scheme and port", raw: "https://10.0.0.10:6443", want: "10.0.0.10:6443"},
		{name: "no scheme", raw: "10.0.0.10:6443", want: "10.0.0.10:6443"},
		{name: "no port", raw: "10.0.0.10", want: "10.0.0.10:6443"},
		{name: "path suffix", raw: "https://k8s.example.com:8443/prefix", want: "k8s.example.com:8443"},
		{name: "in-cluster dns", raw: "kubernetes.default.svc", wantErr: true},
		{name: "svc suffix", raw: "https://api.default.svc:6443", wantErr: true},
		{name: "localhost", raw: "127.0.0.1:6443", wantErr: true},
		{name: "empty", raw: " ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeAPIServer(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseKubeadmClusterConfiguration(t *testing.T) {
	raw := `apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: v1.28.2
imageRepository: registry.k8s.io
controlPlaneEndpoint: 10.0.0.10:8443
networking:
  podSubnet: 10.244.0.0/16
`
	var cfg kubeadmClusterConfiguration
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.KubernetesVersion != "v1.28.2" {
		t.Fatalf("kubernetesVersion = %q", cfg.KubernetesVersion)
	}
	if cfg.ImageRepository != "registry.k8s.io" {
		t.Fatalf("imageRepository = %q", cfg.ImageRepository)
	}
	if cfg.ControlPlaneEndpoint != "10.0.0.10:8443" {
		t.Fatalf("controlPlaneEndpoint = %q", cfg.ControlPlaneEndpoint)
	}
}

func TestRandomToken(t *testing.T) {
	for _, length := range []int{6, 16} {
		token, err := randomToken(length)
		if err != nil {
			t.Fatal(err)
		}
		if len(token) != length {
			t.Fatalf("length = %d, want %d", len(token), length)
		}
		for _, ch := range token {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789", ch) {
				t.Fatalf("unexpected character %q in token %q", ch, token)
			}
		}
	}
}

func TestCAHashFromCertData(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "pixiu-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	hash, err := caHashFromCertData(pemData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
	if strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("hash must not contain sha256 prefix: %s", hash)
	}

	if _, err = caHashFromCertData([]byte("not a cert")); err == nil {
		t.Fatal("expected error for invalid cert data")
	}
}

func TestNodeMatched(t *testing.T) {
	names := map[string]struct{}{"worker-1": {}}
	ips := map[string]struct{}{"10.0.0.11": {}}
	if !nodeMatched("other", "10.0.0.11", names, ips) {
		t.Fatal("expected match by ip")
	}
	if !nodeMatched("worker-1", "10.0.0.99", names, ips) {
		t.Fatal("expected match by name")
	}
	if nodeMatched("other", "10.0.0.99", names, ips) {
		t.Fatal("unexpected match")
	}
}
