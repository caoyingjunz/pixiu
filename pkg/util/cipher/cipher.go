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

/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/14/10
 @Desc    : aes cbc 加密解密实现
*/

package cipher

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
)

// TODO: key后期改成从配置文件中读取
var (
	AES_KEY = "KHGSI69YBWGS0TWX"
	AES_IV  = "3010201735544643"
)

func Encrypt(data []byte) (string, error) {
	//生成 cipher.Block 数据块
	block, err := aes.NewCipher([]byte(AES_KEY))
	if err != nil {
		return "", err
	}
	//填充内容，如果不足16位字符
	blockSize := block.BlockSize()
	originData := pad(data, blockSize)
	//加密方式
	blockMode := cipher.NewCBCEncrypter(block, []byte(AES_IV))
	//加密，输出到[]byte数组
	crypted := make([]byte, len(originData))
	blockMode.CryptBlocks(crypted, originData)
	return base64.StdEncoding.EncodeToString(crypted), nil
}

func Decrypt(decryptText string) ([]byte, error) {
	decodeData, err := base64.StdEncoding.DecodeString(decryptText)
	if err != nil {
		return nil, err
	}
	//生成密码数据块cipher.Block
	block, err := aes.NewCipher([]byte(AES_KEY))
	if err != nil {
		return nil, err
	}
	//解密模式
	blockMode := cipher.NewCBCDecrypter(block, []byte(AES_IV))
	//输出到[]byte数组
	originData := make([]byte, len(decodeData))
	blockMode.CryptBlocks(originData, decodeData)
	//去除填充,并返回
	return unPad(originData), nil
}

func pad(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func unPad(cipherText []byte) []byte {
	length := len(cipherText)
	//去掉最后一次的padding
	unPadding := int(cipherText[length-1])
	return cipherText[:(length - unPadding)]
}
