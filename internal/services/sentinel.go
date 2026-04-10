// Package services — Sentinel: dynamic security expression engine for Cocobase.
//
// Sentinels let you write backend security rules evaluated per-request.
//
// ─── Special shorthands ───────────────────────────────────────────────────────
//
//	$authenticated          — true if any user is logged in (most common gate)
//	$unauthenticated        — true if request has no auth token
//	$admin                  — true if user has role "admin"
//	$owner                  — true if $req.user.id == $doc.owner_id (common pattern)
//	$verified               — true if user's email is verified
//
// ─── Variables ────────────────────────────────────────────────────────────────
//
//	$req.user               — the authenticated user object (null if not logged in)
//	$req.user.id            — user's ID
//	$req.user.email         — user's email
//	$req.user.roles         — user's roles (string array)
//	$req.user.verified      — whether user's email is verified
//	$req.user.data.<field>  — custom field from user.Data
//	$req.user.<field>       — shorthand for $req.user.data.<field>
//	$doc.<field>            — field from the document (nil for pre-create checks)
//	$req.ip                 — client IP address
//	$req.method             — HTTP method (GET, POST, PATCH, DELETE)
//
// ─── Operators ────────────────────────────────────────────────────────────────
//
//	== != < > <= >=         — comparison (loose string fallback)
//	&& ||                   — logical and/or
//	!                       — logical not
//	contains                — string contains substring, OR array contains value
//	startswith              — string starts with prefix
//	endswith                — string ends with suffix
//	in                      — value is in a list: $req.user.id in [$doc.editors]
//	matches                 — simple glob match (* = anything)
//	exists                  — field is not null/empty: exists $doc.avatar_url
//
// ─── Examples ─────────────────────────────────────────────────────────────────
//
//	$authenticated
//	$admin || $owner
//	$req.user.id == $doc.owner_id
//	$req.user.roles contains "editor"
//	$doc.published == true || $req.user.id == $doc.author_id
//	$req.user.plan == "pro" && $doc.team_id == $req.user.team_id
//	$req.user.data.age >= 18
//	exists $doc.approved_at
//	$req.user.email endswith "@acme.com"
package services

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/patrick/cocobase/internal/models"
)

// SentinelContext holds the runtime variables for a sentinel expression.
type SentinelContext struct {
	User   *models.AppUser        // nil if unauthenticated
	Doc    map[string]interface{} // document fields (nil for pre-create)
	IP     string                 // client IP
	Method string                 // HTTP method
}

// EvalSentinel evaluates a sentinel expression string.
// Empty expression → true (no restriction).
// Parse/eval error → false (fail-closed, access denied).
func EvalSentinel(expr string, ctx SentinelContext) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}

	tokens := tokenize(expr)
	parser := &sentinelParser{tokens: tokens, ctx: ctx}
	result, err := parser.parseOr()
	if err != nil {
		return false, fmt.Errorf("sentinel: %w", err)
	}
	return isTruthy(result), nil
}

// ─── Tokenizer ────────────────────────────────────────────────────────────────

type stok struct {
	kind  string // ident | str | num | op | bool | lparen | rparen
	value string
}

func tokenize(expr string) []stok {
	var tokens []stok
	i := 0
	for i < len(expr) {
		ch := expr[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}
		// String literal — both " and '
		if ch == '"' || ch == '\'' {
			quote := ch
			j := i + 1
			for j < len(expr) && expr[j] != quote {
				if expr[j] == '\\' {
					j++ // skip escape
				}
				j++
			}
			tokens = append(tokens, stok{kind: "str", value: expr[i+1 : j]})
			i = j + 1
			continue
		}
		// Two-char operators
		if i+1 < len(expr) {
			two := expr[i : i+2]
			switch two {
			case "==", "!=", "<=", ">=", "&&", "||":
				tokens = append(tokens, stok{kind: "op", value: two})
				i += 2
				continue
			}
		}
		// Single-char
		switch ch {
		case '<', '>', '!':
			tokens = append(tokens, stok{kind: "op", value: string(ch)})
			i++
			continue
		case '(':
			tokens = append(tokens, stok{kind: "lparen"})
			i++
			continue
		case ')':
			tokens = append(tokens, stok{kind: "rparen"})
			i++
			continue
		case '[':
			tokens = append(tokens, stok{kind: "lbracket"})
			i++
			continue
		case ']':
			tokens = append(tokens, stok{kind: "rbracket"})
			i++
			continue
		case ',':
			tokens = append(tokens, stok{kind: "comma"})
			i++
			continue
		}
		// Number
		if ch >= '0' && ch <= '9' || (ch == '-' && i+1 < len(expr) && expr[i+1] >= '0' && expr[i+1] <= '9') {
			j := i
			if expr[j] == '-' {
				j++
			}
			for j < len(expr) && (expr[j] >= '0' && expr[j] <= '9' || expr[j] == '.') {
				j++
			}
			tokens = append(tokens, stok{kind: "num", value: expr[i:j]})
			i = j
			continue
		}
		// Identifier / keyword / variable
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' || ch == '$' {
			j := i
			for j < len(expr) && (expr[j] >= 'a' && expr[j] <= 'z' ||
				expr[j] >= 'A' && expr[j] <= 'Z' ||
				expr[j] >= '0' && expr[j] <= '9' ||
				expr[j] == '_' || expr[j] == '.' || expr[j] == '$') {
				j++
			}
			word := expr[i:j]
			switch word {
			case "true":
				tokens = append(tokens, stok{kind: "bool", value: "true"})
			case "false":
				tokens = append(tokens, stok{kind: "bool", value: "false"})
			case "null", "nil", "undefined":
				tokens = append(tokens, stok{kind: "null"})
			case "contains", "startswith", "endswith", "in", "matches", "exists", "not":
				tokens = append(tokens, stok{kind: "op", value: word})
			default:
				tokens = append(tokens, stok{kind: "ident", value: word})
			}
			i = j
			continue
		}
		i++ // skip unknown
	}
	return tokens
}

