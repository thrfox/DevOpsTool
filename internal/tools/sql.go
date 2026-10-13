package tools

import (
	"fmt"
	"strings"
	"unicode"
)

// SQL 格式化采用词法分段，保留字符串、注释和方言标识符，不改写查询语义。
func FormatSQL(text string) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	tokens, err := sqlTokens(text)
	if err != nil {
		return "", err
	}
	clauses := map[string]bool{"SELECT": true, "FROM": true, "WHERE": true, "GROUP BY": true, "ORDER BY": true, "HAVING": true, "LIMIT": true, "OFFSET": true, "UNION": true, "UNION ALL": true, "INSERT INTO": true, "VALUES": true, "UPDATE": true, "SET": true, "DELETE FROM": true, "LEFT JOIN": true, "RIGHT JOIN": true, "INNER JOIN": true, "FULL JOIN": true, "CROSS JOIN": true, "JOIN": true, "ON": true, "RETURNING": true, "WITH": true}
	keywords := map[string]bool{"AS": true, "AND": true, "OR": true, "NOT": true, "NULL": true, "IS": true, "IN": true, "LIKE": true, "DISTINCT": true, "ASC": true, "DESC": true, "CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true, "TRUE": true, "FALSE": true}
	var out strings.Builder
	depth := 0
	lineStart := true
	previous := ""
	newline := func() {
		if !lineStart {
			out.WriteByte('\n')
		}
		lineStart = true
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		u := strings.ToUpper(t)
		if i+1 < len(tokens) {
			pair := u + " " + strings.ToUpper(tokens[i+1])
			if clauses[pair] {
				t = pair
				u = pair
				i++
			}
		}
		if t == ")" {
			depth--
			if depth < 0 {
				return "", fmt.Errorf("SQL 括号不匹配")
			}
		}
		if clauses[u] || u == "AND" || u == "OR" {
			newline()
			t = u
		} else if keywords[u] {
			t = u
		}
		if lineStart {
			out.WriteString(strings.Repeat("  ", depth))
			lineStart = false
		} else if t != "," && t != ";" && t != ")" && t != "." && previous != "(" && previous != "." {
			out.WriteByte(' ')
		}
		out.WriteString(t)
		if t == "(" {
			depth++
		}
		if t == ";" || strings.HasPrefix(t, "--") || strings.HasPrefix(t, "#") {
			newline()
		}
		previous = t
	}
	if depth != 0 {
		return "", fmt.Errorf("SQL 括号不匹配")
	}
	return strings.TrimSpace(out.String()), nil
}

func sqlTokens(text string) ([]string, error) {
	r := []rune(text)
	var tokens []string
	for i := 0; i < len(r); {
		if unicode.IsSpace(r[i]) {
			i++
			continue
		}
		start := i
		// 保留 SQL Server / PostgreSQL 的 Unicode、转义及位串字面量前缀。
		if strings.ContainsRune("nNeExXbB", r[i]) && i+1 < len(r) && r[i+1] == '\'' {
			i++
		}
		if (r[i] == 'u' || r[i] == 'U') && i+2 < len(r) && r[i+1] == '&' && r[i+2] == '\'' {
			i += 2
		}
		switch {
		case r[i] == '\'' || r[i] == '"' || r[i] == '`' || r[i] == '[':
			end := r[i]
			if end == '[' {
				end = ']'
			}
			i++
			closed := false
			for i < len(r) {
				if r[i] == end {
					i++
					if i < len(r) && r[i] == end {
						i++
						continue
					}
					closed = true
					break
				}
				// 保留反斜线转义，兼容常见 MySQL 字符串。
				if r[i] == '\\' && i+1 < len(r) {
					i += 2
				} else {
					i++
				}
			}
			if !closed {
				return nil, fmt.Errorf("SQL 字符串或标识符未闭合")
			}
		case (r[i] == '#' && !(i+1 < len(r) && r[i+1] == '>')) || (r[i] == '-' && i+1 < len(r) && r[i+1] == '-'):
			for i < len(r) && r[i] != '\n' {
				i++
			}
		case r[i] == '/' && i+1 < len(r) && r[i+1] == '*':
			i += 2
			closed := false
			for i+1 < len(r) {
				if r[i] == '*' && r[i+1] == '/' {
					i += 2
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("SQL 注释未闭合")
			}
		case r[i] == '$':
			j := i + 1
			for j < len(r) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_') {
				j++
			}
			if j < len(r) && r[j] == '$' {
				tag := string(r[i : j+1])
				rest := string(r[j+1:])
				end := strings.Index(rest, tag)
				if end < 0 {
					return nil, fmt.Errorf("SQL 美元引号未闭合")
				}
				i = j + 1 + len([]rune(rest[:end])) + len([]rune(tag))
			} else {
				i = j
			}
		case (r[i] == '@' || r[i] == ':') && i+1 < len(r) && (unicode.IsLetter(r[i+1]) || r[i+1] == '_' || r[i+1] == '@'):
			i++
			for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_' || r[i] == '@') {
				i++
			}
		case unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_':
			i++
			for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_' || r[i] == '$') {
				i++
			}
		default:
			i++
			// 多字符运算符必须作为一个词保留，避免把 ::、->> 等拆成不同语义。
			if i < len(r) {
				for _, op := range []string{"->>", "#>>", "::", "->", "#>", "<=", ">=", "<>", "!=", "||", "&&", "<<", ">>", ":="} {
					if strings.HasPrefix(string(r[start:min(len(r), start+3)]), op) {
						i = start + len([]rune(op))
						break
					}
				}
			}
		}
		tokens = append(tokens, string(r[start:i]))
	}
	return tokens, nil
}
