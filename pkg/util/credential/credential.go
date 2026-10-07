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

package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const prefix = "ENC:v1:"

// Encrypt encrypts a credential with AES-256-GCM.
func Encrypt(plaintext, key string) (string, error) {
	if plaintext == "" {
		return plaintext, nil
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("credential encryption key is not configured")
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}

	block, err := newBlock(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create credential cipher: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate credential nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	encoded := base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...))
	return prefix + encoded, nil
}

// Decrypt decrypts a credential. Values without the ENC:v1: prefix are legacy plaintext.
func Decrypt(value, key string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	if key == "" {
		return "", errors.New("credential encryption key is not configured")
	}

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted credential: %w", err)
	}
	block, err := newBlock(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create credential cipher: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("encrypted credential is too short")
	}
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}
	return string(plaintext), nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

func newBlock(key string) (cipher.Block, error) {
	digest := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, fmt.Errorf("create credential cipher key: %w", err)
	}
	return block, nil
}
