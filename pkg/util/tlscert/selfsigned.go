/*
Copyright 2024 The Pixiu Authors.

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

package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureSelfSigned 若 cert/key 不存在则生成自签证书（供本地 kubectl 经 HTTPS 访问）。
func EnsureSelfSigned(certFile, keyFile string, hosts []string) error {
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("cert_file and key_file are required")
	}
	if fileExists(certFile) && fileExists(keyFile) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(certFile), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o755); err != nil {
		return err
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"Pixiu"}, CommonName: "pixiu-kube-gateway"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	seenDNS := map[string]struct{}{"localhost": {}}
	seenIP := map[string]struct{}{}
	for _, h := range hosts {
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			if _, ok := seenIP[ip.String()]; ok {
				continue
			}
			seenIP[ip.String()] = struct{}{}
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
			continue
		}
		if _, ok := seenDNS[h]; ok {
			continue
		}
		seenDNS[h] = struct{}{}
		tmpl.DNSNames = append(tmpl.DNSNames, h)
	}
	if ip := net.ParseIP("127.0.0.1"); ip != nil {
		if _, ok := seenIP[ip.String()]; !ok {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
