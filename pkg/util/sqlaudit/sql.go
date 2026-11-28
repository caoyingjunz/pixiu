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

// credentialPattern 命中即整条语句隐藏，覆盖折叠 `=` 无法处理的凭据语法（如 IDENTIFIED BY）。
// 敏感词可作标识符前缀（secret_col、api_key）但后接字母不算；不收录 token，避免 token_count 误伤。
var credentialPattern = regexp.MustCompile(
	`(?i)(^|[^a-z0-9])(password|passwd|pwd|secret|identified|credential|api_?key|access_?key|private_?key)($|[^a-z])`)

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
	if credentialPattern.MatchString(identifiers(tokens)) {
		return truncateRunes(strings.ToUpper(tokens[0].lower)+maskedCredential, auditDetailMax)
	}
	return truncateRunes(collapseValues(collapseAssignments(tokens)), auditDetailMax)
}

// identifiers 拼接非字面量 token，列值里的 api_key 不应触发整条隐藏。
func identifiers(tokens []token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.text == "" {
			continue
		}
		switch t.text[0] {
		case '\'', '"':
			continue
		case '`':
			b.WriteString(strings.ToLower(strings.Trim(t.text, "`")))
		default:
			b.WriteString(t.lower)
		}
		b.WriteByte(' ')
	}
	return b.String()
}

// collapseAssignments 把 `= 字面量` 折叠成 `= ?`，避免 UPDATE 的字面量原文落库。
// 字符串一律折叠；数字只在 SET 子句内折叠，保留 WHERE id=5 便于定位。
func collapseAssignments(tokens []token) []token {
	setFrom, setTo := setClauseRange(tokens)
	out := make([]token, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		out = append(out, tokens[i])
		if tokens[i].text != "=" || i+1 >= len(tokens) || !isLiteralToken(tokens[i+1]) {
			continue
		}
		if isNumericToken(tokens[i+1].lower) && (i < setFrom || i > setTo) {
			continue
		}
		out = append(out, token{text: "?", spaceBefore: tokens[i+1].spaceBefore})
		i++
	}
	return out
}

// setClauseRange 返回 SET 子句的下标闭区间，没有 SET 时返回 (-1, -1)。
func setClauseRange(tokens []token) (int, int) {
	start := -1
	for i, t := range tokens {
		if t.lower == "set" {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	for i := start + 1; i < len(tokens); i++ {
		switch tokens[i].lower {
		case "where", "order", "limit", "returning":
			return start, i - 1
		}
	}
	return start, len(tokens) - 1
}

// isLiteralToken 判断是否为字符串或数字字面量（反引号是标识符）。
func isLiteralToken(t token) bool {
	if t.text == "" {
		return false
	}
	if t.text[0] == '\'' || t.text[0] == '"' {
		return true
	}
	return isNumericToken(t.lower)
}

func isNumericToken(s string) bool {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9', c == '.', c == 'e', c == 'E', c == '+', c == '-':
		default:
			return false
		}
	}
	return true
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
			// '' 与 "" 是引号转义，字面量未结束
			if quote != '`' && j+1 < len(s) && s[j+1] == quote {
				j++
				continue
			}
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
		// CREATE 之后没有有效 token 时无可解析对象
		if idx >= len(tokens) {
			return nil
		}
		switch tokens[idx].lower {
		case "table":
			name, _ := qualifiedIdent(tokens, skipKeywords(tokens, idx+1, "if", "not", "exists"))
			return singleTable(name)
		case "unique", "index":
			return singleTable(identAfter(tokens, idx, "on"))
		}
	case "drop":
		if len(tokens) < 2 {
			return nil
		}
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
