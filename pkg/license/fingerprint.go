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

package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Fingerprint 计算部署环境的机器指纹。
// 取值优先级：
//  1. /etc/machine-id（主流 Linux 发行版）
//  2. /var/lib/dbus/machine-id（部分发行版）
//  3. IOPlatformUUID（macOS）
//  4. 主机名（兜底，弱绑定；主机名易变，仅在没有 machine-id 的环境使用）
//
// 返回 machine-id 的 SHA-256 hex（前 16 字节，32 个 hex 字符），用于签发时绑定部署环境；
// 空指纹的 license 不绑定机器。结果进程内缓存一次，避免在热路径反复读文件/exec。
func Fingerprint() string {
	fingerprintOnce.Do(func() { fingerprintValue = detectFingerprint() })
	return fingerprintValue
}

var (
	fingerprintOnce  sync.Once
	fingerprintValue string
)

// detectFingerprint 实际探测机器标识，仅在首次调用 Fingerprint 时执行。
func detectFingerprint() string {
	if id, err := os.ReadFile("/etc/machine-id"); err == nil {
		if s := strings.TrimSpace(string(id)); s != "" {
			return hashFingerprint(s)
		}
	}
	if id, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		if s := strings.TrimSpace(string(id)); s != "" {
			return hashFingerprint(s)
		}
	}
	if out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "IOPlatformUUID") {
				if idx := strings.Index(line, "\" = \""); idx >= 0 {
					value := strings.TrimRight(line[idx+len("\" = \""):], "\"")
					if value != "" {
						return hashFingerprint(value)
					}
				}
			}
		}
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hashFingerprint(hostname)
	}
	return ""
}

// hashFingerprint 对机器标识做单向哈希，避免原始 machine-id 直接出现在
// API 响应/日志中（固定域分隔前缀仅用于区分用途，不是机密盐，不用于对抗离线爆破）。
func hashFingerprint(machineID string) string {
	sum := sha256.Sum256([]byte("pixiu-license:" + machineID))
	return hex.EncodeToString(sum[:16])
}
