package services

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// ─────────────────────────────────────────
// Operator constants
// ─────────────────────────────────────────

const (
	OpEq         = "eq"
	OpNe         = "ne"
	OpGt         = "gt"
	OpGte        = "gte"
	OpLt         = "lt"
	OpLte        = "lte"
	OpContains   = "contains"
	OpStartsWith = "startswith"
	OpEndsWith   = "endswith"
	OpIn         = "in"
	OpNotIn      = "notin"
	OpIsNull     = "isnull"
)

// FilterExpression represents a single filter condition
type FilterExpression struct {
	Field    string
	Operator string
	Value    string
}

// QueryBuilder builds dynamic GORM queries from URL parameters
type QueryBuilder struct {
	db *gorm.DB
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(db *gorm.DB) *QueryBuilder {
	return &QueryBuilder{db: db}
}

// ─────────────────────────────────────────
// BuildQuery
// ─────────────────────────────────────────

// BuildQuery constructs a GORM query from URL parameters.
//
// Key optimization: all AND filters are collapsed into ONE WHERE clause string
// instead of N chained .Where() calls. This lets PostgreSQL receive and plan
// the full predicate in a single pass.
func (qb *QueryBuilder) BuildQuery(baseQuery *gorm.DB, params map[string][]string, reservedParams []string) *gorm.DB {
	reserved := make(map[string]bool, len(reservedParams))
	for _, p := range reservedParams {
		reserved[p] = true
	}

	var andFilters []FilterExpression
	orGroups := make(map[string][]FilterExpression)

	for key, values := range params {
		if len(values) == 0 || reserved[key] {
			continue
		}
		value := values[0]
		orGroup := ""
		actualKey := key

		if strings.HasPrefix(key, "[or]") {
			actualKey = key[4:]
			orGroup = "__simple_or__"
		} else if strings.HasPrefix(key, "[or:") {
			if endIdx := strings.Index(key, "]"); endIdx > 0 {
				orGroup = key[4:endIdx]
				actualKey = key[endIdx+1:]
			}
		}

		switch {
		case strings.Contains(actualKey, "__or__"):
			filters := parseMultiField(actualKey, "__or__", value)
			groupKey := orGroup
			if groupKey == "" {
				groupKey = "__implicit_or__" + actualKey
			}
			orGroups[groupKey] = append(orGroups[groupKey], filters...)

		case strings.Contains(actualKey, "__and__"):
			filters := parseMultiField(actualKey, "__and__", value)
			if orGroup != "" {
				orGroups[orGroup] = append(orGroups[orGroup], filters...)
			} else {
				andFilters = append(andFilters, filters...)
			}

		default:
			field, op := extractFieldAndOperator(actualKey)
			fe := FilterExpression{Field: field, Operator: op, Value: value}
			if orGroup != "" {
				orGroups[orGroup] = append(orGroups[orGroup], fe)
			} else {
				andFilters = append(andFilters, fe)
			}
		}
	}

	// Collapse all AND filters into one WHERE clause
	if len(andFilters) > 0 {
		sql, args := filtersToANDClause(andFilters)
		if sql != "" {
			baseQuery = baseQuery.Where(sql, args...)
		}
	}

	// Each OR group becomes one WHERE clause
	for _, groupFilters := range orGroups {
		if len(groupFilters) == 0 {
			continue
		}
		sql, args := filtersToORClause(groupFilters)
		if sql != "" {
			baseQuery = baseQuery.Where(sql, args...)
		}
	}

	return baseQuery
}

// ─────────────────────────────────────────
// Relationship filters
// ─────────────────────────────────────────

// ParseRelationshipFilters separates regular filters from relationship dot-notation filters.
func ParseRelationshipFilters(params map[string][]string, reservedParams []string) (regular map[string]string, relationship map[string]string) {
	reserved := make(map[string]bool, len(reservedParams))
	for _, p := range reservedParams {
		reserved[p] = true
	}
	regular = make(map[string]string)
	relationship = make(map[string]string)

	for key, values := range params {
		if len(values) == 0 || reserved[key] {
			continue
		}
		if strings.Contains(key, ".") && !strings.HasPrefix(key, "[") {
			relationship[key] = values[0]
		} else {
			regular[key] = values[0]
		}
	}
	return
}

// ApplyRelationshipFilters applies dot-notation relationship filters.
//
// Single condition on a relation  → INNER JOIN  (one pass, PG can use indexes)
// Multiple conditions on a relation → EXISTS    (avoids row multiplication)
func (qb *QueryBuilder) ApplyRelationshipFilters(query *gorm.DB, filters map[string]string, projectID string) *gorm.DB {
	if len(filters) == 0 {
		return query
	}

	type relFilter struct {
		nestedField string
		operator    string
		value       string
		isUser      bool
		targetName  string
		idField     string
	}

	// Group by relation field
	grouped := make(map[string][]relFilter)
	for path, value := range filters {
		dotIdx := strings.Index(path, ".")
		if dotIdx < 0 {
			continue
		}
		relField := path[:dotIdx]
		nestedField, operator := extractFieldAndOperator(path[dotIdx+1:])
		targetName := pluralize(relField)
		grouped[relField] = append(grouped[relField], relFilter{
			nestedField: nestedField,
			operator:    operator,
			value:       value,
			isUser:      IsUserField(relField) || isSystemCollection(targetName),
			targetName:  targetName,
			idField:     relField + "_id",
		})
	}

	for relField, relFilters := range grouped {
		rf := relFilters[0]
		if len(relFilters) == 1 {
			// Single condition → JOIN
			if rf.isUser {
				sql, args := buildAppUserJoinSQL(relField, rf.idField, rf.nestedField, rf.operator, rf.value, projectID)
				query = query.Joins(sql, args...)
			} else {
				sql, args := buildCollectionJoinSQL(relField, rf.idField, rf.nestedField, rf.operator, rf.value, projectID, rf.targetName)
				query = query.Joins(sql, args...)
			}
		} else {
			// Multiple conditions on same relation → EXISTS
			for _, r := range relFilters {
				var sql string
				var args []interface{}
				if r.isUser {
					sql, args = buildAppUserExistsSQL(r.idField, r.nestedField, r.operator, r.value, projectID)
				} else {
					sql, args = buildCollectionExistsSQL(r.idField, r.nestedField, r.operator, r.value, projectID, r.targetName)
				}
				query = query.Where(sql, args...)
			}
		}
	}

	return query
}

// ─────────────────────────────────────────
// Sorting & Pagination
// ─────────────────────────────────────────

func (qb *QueryBuilder) ApplySorting(query *gorm.DB, sortField, sortOrder string) *gorm.DB {
	if sortField == "" {
		sortField = "created_at"
	}
	dir := "DESC"
	if strings.ToLower(sortOrder) == "asc" {
		dir = "ASC"
	}
	switch sortField {
	case "created_at", "updated_at", "id":
		return query.Order(sortField + " " + dir)
	default:
		return query.Order(fmt.Sprintf("data->>'%s' %s", sortField, dir))
	}
}

func (qb *QueryBuilder) ApplyPagination(query *gorm.DB, limit, offset int) *gorm.DB {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	return query.Limit(limit).Offset(offset)
}

// ─────────────────────────────────────────
// Clause builders
// ─────────────────────────────────────────

// filtersToANDClause collapses N filters into one SQL AND string.
// One .Where() call → one predicate for PG to plan.
func filtersToANDClause(filters []FilterExpression) (string, []interface{}) {
	parts := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters)*2)
	for _, f := range filters {
		sql, fArgs := filterToSQL(f)
		if sql != "" {
			parts = append(parts, "("+sql+")")
			args = append(args, fArgs...)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " AND "), args
}