// ─── Parser ───────────────────────────────────────────────────────────────────

type sentinelParser struct {
	tokens []stok
	pos    int
	ctx    SentinelContext
}

func (p *sentinelParser) peek() *stok {
	if p.pos >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.pos]
}

func (p *sentinelParser) consume() stok {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

// parseOr handles ||
func (p *sentinelParser) parseOr() (interface{}, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t == nil || !(t.kind == "op" && t.value == "||") {
			break
		}
		p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = isTruthy(left) || isTruthy(right)
	}
	return left, nil
}

// parseAnd handles &&
func (p *sentinelParser) parseAnd() (interface{}, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t == nil || !(t.kind == "op" && t.value == "&&") {
			break
		}
		p.consume()
		right, err := p.parsePrefix()
		if err != nil {
			return nil, err
		}
		left = isTruthy(left) && isTruthy(right)
	}
	return left, nil
}

// parsePrefix handles prefix ops: ! and exists
func (p *sentinelParser) parsePrefix() (interface{}, error) {
	t := p.peek()
	if t == nil {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	if t.kind == "op" && t.value == "!" {
		p.consume()
		val, err := p.parsePrefix()
		if err != nil {
			return nil, err
		}
		return !isTruthy(val), nil
	}
	// exists <expr> — checks that the value is not null/empty
	if t.kind == "op" && t.value == "exists" {
		p.consume()
		val, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return isTruthy(val), nil
	}
	return p.parseComparison()
}

// parseComparison handles ==, !=, <, >, <=, >=, contains, startswith, endswith, in, matches
func (p *sentinelParser) parseComparison() (interface{}, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	t := p.peek()
	if t == nil || t.kind != "op" {
		return left, nil
	}
	op := t.value
	validOps := map[string]bool{
		"==": true, "!=": true, "<": true, ">": true, "<=": true, ">=": true,
		"contains": true, "startswith": true, "endswith": true, "in": true, "matches": true,
	}
	if !validOps[op] {
		return left, nil
	}
	p.consume()

	right, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	switch op {
	case "==":
		return compareEqual(left, right), nil
	case "!=":
		return !compareEqual(left, right), nil
	case "contains":
		return sentinelContains(left, right), nil
	case "startswith":
		ls, rs := fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)
		return strings.HasPrefix(ls, rs), nil
	case "endswith":
		ls, rs := fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)
		return strings.HasSuffix(ls, rs), nil
	case "in":
		// value in array — right should be a list literal
		return sentinelIn(left, right), nil
	case "matches":
		ls, rs := fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)
		return globMatch(rs, ls), nil
	case "<", ">", "<=", ">=":
		return compareNumeric(left, op, right)
	}
	return false, nil
}

