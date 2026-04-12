package functions

// dbquery.go — implements the db.query / db.findOne / db.queryUsers / db.findUser API.
//
// Filter keys use operator suffixes matching the Cocobase cloud docs:
//   field           — exact match (eq)
//   field_eq        — exact match
//   field_ne        — not equal
//   field_gt        — greater than
//   field_gte       — greater than or equal
//   field_lt        — less than
//   field_lte       — less than or equal
//   field_contains  — case-insensitive string contains
//   field_startswith— starts with
//   field_endswith  — ends with
//   field_in        — comma-separated value list
//   field_notin     — not in comma-separated list
//   field_isnull    — "true"/"false" null check
//
// OR logic:
//   [or]field           — simple OR group
//   [or:groupName]field — named OR group (all conditions in same group are OR'd)
//
// Special opts keys (not treated as filters):
//   limit, offset, sort, order, select, populate

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"gorm.io/gorm"
)

// ── Filter parsing ────────────────────────────────────────────────────────────

var controlKeys = map[string]bool{
	"limit": true, "offset": true, "sort": true,
	"order": true, "select": true, "populate": true,
}

type filterClause struct {
	sql  string
	args []interface{}
}

// parseFilters converts an options map into WHERE clauses, separated into
// AND clauses and named OR groups.
func parseFilters(opts map[string]interface{}, jsonField string) (andClauses []filterClause, orGroups map[string][]filterClause) {
	orGroups = map[string][]filterClause{}

	for rawKey, rawVal := range opts {
		if controlKeys[rawKey] {
			continue
		}

		// Detect OR prefix: [or]field  or  [or:name]field
		orGroup := ""
		key := rawKey
		if strings.HasPrefix(rawKey, "[or") {
			end := strings.Index(rawKey, "]")
			if end < 0 {
				continue
			}
			prefix := rawKey[1:end] // "or" or "or:name"
			key = rawKey[end+1:]
			if strings.Contains(prefix, ":") {
				orGroup = strings.SplitN(prefix, ":", 2)[1]
			} else {
				orGroup = "__default__"
			}
		}

		// Strip numeric suffix used to repeat the same key (status_2, status_3…)
		// e.g. "[or:g]status_2" → field "status", op ""
		cleanKey := key
		if idx := strings.LastIndex(key, "_"); idx > 0 {
			suffix := key[idx+1:]
			if isNumericSuffix(suffix) {
				cleanKey = key[:idx]
			}
		}

		clause, ok := buildClause(cleanKey, rawVal, jsonField)
		if !ok {
			continue
		}

		if orGroup != "" {
			orGroups[orGroup] = append(orGroups[orGroup], clause)
		} else {
			andClauses = append(andClauses, clause)
		}
	}
	return
}

