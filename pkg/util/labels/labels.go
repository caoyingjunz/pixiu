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

// Package labels 提供平台级标签校验与选择器解析，与资源类型、存储方式无关。
package labels

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxLabels      = 64
	MaxKeyLength   = 63
	MaxValueLength = 128
)

// Operator 标签选择器支持的运算符。
type Operator string

const (
	Equals       Operator = "="
	NotEquals    Operator = "!="
	In           Operator = "in"
	NotIn        Operator = "notin"
	Exists       Operator = "exists"
	DoesNotExist Operator = "!"
)

// Requirement 一条标签筛选条件，多个 Requirement 之间为 AND。
type Requirement struct {
	Key      string
	Operator Operator
	Values   []string
}

var (
	setBasedPattern = regexp.MustCompile(`^([^\s()]+)\s+(in|notin)\s*\((.*)\)$`)
	equalityPattern = regexp.MustCompile(`^([^=!<>]+)(==|!=|=)(.*)$`)
)

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// validPart 校验标签名/值：允许 Unicode 字母数字与 - _ .，首尾须为字母或数字。
// 拒绝空白与逗号、等号等元字符，保证 selector 可无歧义切分、key 可用于 JSON path。
func validPart(s string, allowEmpty bool, maxLen int) error {
	if s == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("must not be empty")
	}
	if n := utf8.RuneCountInString(s); n > maxLen {
		return fmt.Errorf("length %d exceeds limit %d", n, maxLen)
	}
	runes := []rune(s)
	for _, r := range runes {
		if isAlnum(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("contains disallowed character %q", r)
	}
	if !isAlnum(runes[0]) || !isAlnum(runes[len(runes)-1]) {
		return fmt.Errorf("must start and end with a letter or digit")
	}
	return nil
}

// ValidateKey 校验标签键，允许中文。
func ValidateKey(key string) error {
	return validPart(key, false, MaxKeyLength)
}

// ValidateValue 校验标签值，允许空串。
func ValidateValue(value string) error {
	return validPart(value, true, MaxValueLength)
}

// Validate 校验一组标签，是标签合法性的唯一权威。
func Validate(labels map[string]string) error {
	if len(labels) > MaxLabels {
		return fmt.Errorf("too many labels: %d (max %d)", len(labels), MaxLabels)
	}
	for key, value := range labels {
		if err := ValidateKey(key); err != nil {
			return fmt.Errorf("invalid label key %q: %w", key, err)
		}
		if err := ValidateValue(value); err != nil {
			return fmt.Errorf("invalid value for label %q: %w", key, err)
		}
	}
	return nil
}

// Parse 解析标签选择器：全角逗号归一，支持 k=v / k==v / k!=v /
// k in (a,b) / k notin (a,b) / k / !k，逗号分隔的多个条件为 AND。
func Parse(selector string) ([]Requirement, error) {
	selector = strings.TrimSpace(strings.ReplaceAll(selector, "\uFF0C", ","))
	if selector == "" {
		return nil, nil
	}
	parts, err := splitRequirements(selector)
	if err != nil {
		return nil, err
	}
	requirements := make([]Requirement, 0, len(parts))
	for _, part := range parts {
		expr := strings.TrimSpace(part)
		if expr == "" {
			return nil, fmt.Errorf("empty label selector requirement")
		}
		req, err := parseRequirement(expr)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, req)
	}
	return requirements, nil
}

// splitRequirements 按逗号切分，忽略 in/notin 括号内的逗号。
func splitRequirements(selector string) ([]string, error) {
	var parts []string
	depth, start := 0, 0
	for i, r := range selector {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced parenthesis in label selector")
			}
		case ',':
			if depth == 0 {
				parts = append(parts, selector[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parenthesis in label selector")
	}
	return append(parts, selector[start:]), nil
}

func parseRequirement(expr string) (Requirement, error) {
	// !k：标签不存在
	if strings.HasPrefix(expr, "!") {
		key := strings.TrimSpace(strings.TrimPrefix(expr, "!"))
		if err := ValidateKey(key); err != nil {
			return Requirement{}, err
		}
		return Requirement{Key: key, Operator: DoesNotExist}, nil
	}

	// k in (a,b) / k notin (a,b)
	if m := setBasedPattern.FindStringSubmatch(expr); m != nil {
		key, op := m[1], Operator(m[2])
		if err := ValidateKey(key); err != nil {
			return Requirement{}, err
		}
		if strings.TrimSpace(m[3]) == "" {
			return Requirement{}, fmt.Errorf("empty value set for label %q", key)
		}
		var values []string
		for _, value := range strings.Split(m[3], ",") {
			value = strings.TrimSpace(value)
			if err := ValidateValue(value); err != nil {
				return Requirement{}, fmt.Errorf("invalid value for label %q: %w", key, err)
			}
			values = append(values, value)
		}
		return Requirement{Key: key, Operator: op, Values: values}, nil
	}

	// k=v / k==v / k!=v
	if m := equalityPattern.FindStringSubmatch(expr); m != nil {
		key := strings.TrimSpace(m[1])
		value := strings.TrimSpace(m[3])
		if err := ValidateKey(key); err != nil {
			return Requirement{}, err
		}
		if err := ValidateValue(value); err != nil {
			return Requirement{}, fmt.Errorf("invalid value for label %q: %w", key, err)
		}
		op := Equals
		if m[2] == "!=" {
			op = NotEquals
		}
		return Requirement{Key: key, Operator: op, Values: []string{value}}, nil
	}

	// k：标签存在
	if err := ValidateKey(expr); err != nil {
		return Requirement{}, fmt.Errorf("invalid label selector %q: %w", expr, err)
	}
	return Requirement{Key: expr, Operator: Exists}, nil
}