// parsePrimary handles literals, shorthands, variables, lists, and parenthesised exprs
func (p *sentinelParser) parsePrimary() (interface{}, error) {
	t := p.peek()
	if t == nil {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	// Parenthesised expression
	if t.kind == "lparen" {
		p.consume()
		val, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if r := p.peek(); r != nil && r.kind == "rparen" {
			p.consume()
		}
		return val, nil
	}

	// List literal: [val1, val2, ...]
	if t.kind == "lbracket" {
		p.consume()
		var items []interface{}
		for {
			end := p.peek()
			if end == nil || end.kind == "rbracket" {
				if end != nil {
					p.consume()
				}
				break
			}
			if end.kind == "comma" {
				p.consume()
				continue
			}
			item, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	}

	p.consume()

	switch t.kind {
	case "bool":
		return t.value == "true", nil
	case "null":
		return nil, nil
	case "num":
		f, err := strconv.ParseFloat(t.value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", t.value)
		}
		return f, nil
	case "str":
		return t.value, nil
	case "ident":
		return p.resolveIdent(t.value)
	}
	return nil, fmt.Errorf("unexpected token %q", t.value)
}

// resolveIdent resolves all variable references and shorthands.
func (p *sentinelParser) resolveIdent(name string) (interface{}, error) {
	// ── Shorthands ─────────────────────────────────────────────────────────
	switch name {
	case "$authenticated":
		return p.ctx.User != nil, nil

	case "$unauthenticated":
		return p.ctx.User == nil, nil

	case "$admin":
		if p.ctx.User == nil {
			return false, nil
		}
		for _, r := range p.ctx.User.Roles {
			if r == "admin" {
				return true, nil
			}
		}
		return false, nil

	case "$verified":
		if p.ctx.User == nil {
			return false, nil
		}
		return p.ctx.User.EmailVerified, nil

	case "$owner":
		// $owner = $req.user.id == $doc.owner_id
		if p.ctx.User == nil || p.ctx.Doc == nil {
			return false, nil
		}
		ownerVal, ok := p.ctx.Doc["owner_id"]
		if !ok {
			return false, nil
		}
		return fmt.Sprintf("%v", ownerVal) == p.ctx.User.ID, nil
	}

	parts := strings.Split(name, ".")

	// ── $req.* ──────────────────────────────────────────────────────────────
	if parts[0] == "$req" {
		if len(parts) == 1 {
			return nil, fmt.Errorf("$req requires a sub-field (e.g. $req.user.id)")
		}
		switch parts[1] {
		case "ip":
			return p.ctx.IP, nil
		case "method":
			return p.ctx.Method, nil

		case "user":
			if len(parts) == 2 {
				// bare $req.user — truthy if authenticated, falsy if not
				if p.ctx.User == nil {
					return nil, nil
				}
				return p.ctx.User, nil // truthy object
			}
			if p.ctx.User == nil {
				return nil, nil
			}
			field := parts[2]
			switch field {
			case "id":
				return p.ctx.User.ID, nil
			case "email":
				return p.ctx.User.Email, nil
			case "roles":
				return p.ctx.User.Roles, nil
			case "verified", "email_verified":
				return p.ctx.User.EmailVerified, nil
			case "data":
				// $req.user.data.<field>
				if len(parts) >= 4 {
					if v, ok := p.ctx.User.Data[parts[3]]; ok {
						return v, nil
					}
				}
				return p.ctx.User.Data, nil
			default:
				// $req.user.<anything> → check user.Data directly
				if v, ok := p.ctx.User.Data[field]; ok {
					return v, nil
				}
				return nil, nil
			}
		}
		return nil, fmt.Errorf("unknown $req field %q", parts[1])
	}

	// ── $doc.* ──────────────────────────────────────────────────────────────
	if parts[0] == "$doc" {
		if p.ctx.Doc == nil {
			return nil, nil
		}
		if len(parts) == 1 {
			return p.ctx.Doc, nil
		}
		// support nested: $doc.meta.title → walk the map
		var cur interface{} = map[string]interface{}(p.ctx.Doc)
		for _, seg := range parts[1:] {
			m, ok := cur.(map[string]interface{})
			if !ok {
				return nil, nil
			}
			cur, ok = m[seg]
			if !ok {
				return nil, nil
			}
		}
		return cur, nil
	}

	return nil, fmt.Errorf("unknown variable %q — use $req.user.*, $doc.*, or a shorthand like $authenticated", name)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	case []string:
		return len(val) > 0
	case []interface{}:
		return len(val) > 0
	case map[string]interface{}:
		return len(val) > 0
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map:
			return rv.Len() > 0
		case reflect.Ptr, reflect.Interface:
			return !rv.IsNil()
		case reflect.Struct:
			return true // a struct is always truthy (e.g. AppUser)
		}
		return true
	}
}

func compareEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// bool to bool
	if ab, ok := a.(bool); ok {
		if bb, ok2 := b.(bool); ok2 {
			return ab == bb
		}
	}
	// Numeric
	if fa, err := toFloat(a); err == nil {
		if fb, err2 := toFloat(b); err2 == nil {
			return fa == fb
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func sentinelContains(haystack, needle interface{}) bool {
	if haystack == nil {
		return false
	}
	needleStr := fmt.Sprintf("%v", needle)
	switch h := haystack.(type) {
	case string:
		return strings.Contains(h, needleStr)
	case []string:
		for _, s := range h {
			if s == needleStr {
				return true
			}
		}
		return false
	case []interface{}:
		for _, v := range h {
			if fmt.Sprintf("%v", v) == needleStr {
				return true
			}
		}
		return false
	default:
		rv := reflect.ValueOf(haystack)
		if rv.Kind() == reflect.Slice {
			for i := 0; i < rv.Len(); i++ {
				if fmt.Sprintf("%v", rv.Index(i).Interface()) == needleStr {
					return true
				}
			}
		}
	}
	return false
}

// sentinelIn checks whether left is found inside the right (list).
// Supports: value in [a, b, c]   and   value in $doc.editors
func sentinelIn(value, list interface{}) bool {
	if list == nil {
		return false
	}
	valStr := fmt.Sprintf("%v", value)
	switch l := list.(type) {
	case []interface{}:
		for _, item := range l {
			if fmt.Sprintf("%v", item) == valStr {
				return true
			}
		}
	case []string:
		for _, s := range l {
			if s == valStr {
				return true
			}
		}
	default:
		// Try reflection
		rv := reflect.ValueOf(list)
		if rv.Kind() == reflect.Slice {
			for i := 0; i < rv.Len(); i++ {
				if fmt.Sprintf("%v", rv.Index(i).Interface()) == valStr {
					return true
				}
			}
		}
	}
	return false
}

// globMatch does simple * wildcard matching (case-sensitive).
func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return s == pattern
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i, p := range parts[1:] {
		if i == len(parts)-2 {
			return strings.HasSuffix(s, p)
		}
		idx := strings.Index(s, p)
		if idx < 0 {
			return false
		}
		s = s[idx+len(p):]
	}
	return true
}

func compareNumeric(a interface{}, op string, b interface{}) (bool, error) {
	fa, err := toFloat(a)
	if err != nil {
		return false, fmt.Errorf("left side of %s is not numeric: %v", op, a)
	}
	fb, err := toFloat(b)
	if err != nil {
		return false, fmt.Errorf("right side of %s is not numeric: %v", op, b)
	}
	switch op {
	case "<":
		return fa < fb, nil
	case ">":
		return fa > fb, nil
	case "<=":
		return fa <= fb, nil
	case ">=":
		return fa >= fb, nil
	}
	return false, nil
}

func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	case string:
		return strconv.ParseFloat(val, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to float64", v)
}

// SentinelAutocomplete returns suggestions for the dashboard editor.
var SentinelAutocomplete = []map[string]string{
	// Shorthands — the most useful ones first
	{"insert": "$authenticated", "detail": "true if user is logged in — the most common gate"},
	{"insert": "$unauthenticated", "detail": "true if no auth token was sent"},
	{"insert": "$admin", "detail": "true if user has the 'admin' role"},
	{"insert": "$owner", "detail": "true if $req.user.id == $doc.owner_id"},
	{"insert": "$verified", "detail": "true if user's email is verified"},
	// User fields
	{"insert": "$req.user.id", "detail": "Authenticated user's ID"},
	{"insert": "$req.user.email", "detail": "Authenticated user's email"},
	{"insert": "$req.user.roles", "detail": "Authenticated user's roles (array)"},
	{"insert": "$req.user.verified", "detail": "Whether user's email is verified"},
	{"insert": "$req.user.plan", "detail": "Custom field from user.Data (example)"},
	{"insert": "$req.user.team_id", "detail": "Custom field from user.Data (example)"},
	{"insert": "$req.user.data.", "detail": "Any field from user.Data map"},
	// Doc fields
	{"insert": "$doc.", "detail": "A field from the document"},
	{"insert": "$doc.owner_id", "detail": "Document's owner_id field (example)"},
	{"insert": "$doc.published", "detail": "Document's published field (example)"},
	// Request context
	{"insert": "$req.ip", "detail": "Client IP address"},
	{"insert": "$req.method", "detail": "HTTP method (GET, POST, PATCH, DELETE)"},
	// Operators
	{"insert": "==", "detail": "Equal"},
	{"insert": "!=", "detail": "Not equal"},
	{"insert": "contains", "detail": "String/array contains value"},
	{"insert": "startswith", "detail": "String starts with prefix"},
	{"insert": "endswith", "detail": "String ends with suffix"},
	{"insert": "in", "detail": "Value is in array: $req.user.id in $doc.editors"},
	{"insert": "matches", "detail": "Glob match (* wildcard): $req.user.email matches \"*@acme.com\""},
	{"insert": "exists", "detail": "Field is not null/empty: exists $doc.approved_at"},
	{"insert": "&&", "detail": "Logical AND"},
	{"insert": "||", "detail": "Logical OR"},
	{"insert": "!", "detail": "Logical NOT"},
	{"insert": "true", "detail": "Boolean true"},
	{"insert": "false", "detail": "Boolean false"},
	{"insert": "null", "detail": "Null value"},
}
