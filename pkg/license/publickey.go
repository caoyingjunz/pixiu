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

import "crypto/ed25519"

// EmbeddedPublicKey 内嵌在服务端二进制中的 license 验签公钥（Ed25519，hex 编码）。
//
// 更换密钥对流程（见 docs/license.md）：
//  1. 运行 `go run ./cmd/license-gen keygen` 生成新密钥对；
//  2. 将输出的公钥 hex 替换下方常量并重新编译 pixiu-server；
//  3. 新私钥仅签发方持有，旧 license 将全部失效，需重新签发。
//
// 当前为开发默认密钥对，正式商用前务必重新生成！
const embeddedPublicKeyHex = "9ddea89274887178c77c2cff99f8057febf8a7523a5735733f35122cf52f75a7"

// EmbeddedPublicKey 解析后的验签公钥
var EmbeddedPublicKey = mustParsePublicKey(embeddedPublicKeyHex)

func mustParsePublicKey(hexKey string) ed25519.PublicKey {
	pub, err := ParsePublicKey(hexKey)
	if err != nil {
		// 公钥在编译期内嵌，解析失败属于程序缺陷，直接 panic 暴露问题
		panic("license: 内嵌公钥解析失败: " + err.Error())
	}
	return pub
}
