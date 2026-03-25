package services

import (
	"strings"
	"sync"
	"time"

	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────
// Types
// ─────────────────────────────────────────

type RelationshipResolver struct {
	db       *gorm.DB
	colCache sync.Map
}

func NewRelationshipResolver() *RelationshipResolver {
	return &RelationshipResolver{db: database.DB}
}

type PopulateRequest struct {
	Field            string
	Select           []string
	Nested           bool
	NestedPopulates  []string
	ForceSource      string
	TargetCollection string
}

// ─────────────────────────────────────────
// Parse
// ─────────────────────────────────────────

func ParsePopulateParams(populate, selectParam string) []PopulateRequest {
	if populate == "" {
		return nil
	}

	var selectedFields []string
	for _, f := range strings.Split(selectParam, ",") {
		if f = strings.TrimSpace(f); f != "" {
			selectedFields = append(selectedFields, f)
		}
	}

	parentMap := make(map[string]*PopulateRequest)

	for _, raw := range strings.Split(populate, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		forceSource := ""
		targetCollection := ""
		fieldPath := raw

		if strings.Contains(raw, ":") && !strings.Contains(raw, ".") {
			parts := strings.SplitN(raw, ":", 2)
			fieldPath = parts[0]
			spec := strings.ToLower(parts[1])
			if spec == "appuser" {
				forceSource = "appuser"
			} else {
				forceSource = "collection"
				targetCollection = spec
			}
		}

		if strings.Contains(fieldPath, ".") {
			parts := strings.SplitN(fieldPath, ".", 2)
			parent, nested := parts[0], parts[1]
			if req, ok := parentMap[parent]; ok {
				req.NestedPopulates = append(req.NestedPopulates, nested)
				req.Nested = true
			} else {
				parentMap[parent] = &PopulateRequest{
					Field:           parent,
					Select:          selectedFields,
					Nested:          true,
					NestedPopulates: []string{nested},
				}
			}
		} else {
			if existing, ok := parentMap[fieldPath]; ok {
				if forceSource != "" {
					existing.ForceSource = forceSource
					existing.TargetCollection = targetCollection
				}
			} else {
				parentMap[fieldPath] = &PopulateRequest{
					Field:            fieldPath,
					Select:           selectedFields,
					ForceSource:      forceSource,
					TargetCollection: targetCollection,
				}
			}
		}
	}

	result := make([]PopulateRequest, 0, len(parentMap))
	for _, r := range parentMap {
		result = append(result, *r)
	}
	return result
}

// ─────────────────────────────────────────
// Populate
// ─────────────────────────────────────────

func (r *RelationshipResolver) PopulateDocuments(documents []map[string]interface{}, projectID string, populateRequests []PopulateRequest) error {
	if len(populateRequests) == 0 || len(documents) == 0 {
		return nil
	}
	for _, req := range populateRequests {
		_ = r.populateField(documents, projectID, req)
	}
	return nil
}

func (r *RelationshipResolver) populateField(documents []map[string]interface{}, projectID string, req PopulateRequest) error {
	idField := req.Field + "_id"
	idsField := req.Field + "_ids"

	isUser := false
	targetName := req.TargetCollection
	if targetName == "" {
		targetName = pluralize(req.Field)
	}
	switch req.ForceSource {
	case "appuser":
		isUser = true
	case "collection":
		isUser = false
	default:
		isUser = IsUserField(req.Field)
	}

	// ── Collect IDs ───────────────────────────────────────────────────────
	singleIDs := make(map[string]bool)
	arrayIDs := make(map[string]bool)

	for _, doc := range documents {
		// getDataFromDoc handles both map[string]interface{} and models.JSONMap
		data := getDataFromDoc(doc)
		if data == nil {
			continue
		}

		if v, ok := data[idField]; ok && v != nil {
			if s, ok := v.(string); ok && s != "" {
				singleIDs[s] = true
				continue
			}
		}

		if v, ok := data[req.Field]; ok && v != nil {
			switch val := v.(type) {
			case string:
				if val != "" && !strings.Contains(val, " ") {
					singleIDs[val] = true
				}
			case []interface{}:
				for _, item := range val {
					if s, ok := item.(string); ok && s != "" {
						arrayIDs[s] = true
					}
				}
			}
		}

		if v, ok := data[idsField]; ok && v != nil {
			switch val := v.(type) {
			case []interface{}:
				for _, item := range val {
					if s, ok := item.(string); ok && s != "" {
						arrayIDs[s] = true
					}
				}
			case []string:
				for _, s := range val {
					if s != "" {
						arrayIDs[s] = true
					}
				}
			}
		}
	}

	allIDs := mergeToSlice(singleIDs, arrayIDs)
	if len(allIDs) == 0 {
		return r.populateReverseField(documents, projectID, req.Field, req.Select, req.NestedPopulates)
	}

	// ── Batch fetch by primary key ────────────────────────────────────────
	var fetched map[string]map[string]interface{}
	var err error
	if isUser {
		fetched, err = r.fetchAppUsers(allIDs, projectID, req.Select)
	} else {
		fetched, err = r.fetchDocumentsFromCollection(allIDs, projectID, targetName, req.Select)
	}
	if err != nil || len(fetched) == 0 {
		return err
	}

	// ── Nested population ─────────────────────────────────────────────────
	if len(req.NestedPopulates) > 0 {
		nestedDocs := make([]map[string]interface{}, 0, len(fetched))
		for _, v := range fetched {
			nestedDocs = append(nestedDocs, v)
		}
		nestedReqs := ParsePopulateParams(strings.Join(req.NestedPopulates, ","), "")
		_ = r.PopulateDocuments(nestedDocs, projectID, nestedReqs)
		for _, d := range nestedDocs {
			if id, ok := d["id"].(string); ok {
				fetched[id] = d
			}
		}
	}

	// ── Assign back ───────────────────────────────────────────────────────
	for i, doc := range documents {
		data := getDataFromDoc(doc)
		if data == nil {
			continue
		}

		if v, ok := data[idField]; ok && v != nil {
			if id, ok := v.(string); ok {
				if related, ok := fetched[id]; ok {
					data[req.Field] = related
				}
			}
		} else if v, ok := data[req.Field]; ok && v != nil {
			switch val := v.(type) {
			case string:
				if related, ok := fetched[val]; ok {
					data[req.Field] = related
				}
			case []interface{}:
				populated := make([]interface{}, 0, len(val))
				for _, item := range val {
					if id, ok := item.(string); ok {
						if related, ok := fetched[id]; ok {
							populated = append(populated, related)
						}
					}
				}
				if len(populated) > 0 {
					data[req.Field] = populated
				}
			}
		}

		if v, ok := data[idsField]; ok && v != nil {
			populated := make([]interface{}, 0)
			switch val := v.(type) {
			case []interface{}:
				for _, item := range val {
					if id, ok := item.(string); ok {
						if related, ok := fetched[id]; ok {
							populated = append(populated, related)
						}
					}
				}
			case []string:
				for _, id := range val {
					if related, ok := fetched[id]; ok {
						populated = append(populated, related)
					}
				}
			}
			if len(populated) > 0 {
				data[req.Field] = populated
			}
		}

		// Write back — must convert to map[string]interface{} for the outer doc
		documents[i]["data"] = map[string]interface{}(data)
	}

	return nil
}

func (r *RelationshipResolver) populateReverseField(documents []map[string]interface{}, projectID, fieldName string, selectFields, nestedPopulates []string) error {
	col, err := r.getCollectionCached(projectID, fieldName)
	if err != nil || col == nil {
		return nil
	}

	singular := singularize(fieldName)
	foreignKey := singular + "_id"

	parentIDs := make([]string, 0, len(documents))
	idxByID := make(map[string]int, len(documents))
	for i, doc := range documents {
		if id, ok := doc["id"].(string); ok && id != "" {
			parentIDs = append(parentIDs, id)
			idxByID[id] = i
		}
	}
	if len(parentIDs) == 0 {
		return nil
	}

	var related []models.Document
	r.db.Where("collection_id = ? AND data->>? IN ?", col.ID, foreignKey, parentIDs).Find(&related)

	grouped := make(map[string][]map[string]interface{})
	for _, d := range related {
		parentID, _ := d.Data[foreignKey].(string)
		if parentID == "" {
			continue
		}
		grouped[parentID] = append(grouped[parentID], documentToRelatedMap(&d, selectFields))
	}

	if len(nestedPopulates) > 0 {
		allNested := make([]map[string]interface{}, 0)
		for _, group := range grouped {
			allNested = append(allNested, group...)
		}
		if len(allNested) > 0 {
			nestedReqs := ParsePopulateParams(strings.Join(nestedPopulates, ","), "")
			_ = r.PopulateDocuments(allNested, projectID, nestedReqs)
		}
	}

	for parentID, results := range grouped {
		if idx, ok := idxByID[parentID]; ok {
			data := getDataFromDoc(documents[idx])
			if data != nil {
				data[fieldName] = results
				documents[idx]["data"] = map[string]interface{}(data)
			}
		}
	}

	return nil
}

// ─────────────────────────────────────────
// Fetchers
// ─────────────────────────────────────────

func (r *RelationshipResolver) fetchDocumentsFromCollection(ids []string, projectID, collectionName string, selectFields []string) (map[string]map[string]interface{}, error) {
	col, err := r.getCollectionCached(projectID, collectionName)
	if err != nil || col == nil {
		return nil, nil
	}

	var documents []models.Document
	if err := r.db.Where("collection_id = ? AND id IN ?", col.ID, ids).Find(&documents).Error; err != nil {
		return nil, err
	}

	result := make(map[string]map[string]interface{}, len(documents))
	for i := range documents {
		result[documents[i].ID] = documentToRelatedMap(&documents[i], selectFields)
	}
	return result, nil
}

func (r *RelationshipResolver) fetchAppUsers(ids []string, projectID string, selectFields []string) (map[string]map[string]interface{}, error) {
	var appUsers []models.AppUser
	if err := r.db.Where("client_id = ? AND id IN ?", projectID, ids).Find(&appUsers).Error; err != nil {
		return nil, err
	}

	result := make(map[string]map[string]interface{}, len(appUsers))
	for _, user := range appUsers {
		userMap := map[string]interface{}{
			"id":         user.ID,
			"email":      user.Email,
			"created_at": user.CreatedAt.Format(time.RFC3339),
		}
		if len(selectFields) > 0 {
			for _, field := range selectFields {
				switch strings.TrimSpace(field) {
				case "roles":
					userMap["roles"] = []string(user.Roles)
				case "email", "id", "created_at":
				default:
					if val, ok := user.Data[field]; ok {
						userMap[field] = val
					}
				}
			}
		} else {
			userMap["roles"] = []string(user.Roles)
			for k, v := range user.Data {
				if k != "password" {
					userMap[k] = v
				}
			}
		}
		result[user.ID] = userMap
	}
	return result, nil
}

// ─────────────────────────────────────────
// Collection cache
// ─────────────────────────────────────────

func (r *RelationshipResolver) getCollectionCached(projectID, name string) (*models.Collection, error) {
	key := projectID + ":" + name
	if v, ok := r.colCache.Load(key); ok {
		return v.(*models.Collection), nil
	}

	var col models.Collection
	err := r.db.Where(
		"project_id = ? AND (name = ? OR name = ? OR name = ?)",
		projectID, name, pluralize(name), singularize(name),
	).First(&col).Error
	if err != nil {
		return nil, err
	}

	r.colCache.Store(key, &col)
	time.AfterFunc(5*time.Minute, func() {
		r.colCache.Delete(key)
	})
	return &col, nil
}

// ─────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────

// getDataFromDoc extracts the "data" field from a document map,
// handling both map[string]interface{} and models.JSONMap types.
// models.JSONMap is defined as `type JSONMap map[string]interface{}`
// but Go type assertions are exact — you must handle both.
func getDataFromDoc(doc map[string]interface{}) map[string]interface{} {
	raw, ok := doc["data"]
	if !ok || raw == nil {
		return nil
	}
	// Try plain map first
	if m, ok := raw.(map[string]interface{}); ok {
		return m
	}
	// Try models.JSONMap (type JSONMap map[string]interface{})
	if m, ok := raw.(models.JSONMap); ok {
		return map[string]interface{}(m)
	}
	return nil
}

func IsUserField(field string) bool {
	fieldLower := strings.ToLower(field)
	for _, uf := range []string{"user", "author", "creator", "owner", "assignee", "created_by", "updated_by", "assigned_to"} {
		if fieldLower == uf || strings.HasPrefix(fieldLower, uf+"_") {
			return true
		}
	}
	return false
}

func isUserField(field string) bool { return IsUserField(field) }

func pluralize(singular string) string {
	s := strings.ToLower(singular)
	specials := map[string]string{
		"person": "people", "child": "children", "tooth": "teeth",
		"foot": "feet", "mouse": "mice", "goose": "geese",
		"man": "men", "woman": "women",
		"category": "categories", "company": "companies",
	}
	if p, ok := specials[s]; ok {
		return p
	}
	if strings.HasSuffix(s, "y") && len(s) > 1 {
		if strings.ContainsRune("bcdfghjklmnpqrstvwxz", rune(s[len(s)-2])) {
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") || strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}

func singularize(word string) string {
	switch {
	case strings.HasSuffix(word, "ies"):
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(word, "ses"):
		return word[:len(word)-2]
	case strings.HasSuffix(word, "s"):
		return word[:len(word)-1]
	}
	return word
}

func SelectFields(doc map[string]interface{}, selectFields []string) map[string]interface{} {
	if len(selectFields) == 0 {
		return doc
	}
	result := make(map[string]interface{})
	if id, ok := doc["id"]; ok {
		result["id"] = id
	}
	if ca, ok := doc["created_at"]; ok {
		result["created_at"] = ca
	}
	for _, fieldPath := range selectFields {
		if fieldPath = strings.TrimSpace(fieldPath); fieldPath == "" {
			continue
		}
		parts := strings.Split(fieldPath, ".")
		if val, ok := getNestedValue(doc, parts); ok {
			setNestedValue(result, parts, val)
		}
	}
	return result
}

func getNestedValue(obj map[string]interface{}, path []string) (interface{}, bool) {
	var cur interface{} = obj
	for _, key := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func setNestedValue(obj map[string]interface{}, path []string, value interface{}) {
	for i, key := range path {
		if i == len(path)-1 {
			obj[key] = value
			return
		}
		if _, ok := obj[key]; !ok {
			obj[key] = make(map[string]interface{})
		}
		if next, ok := obj[key].(map[string]interface{}); ok {
			obj = next
		}
	}
}

func documentToRelatedMap(doc *models.Document, selectFields []string) map[string]interface{} {
	data := make(map[string]interface{}, len(doc.Data))
	if len(selectFields) > 0 {
		for _, f := range selectFields {
			if val, ok := doc.Data[f]; ok {
				data[f] = val
			}
		}
	} else {
		for k, v := range doc.Data {
			data[k] = v
		}
	}
	return map[string]interface{}{
		"id":         doc.ID,
		"data":       data,
		"created_at": doc.CreatedAt.Format(time.RFC3339),
	}
}

func mergeToSlice(a, b map[string]bool) []string {
	result := make([]string, 0, len(a)+len(b))
	for k := range a {
		result = append(result, k)
	}
	for k := range b {
		if !a[k] {
			result = append(result, k)
		}
	}
	return result
}
