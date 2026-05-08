/*
Copyright 2024 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package ssh

import (
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// NormalizePrivateKeyPEM 将库表/JSON 中常见的私钥字符串整理为 PEM，便于 encoding/pem 与 ssh.ParsePrivateKey 解析。
// 处理：首尾空白、BOM、字面量 \n、\r、END 行后误带的 }、仅截取 BEGIN…END 块等。
func NormalizePrivateKeyPEM(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.ReplaceAll(s, "\\r\\n", "\n")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// 常见误写：END 行末尾多一个 }
	s = strings.ReplaceAll(s, "-----END RSA PRIVATE KEY-----}", "-----END RSA PRIVATE KEY-----")
	s = strings.ReplaceAll(s, "-----END OPENSSH PRIVATE KEY-----}", "-----END OPENSSH PRIVATE KEY-----")
	s = strings.ReplaceAll(s, "-----END EC PRIVATE KEY-----}", "-----END EC PRIVATE KEY-----")
	s = strings.ReplaceAll(s, "-----END DSA PRIVATE KEY-----}", "-----END DSA PRIVATE KEY-----")
	s = strings.ReplaceAll(s, "-----END PRIVATE KEY-----}", "-----END PRIVATE KEY-----")

	begin := strings.Index(s, "-----BEGIN")
	if begin < 0 {
		return s
	}
	sub := s[begin:]
	lines := strings.Split(sub, "\n")
	var b strings.Builder
	in := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		t = strings.TrimSuffix(t, "}")
		if strings.HasPrefix(t, "-----BEGIN") {
			in = true
		}
		if !in {
			continue
		}
		b.WriteString(t)
		b.WriteByte('\n')
		if strings.HasPrefix(t, "-----END") {
			break
		}
	}
	out := strings.TrimSuffix(b.String(), "\n")
	if out != "" {
		return out
	}
	return sub
}

// repairFirstBase64LineOfPEM 若 PEM 首段 base64 行前带有非 PEM 字符（如误插入的 "1"），去掉直至合法 base64 字母。
func repairFirstBase64LineOfPEM(pem string) string {
	lines := strings.Split(pem, "\n")
	beginIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "-----BEGIN") {
			beginIdx = i
			break
		}
	}
	if beginIdx < 0 || beginIdx+1 >= len(lines) {
		return pem
	}
	bodyIdx := beginIdx + 1
	line := strings.TrimSpace(lines[bodyIdx])
	if line == "" || strings.HasPrefix(line, "-----") {
		return pem
	}
	const base64Set = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	j := 0
	for j < len(line) {
		if strings.ContainsRune(base64Set, rune(line[j])) {
			break
		}
		j++
	}
	if j == 0 {
		return pem
	}
	lines[bodyIdx] = line[j:]
	return strings.Join(lines, "\n")
}

// repairLeadingDigitBeforeMII 修复首行 base64 前误插入的单个数字（如 "1MII..."，1 属 base64 字母集，不会被 repairFirstBase64LineOfPEM 去掉）。
func repairLeadingDigitBeforeMII(pem string) string {
	lines := strings.Split(pem, "\n")
	beginIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "-----BEGIN") {
			beginIdx = i
			break
		}
	}
	if beginIdx < 0 || beginIdx+1 >= len(lines) {
		return pem
	}
	bodyIdx := beginIdx + 1
	line := strings.TrimSpace(lines[bodyIdx])
	if line == "" || strings.HasPrefix(line, "-----") {
		return pem
	}
	if len(line) >= 4 && line[0] >= '0' && line[0] <= '9' && strings.HasPrefix(line[1:], "MII") {
		lines[bodyIdx] = line[1:]
		return strings.Join(lines, "\n")
	}
	return pem
}

// ParsePrivateKeySigner 解析 PEM 私钥（含库表常见脏数据修复），供 SSH 客户端使用。
func ParsePrivateKeySigner(raw string) (ssh.Signer, error) {
	try := func(pemStr string) (ssh.Signer, error) {
		block, _ := pem.Decode([]byte(pemStr))
		if block == nil {
			return nil, fmt.Errorf("no PEM block found")
		}
		return ssh.ParsePrivateKey(pem.EncodeToMemory(block))
	}

	s := NormalizePrivateKeyPEM(raw)
	variants := []string{s}
	add := func(v string) {
		for _, x := range variants {
			if x == v {
				return
			}
		}
		variants = append(variants, v)
	}
	if v := repairFirstBase64LineOfPEM(s); v != s {
		add(v)
	}
	if v := repairLeadingDigitBeforeMII(s); v != s {
		add(v)
	}
	if v := repairLeadingDigitBeforeMII(repairFirstBase64LineOfPEM(s)); v != s {
		add(v)
	}
	if v := repairFirstBase64LineOfPEM(repairLeadingDigitBeforeMII(s)); v != s {
		add(v)
	}

	var lastErr error
	for _, pemStr := range variants {
		signer, err := try(pemStr)
		if err == nil {
			return signer, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("parse private key: %w", lastErr)
	}
	return nil, fmt.Errorf("parse private key: no PEM block found")
}
