/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/14/10
 @Desc    : aes cbc 加密解密实现
*/

package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"

	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// TODO: key后期改成从配置文件中读取
var (
	AES_KEY = "KHGSI69YBWGS0TWX"
	AES_IV  = "3010201735544643"
)

func AesCBCDecrypt(decryptText string) (string, error) {
	decode_data, err := base64.StdEncoding.DecodeString(decryptText)
	if err != nil {
		log.Logger.Errorf("the text you are going to decrypt is illegal. error is: %+v", err)
		return "", err
	}
	//生成密码数据块cipher.Block
	block, err := aes.NewCipher([]byte(AES_KEY))
	if err != nil {
		return "", err
	}
	//解密模式
	blockMode := cipher.NewCBCDecrypter(block, []byte(AES_IV))
	//输出到[]byte数组
	origin_data := make([]byte, len(decode_data))
	blockMode.CryptBlocks(origin_data, decode_data)
	//去除填充,并返回
	return string(unpad(origin_data)), nil
}

func AesCBCEncrypt(encryptText string) (string, error) {
	//生成cipher.Block 数据块
	block, err := aes.NewCipher([]byte(AES_KEY))
	if err != nil {
		return "", err
	}
	//填充内容，如果不足16位字符
	blockSize := block.BlockSize()
	originData := pad([]byte(encryptText), blockSize)
	//加密方式
	blockMode := cipher.NewCBCEncrypter(block, []byte(AES_IV))
	//加密，输出到[]byte数组
	crypted := make([]byte, len(originData))
	blockMode.CryptBlocks(crypted, originData)
	return base64.StdEncoding.EncodeToString(crypted), nil
}

func pad(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func unpad(ciphertext []byte) []byte {
	length := len(ciphertext)
	//去掉最后一次的padding
	unpadding := int(ciphertext[length-1])
	return ciphertext[:(length - unpadding)]
}
