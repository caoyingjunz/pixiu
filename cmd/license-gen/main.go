/*
Copyright 2021 The Pixiu Authors.

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

// license-gen Pixiu 商业 License 签发工具（仅签发方持有，不得随产品分发）。
//
// 子命令：
//
//	keygen  生成 Ed25519 密钥对（私钥务必妥善保管，公钥需更新到 pkg/license/publickey.go 重新编译）
//	sign    对 license 载荷 JSON 签名，产出 license.key
//	verify  校验 license.key 并打印载荷内容
//	print   打印本机机器指纹（签发绑定环境时使用）
//
// 用法示例见 docs/license.md。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/caoyingjunz/pixiu/pkg/license"
)

func main() {
	root := &cobra.Command{
		Use:   "license-gen",
		Short: "Pixiu 商业 License 签发工具",
	}
	root.AddCommand(newKeygenCommand(), newSignCommand(), newVerifyCommand(), newPrintCommand())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newKeygenCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "生成 Ed25519 密钥对",
		Run: func(cmd *cobra.Command, args []string) {
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				fatal("生成密钥失败: %v", err)
			}
			if err = os.MkdirAll(dir, 0o700); err != nil {
				fatal("创建目录失败: %v", err)
			}
			pubFile := filepath.Join(dir, "ed25519_public.key")
			privFile := filepath.Join(dir, "ed25519_private.key")
			if err = os.WriteFile(pubFile, []byte(hex.EncodeToString(pub)), 0o644); err != nil {
				fatal("写公钥失败: %v", err)
			}
			if err = os.WriteFile(privFile, []byte(hex.EncodeToString(priv)), 0o600); err != nil {
				fatal("写私钥失败: %v", err)
			}
			fmt.Printf("密钥对已生成:\n  公钥: %s（内容更新到 pkg/license/publickey.go 后重新编译 pixiu-server）\n  私钥: %s（仅签发方持有，严禁泄露）\n",
				pubFile, privFile)
			fmt.Printf("公钥 hex:\n%s\n", hex.EncodeToString(pub))
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "./license-keys", "密钥输出目录")
	return cmd
}

func newSignCommand() *cobra.Command {
	var (
		keyFile  string
		inFile   string
		outFile  string
		duration time.Duration
	)
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "对 license 载荷 JSON 签名，产出 license.key",
		Run: func(cmd *cobra.Command, args []string) {
			raw, err := os.ReadFile(inFile)
			if err != nil {
				fatal("读取载荷失败: %v", err)
			}
			var payload license.Payload
			if err = json.Unmarshal(raw, &payload); err != nil {
				fatal("解析载荷 JSON 失败: %v", err)
			}
			// 允许用 -duration 快速设置有效期，覆盖载荷中的 expires_at
			if duration > 0 {
				payload.ExpiresAt = time.Now().Add(duration).Unix()
			}
			payload.IssuedAt = time.Now().Unix()

			keyRaw, err := os.ReadFile(keyFile)
			if err != nil {
				fatal("读取私钥失败: %v", err)
			}
			priv, err := license.ParsePrivateKey(string(keyRaw))
			if err != nil {
				fatal("解析私钥失败: %v", err)
			}
			signed, err := license.SignPayload(priv, &payload)
			if err != nil {
				fatal("签名失败: %v", err)
			}
			if err = os.WriteFile(outFile, signed, 0o644); err != nil {
				fatal("写出 license 失败: %v", err)
			}
			fmt.Printf("license 已签发: %s\n载荷摘要: customer=%s edition=%s expires_at=%s\n",
				outFile, payload.Customer, payload.Edition, formatUnix(payload.ExpiresAt))
		},
	}
	cmd.Flags().StringVar(&keyFile, "key", "license-keys/ed25519_private.key", "私钥文件路径")
	cmd.Flags().StringVar(&inFile, "in", "license.json", "载荷 JSON 文件")
	cmd.Flags().StringVar(&outFile, "out", "license.key", "签发结果文件")
	cmd.Flags().DurationVar(&duration, "duration", 0, "有效期（如 8760h 为一年），设置后覆盖载荷中的 expires_at")
	return cmd
}

func newVerifyCommand() *cobra.Command {
	var (
		pubFile string
		inFile  string
	)
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "校验 license.key 并打印载荷内容",
		Run: func(cmd *cobra.Command, args []string) {
			pubRaw, err := os.ReadFile(pubFile)
			if err != nil {
				fatal("读取公钥失败: %v", err)
			}
			pub, err := license.ParsePublicKey(string(pubRaw))
			if err != nil {
				fatal("解析公钥失败: %v", err)
			}
			data, err := os.ReadFile(inFile)
			if err != nil {
				fatal("读取 license 失败: %v", err)
			}
			payload, err := license.VerifyLicense(pub, data)
			if err != nil {
				fatal("校验失败: %v", err)
			}
			out, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Printf("签名验证通过，载荷内容:\n%s\n", out)
			fmt.Printf("过期时间: %s\n", formatUnix(payload.ExpiresAt))
		},
	}
	cmd.Flags().StringVar(&pubFile, "pub", "license-keys/ed25519_public.key", "公钥文件路径")
	cmd.Flags().StringVar(&inFile, "in", "license.key", "license 文件路径")
	return cmd
}

func newPrintCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "打印本机机器指纹（签发绑定环境时使用）",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(license.Fingerprint())
		},
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}

func formatUnix(ts int64) string {
	if ts == 0 {
		return "永不过期"
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}