// filtersToORClause collapses N filters into one SQL OR string.
func filtersToORClause(filters []FilterExpression) (string, []interface{}) {
	parts := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters)*2)
	for _, f := range filters {
		sql, fArgs := filterToSQL(f)
		if sql != "" {
			parts = append(parts, "("+sql+")")
			args = append(args, fArgs...)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func filterToSQL(f FilterExpression) (string, []interface{}) {
	col := "data->>'" + f.Field + "'"
	return applyOperatorToCol(col, f.Operator, f.Value)
}

// ─────────────────────────────────────────
// JOIN builders
// ─────────────────────────────────────────

func buildAppUserJoinSQL(relField, idField, nestedField, operator, value, projectID string) (string, []interface{}) {
	alias := "au_" + relField
	colExpr := resolveUserColForAlias(alias, nestedField)
	condSQL, condArgs := applyOperatorToCol(colExpr, operator, value)
	sql := fmt.Sprintf(
		`INNER JOIN app_users %s ON %s.id::text = (documents.data->>'%s') AND %s.client_id = ? AND (%s)`,
		alias, alias, idField, alias, condSQL,
	)
	return sql, append([]interface{}{projectID}, condArgs...)
}

func buildCollectionJoinSQL(relField, idField, nestedField, operator, value, projectID, collectionName string) (string, []interface{}) {
	dAlias := "rel_" + relField
	cAlias := "relc_" + relField
	condSQL, condArgs := applyOperatorToCol(fmt.Sprintf("(%s.data->>'%s')", dAlias, nestedField), operator, value)
	sql := fmt.Sprintf(`
		INNER JOIN documents %s ON %s.id::text = (documents.data->>'%s')
		INNER JOIN collections %s ON %s.id = %s.collection_id
			AND %s.project_id = ?
			AND (%s.name = ? OR %s.name = ? OR %s.name = ?)
		AND (%s)`,
		dAlias, dAlias, idField,
		cAlias, cAlias, dAlias,
		cAlias, cAlias, cAlias, cAlias,
		condSQL,
	)
	return sql, append([]interface{}{projectID, collectionName, pluralize(collectionName), singularize(collectionName)}, condArgs...)
}

// ─────────────────────────────────────────
// EXISTS builders
// ─────────────────────────────────────────

func buildAppUserExistsSQL(idField, nestedField, operator, value, projectID string) (string, []interface{}) {
	col := resolveUserColRaw(nestedField)
	condSQL, condArgs := applyOperatorToCol(col, operator, value)
	sql := fmt.Sprintf(`EXISTS (
		SELECT 1 FROM app_users au
		WHERE au.id::text = (documents.data->>'%s')
		AND au.client_id = ?
		AND (%s)
	)`, idField, condSQL)
	return sql, append([]interface{}{projectID}, condArgs...)
}

func buildCollectionExistsSQL(idField, nestedField, operator, value, projectID, collectionName string) (string, []interface{}) {
	condSQL, condArgs := applyOperatorToCol(fmt.Sprintf("(sub.data->>'%s')", nestedField), operator, value)
	sql := fmt.Sprintf(`EXISTS (
		SELECT 1 FROM documents sub
		JOIN collections c ON c.id = sub.collection_id
		WHERE sub.id::text = (documents.data->>'%s')
		AND c.project_id = ?
		AND (c.name = ? OR c.name = ? OR c.name = ?)
		AND (%s)
	)`, idField, condSQL)
	return sql, append([]interface{}{projectID, collectionName, pluralize(collectionName), singularize(collectionName)}, condArgs...)
}

// ─────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────

func applyOperatorToCol(col, operator, value string) (string, []interface{}) {
	switch operator {
	case OpEq:
		return col + " = ?", []interface{}{value}
	case OpNe:
		return col + " != ?", []interface{}{value}
	case OpContains:
		return "LOWER(" + col + ") LIKE LOWER(?)", []interface{}{"%" + value + "%"}
	case OpStartsWith:
		return "LOWER(" + col + ") LIKE LOWER(?)", []interface{}{value + "%"}
	case OpEndsWith:
		return "LOWER(" + col + ") LIKE LOWER(?)", []interface{}{"%" + value}
	case OpGt:
		if n, err := parseNumber(value); err == nil {
			return "CAST(" + col + " AS numeric) > ?", []interface{}{n}
		}
	case OpGte:
		if n, err := parseNumber(value); err == nil {
			return "CAST(" + col + " AS numeric) >= ?", []interface{}{n}
		}
	case OpLt:
		if n, err := parseNumber(value); err == nil {
			return "CAST(" + col + " AS numeric) < ?", []interface{}{n}
		}
	case OpLte:
		if n, err := parseNumber(value); err == nil {
			return "CAST(" + col + " AS numeric) <= ?", []interface{}{n}
		}
	case OpIn:
		return col + " IN ?", []interface{}{splitTrimmed(value)}
	case OpNotIn:
		return col + " NOT IN ?", []interface{}{splitTrimmed(value)}
	case OpIsNull:
		if value == "true" || value == "1" {
			return "(" + col + " IS NULL OR " + col + " = '')", nil
		}
		return "(" + col + " IS NOT NULL AND " + col + " != '')", nil
	}
	return col + " = ?", []interface{}{value}
}

func resolveUserColForAlias(alias, field string) string {
	switch field {
	case "email", "phone_number", "id":
		return alias + "." + field
	default:
		return fmt.Sprintf("(%s.data->>'%s')", alias, field)
	}
}

func resolveUserColRaw(field string) string {
	switch field {
	case "email":
		return "au.email"
	case "phone_number":
		return "au.phone_number"
	case "id":
		return "au.id::text"
	default:
		return fmt.Sprintf("(au.data->>'%s')", field)
	}
}

func parseMultiField(key, separator, value string) []FilterExpression {
	parts := strings.Split(key, separator)
	op := extractOperatorFromField(parts[len(parts)-1])
	filters := make([]FilterExpression, 0, len(parts))
	for _, part := range parts {
		field, _ := extractFieldAndOperator(part)
		filters = append(filters, FilterExpression{Field: field, Operator: op, Value: value})
	}
	return filters
}

func extractFieldAndOperator(fieldExpr string) (string, string) {
	if strings.Contains(fieldExpr, "__") {
		parts := strings.Split(fieldExpr, "__")
		last := parts[len(parts)-1]
		if isValidOperator(last) {
			return strings.Join(parts[:len(parts)-1], "__"), last
		}
	}
	if strings.Contains(fieldExpr, "_") {
		parts := strings.Split(fieldExpr, "_")
		last := parts[len(parts)-1]
		if isValidOperator(last) {
			return strings.Join(parts[:len(parts)-1], "_"), last
		}
	}
	return fieldExpr, OpEq
}

func extractOperatorFromField(fieldExpr string) string {
	_, op := extractFieldAndOperator(fieldExpr)
	return op
}

func isValidOperator(op string) bool {
	switch op {
	case OpEq, OpNe, OpGt, OpGte, OpLt, OpLte,
		OpContains, OpStartsWith, OpEndsWith,
		OpIn, OpNotIn, OpIsNull:
		return true
	}
	return false
}

func isSystemCollection(name string) bool {
	switch strings.ToLower(name) {
	case "users", "app_users", "appusers", "user", "appuser":
		return true
	}
	return false
}

func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}