func isNumericSuffix(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// buildClause converts one key=value filter into a SQL clause.
// jsonField is the column name that holds JSON data (e.g. "data" for documents).
// For top-level columns (id, created_at) jsonField is ignored.
func buildClause(key string, val interface{}, jsonField string) (filterClause, bool) {
	valStr := fmt.Sprintf("%v", val)

	// Split field and operator
	field, op := splitOp(key)

	// Top-level columns that don't need json_extract
	topLevel := map[string]string{
		"id":         "id",
		"created_at": "created_at",
		"updated_at": "updated_at",
		"email":      "email",    // app_users
		"role":       "role",     // app_users
	}

	var colExpr string
	if col, ok := topLevel[field]; ok {
		colExpr = col
	} else if jsonField != "" {
		colExpr = fmt.Sprintf("json_extract(%s, '$.%s')", jsonField, field)
	} else {
		colExpr = field
	}

	switch op {
	case "", "eq":
		return filterClause{colExpr + " = ?", []interface{}{val}}, true
	case "ne":
		return filterClause{colExpr + " != ?", []interface{}{val}}, true
	case "gt":
		return filterClause{colExpr + " > ?", []interface{}{val}}, true
	case "gte":
		return filterClause{colExpr + " >= ?", []interface{}{val}}, true
	case "lt":
		return filterClause{colExpr + " < ?", []interface{}{val}}, true
	case "lte":
		return filterClause{colExpr + " <= ?", []interface{}{val}}, true
	case "contains":
		return filterClause{colExpr + " LIKE ?", []interface{}{"%" + valStr + "%"}}, true
	case "startswith":
		return filterClause{colExpr + " LIKE ?", []interface{}{valStr + "%"}}, true
	case "endswith":
		return filterClause{colExpr + " LIKE ?", []interface{}{"%" + valStr}}, true
	case "in":
		parts := strings.Split(valStr, ",")
		placeholders := make([]string, len(parts))
		args := make([]interface{}, len(parts))
		for i, p := range parts {
			placeholders[i] = "?"
			args[i] = strings.TrimSpace(p)
		}
		return filterClause{colExpr + " IN (" + strings.Join(placeholders, ",") + ")", args}, true
	case "notin":
		parts := strings.Split(valStr, ",")
		placeholders := make([]string, len(parts))
		args := make([]interface{}, len(parts))
		for i, p := range parts {
			placeholders[i] = "?"
			args[i] = strings.TrimSpace(p)
		}
		return filterClause{colExpr + " NOT IN (" + strings.Join(placeholders, ",") + ")", args}, true
	case "isnull":
		if valStr == "true" {
			return filterClause{colExpr + " IS NULL", nil}, true
		}
		return filterClause{colExpr + " IS NOT NULL", nil}, true
	}
	return filterClause{}, false
}

// splitOp splits "price_gte" → ("price", "gte"), "name" → ("name", "").
// Uses the known operator set so fields like "created_at" don't get mis-split.
var knownOps = map[string]bool{
	"eq": true, "ne": true, "gt": true, "gte": true,
	"lt": true, "lte": true, "contains": true,
	"startswith": true, "endswith": true,
	"in": true, "notin": true, "isnull": true,
}

func splitOp(key string) (field, op string) {
	idx := strings.LastIndex(key, "_")
	if idx < 0 {
		return key, ""
	}
	maybOp := key[idx+1:]
	if knownOps[maybOp] {
		return key[:idx], maybOp
	}
	return key, ""
}

// applyFilters adds AND + OR where clauses to a gorm query.
func applyFilters(q *gorm.DB, opts map[string]interface{}, jsonField string) *gorm.DB {
	andClauses, orGroups := parseFilters(opts, jsonField)

	for _, c := range andClauses {
		q = q.Where(c.sql, c.args...)
	}

	for _, group := range orGroups {
		if len(group) == 0 {
			continue
		}
		// Combine group into a single OR expression
		parts := make([]string, len(group))
		var args []interface{}
		for i, c := range group {
			parts[i] = c.sql
			args = append(args, c.args...)
		}
		q = q.Where("("+strings.Join(parts, " OR ")+")", args...)
	}

	return q
}

// ── Document query ────────────────────────────────────────────────────────────

// queryDocs runs a filtered query against a collection's documents.
// Returns ([]map, total).
func queryDocs(collectionID string, opts map[string]interface{}, _ bool) ([]map[string]interface{}, int64) {
	limit := optInt(opts, "limit", 10)
	offset := optInt(opts, "offset", 0)
	sort := optStr(opts, "sort", "created_at")
	order := optStr(opts, "order", "desc")
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	q := database.DB.Model(&models.Document{}).Where("collection_id = ?", collectionID)
	q = applyFilters(q, opts, "data")

	// Count total (before limit/offset)
	var total int64
	q.Count(&total)

	// Fetch rows
	orderExpr := sortExpr(sort, order)
	var docs []models.Document
	q.Order(orderExpr).Limit(limit).Offset(offset).Find(&docs)

	result := make([]map[string]interface{}, len(docs))
	for i, d := range docs {
		result[i] = docToMap(&d)
	}
	return result, total
}

// sortExpr builds an ORDER BY expression, handling json_extract for data fields.
func sortExpr(field, order string) string {
	topLevel := map[string]bool{"id": true, "created_at": true, "updated_at": true, "collection_id": true}
	if topLevel[field] {
		return field + " " + order
	}
	return fmt.Sprintf("json_extract(data, '$.%s') %s", field, order)
}

// ── User query ────────────────────────────────────────────────────────────────

// queryUsers runs a filtered query against app_users for a project.
func queryUsers(projectID string, opts map[string]interface{}) ([]map[string]interface{}, int64) {
	limit := optInt(opts, "limit", 10)
	offset := optInt(opts, "offset", 0)
	sort := optStr(opts, "sort", "created_at")
	order := optStr(opts, "order", "desc")
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	q := database.DB.Model(&models.AppUser{}).Where("project_id = ?", projectID)
	q = applyFilters(q, opts, "data")

	var total int64
	q.Count(&total)

	var users []models.AppUser
	q.Order(sort + " " + order).Limit(limit).Offset(offset).Find(&users)

	result := make([]map[string]interface{}, len(users))
	for i, u := range users {
		result[i] = userToMap(&u)
	}
	return result, total
}

func userToMap(u *models.AppUser) map[string]interface{} {
	m := map[string]interface{}{
		"id":             u.ID,
		"email":          u.Email,
		"roles":          u.Roles,
		"email_verified": u.EmailVerified,
		"created_at":     u.CreatedAt,
	}
	for k, v := range u.Data {
		if _, reserved := m[k]; !reserved {
			m[k] = v
		}
	}
	return m
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// exportMap converts a goja Value to map[string]interface{}.
// Returns empty map if value is undefined/null or not an object.
func exportMap(v goja.Value) map[string]interface{} {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return map[string]interface{}{}
	}
	exported := v.Export()
	if m, ok := exported.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func optInt(opts map[string]interface{}, key string, def int) int {
	if v, ok := opts[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		case string:
			var i int
			fmt.Sscanf(n, "%d", &i)
			return i
		}
	}
	return def
}

func optStr(opts map[string]interface{}, key, def string) string {
	if v, ok := opts[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}
