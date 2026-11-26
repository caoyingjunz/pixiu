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

package sqlaudit

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	auditDetailMax   = 512
	auditSQLTableMax = 3

	maskedCredential = " [敏感内容已隐藏]"
)

// credentialPattern 命中即整条语句隐藏，避免引号、转义与方言差异漏出密码。
var credentialPattern = regexp.MustCompile(`(?i)\b(?:password|passwd|secret|token|identified)\b`)

// token 是词法扫描的一个片段。
// text 保留原文（标识符含引号），lower 仅对未加引号的内容生效用于关键字匹配。
type token struct {
	text        string
	lower       string
	spaceBefore bool
}

// Summary 基于控制器已拆分的语句生成有长度上限的脱敏摘要。
func Summary(statements ...string) string {
	parts := make([]string, 0, len(statements))
	for _, statement := range statements {
		if s := summarizeStatement(statement); s != "" {
			parts = append(parts, s)
		}
	}
	return truncateRunes(strings.Join(parts, "; "), auditDetailMax)
}

func summarizeStatement(sqlText string) string {
	tokens := scan(sqlText)
	if len(tokens) == 0 {
		return ""
	}
	text := render(tokens)
	if credentialPattern.MatchString(text) {
		return truncateRunes(strings.ToUpper(tokens[0].lower)+maskedCredential, auditDetailMax)
	}
	return truncateRunes(collapseValues(tokens), auditDetailMax)
}

// collapseValues 把 VALUES 之后的字面量折叠成 (?)。
func collapseValues(tokens []token) string {
	for i, t := range tokens {
		if t.lower != "values" {
			continue
		}
		if i+1 < len(tokens) && tokens[i+1].text == "(" {
			return render(tokens[:i]) + " VALUES (?)"
		}
		return render(tokens)
	}
	return render(tokens)
}

// render 用单空格还原语句，保持原词之间的分隔关系。
func render(tokens []token) string {
	var b strings.Builder
	for i, t := range tokens {
		if i > 0 && t.spaceBefore {
			b.WriteByte(' ')
		}
		b.WriteString(t.text)
	}
	return b.String()
}

// scan 单次遍历完成去注释、引号保留与空白折叠。
func scan(s string) []token {
	tokens := make([]token, 0, 16)
	space := false
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			space = true
			i++
		case ch == '-' && i+1 < len(s) && s[i+1] == '-', ch == '#':
			i = skipLineComment(s, i)
			space = true
		case ch == '/' && i+1 < len(s) && s[i+1] == '*':
			i, _ = skipBlockComment(s, i)
			space = true
		case isQuote(ch):
			raw, next := readQuoted(s, i)
			tokens = append(tokens, token{text: raw, spaceBefore: space})
			space = false
			i = next
		case isIdentChar(ch):
			j := i
			for j < len(s) && isIdentChar(s[j]) {
				j++
			}
			word := s[i:j]
			tokens = append(tokens, token{text: word, lower: strings.ToLower(word), spaceBefore: space})
			space = false
			i = j
		default:
			tokens = append(tokens, token{text: s[i : i+1], spaceBefore: space})
			space = false
			i++
		}
	}
	return tokens
}

func skipLineComment(s string, i int) int {
	if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
		return i + j + 1
	}
	return len(s)
}

func skipBlockComment(s string, i int) (int, bool) {
	j := strings.Index(s[i+2:], "*/")
	if j < 0 {
		return len(s), false
	}
	return i + j + 4, true
}

// readQuoted 读取一段带引号的内容，返回含引号的原文与下一个位置。
func readQuoted(s string, i int) (string, int) {
	quote := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			if quote != '`' {
				j++
			}
		case quote:
			return s[i : j+1], j + 1
		}
	}
	return s[i:], len(s)
}

func isQuote(ch byte) bool {
	return ch == '\'' || ch == '"' || ch == '`'
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '$'
}

// Tables 从已拆分的变更语句提取最多 3 张表，补齐当前数据库名。
func Tables(database string, statements ...string) string {
	seen := make(map[string]struct{}, auditSQLTableMax)
	tables := make([]string, 0, auditSQLTableMax)
	for _, statement := range statements {
		for _, table := range tablesFromTokens(scan(statement)) {
			key := strings.ToLower(table)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			tables = append(tables, table)
			if len(tables) >= auditSQLTableMax {
				return truncateRunes(qualifyTables(database, tables), 255)
			}
		}
	}
	return truncateRunes(qualifyTables(database, tables), 255)
}

func tablesFromTokens(tokens []token) []string {
	if len(tokens) == 0 {
		return nil
	}
	switch tokens[0].lower {
	case "insert", "replace":
		return singleTable(identAfter(tokens, 0, "into"))
	case "delete":
		return singleTable(identAfter(tokens, 0, "from"))
	case "update":
		// UPDATE [ONLY] [LOW_PRIORITY] [IGNORE] tbl [JOIN tbl2]
		idx := skipKeywords(tokens, 1, "only", "low_priority", "ignore")
		name, next := qualifiedIdent(tokens, idx)
		if name == "" {
			return nil
		}
		tables := []string{name}
		for i := next; i < len(tokens)-1; i++ {
			if tokens[i].lower == "join" {
				if joined, _ := qualifiedIdent(tokens, i+1); joined != "" {
					tables = append(tables, joined)
				}
				break
			}
		}
		return tables
	case "truncate", "rename", "alter":
		name, _ := qualifiedIdent(tokens, skipKeywords(tokens, 1, "table", "only"))
		return singleTable(name)
	case "create":
		idx := skipKeywords(tokens, 1, "or", "replace", "temporary", "temp", "unlogged")
		switch tokens[idx].lower {
		case "table":
			name, _ := qualifiedIdent(tokens, skipKeywords(tokens, idx+1, "if", "not", "exists"))
			return singleTable(name)
		case "unique", "index":
			return singleTable(identAfter(tokens, idx, "on"))
		}
	case "drop":
		idx := 1
		switch tokens[idx].lower {
		case "table":
			name, _ := qualifiedIdent(tokens, skipKeywords(tokens, idx+1, "if", "exists"))
			return singleTable(name)
		case "index":
			return singleTable(identAfter(tokens, idx, "on"))
		}
	}
	return nil
}

func singleTable(name string) []string {
	if name == "" {
		return nil
	}
	return []string{name}
}

// identAfter 返回 keyword 之后紧跟的（可带库名前缀的）标识符。
func identAfter(tokens []token, from int, keyword string) string {
	for i := from; i < len(tokens)-1; i++ {
		if tokens[i].lower == keyword {
			name, _ := qualifiedIdent(tokens, i+1)
			return name
		}
	}
	return ""
}

// skipKeywords 跳过 tokens 开头的保留字，返回第一个有效 token 的下标。
func skipKeywords(tokens []token, from int, keywords ...string) int {
	for from < len(tokens) {
		skipped := false
		for _, keyword := range keywords {
			if tokens[from].lower == keyword {
				from++
				skipped = true
				break
			}
		}
		if !skipped {
			return from
		}
	}
	return from
}

// identAt 返回下标处标识符（去引号），不是标识符时返回空串。
func identAt(tokens []token, i int) string {
	if i < 0 || i >= len(tokens) {
		return ""
	}
	t := tokens[i]
	if t.lower != "" {
		return t.text
	}
	if isQuote(t.text[0]) && len(t.text) >= 2 {
		return t.text[1 : len(t.text)-1]
	}
	return ""
}

// qualifiedIdent 解析 a.b.c 形式的标识符。
func qualifiedIdent(tokens []token, i int) (string, int) {
	name := identAt(tokens, i)
	if name == "" {
		return "", i
	}
	next := i + 1
	for next+1 < len(tokens) && tokens[next].text == "." {
		part := identAt(tokens, next+1)
		if part == "" {
			break
		}
		name += "." + part
		next += 2
	}
	return name, next
}

func qualifyTables(database string, tables []string) string {
	database = strings.TrimSpace(database)
	if database == "" {
		return strings.Join(tables, ", ")
	}
	out := make([]string, 0, len(tables))
	for _, table := range tables {
		if strings.Contains(table, ".") {
			out = append(out, table)
			continue
		}
		out = append(out, database+"."+table)
	}
	return strings.Join(out, ", ")
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
