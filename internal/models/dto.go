package models

// CollectionPermissionsRequest represents permissions for CRUD operations
type CollectionPermissionsRequest struct {
	Create []string `json:"create"`
	Read   []string `json:"read"`
	Update []string `json:"update"`
	Delete []string `json:"delete"`
}

// CollectionCreateRequest represents the request to create a collection
type CollectionCreateRequest struct {
	Name        string                        `json:"name" validate:"required"`
	WebhookURL  *string                       `json:"webhook_url,omitempty"`
	Permissions *CollectionPermissionsRequest `json:"permissions,omitempty"`
}

// CollectionUpdateRequest represents the request to update a collection
type CollectionUpdateRequest struct {
	Name *string `json:"name,omitempty"`
}

// DocumentCreateRequest represents the request to create a document
type DocumentCreateRequest struct {
	Data map[string]interface{} `json:"data" validate:"required"`
}

// DocumentUpdateRequest represents the request to update a document
type DocumentUpdateRequest struct {
	Data map[string]interface{} `json:"data" validate:"required"`
}

// BatchDeleteRequest represents the request to delete multiple documents
type BatchDeleteRequest struct {
	IDs []string `json:"ids" validate:"required"`
}

// BatchCreateRequest represents the request to create multiple documents
type BatchCreateRequest struct {
	Documents []map[string]interface{} `json:"documents" validate:"required"`
}

// BatchUpdateRequest for batch updating documents
type BatchUpdateRequest struct {
	Updates []struct {
		ID   string                 `json:"id"`
		Data map[string]interface{} `json:"data"`
	} `json:"updates" validate:"required"`
}

// CollectionResponse represents a collection with its metadata
type CollectionResponse struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ProjectID   string      `json:"project_id"`
	WebhookURL  *string     `json:"webhook_url,omitempty"`
	Permissions Permissions `json:"permissions"`
	CreatedAt   string      `json:"created_at"`
}

// DocumentResponse represents a document with its data
type DocumentResponse struct {
	ID           string                 `json:"id"`
	CollectionID string                 `json:"collection_id"`
	Data         map[string]interface{} `json:"data"`
	CreatedAt    string                 `json:"created_at"`
}

// PaginatedDocumentsResponse represents paginated document results
type PaginatedDocumentsResponse struct {
	Data    []map[string]interface{} `json:"data"`
	Total   int64                    `json:"total"`
	Limit   int                      `json:"limit"`
	Offset  int                      `json:"offset"`
	HasMore bool                     `json:"has_more"`
}

// CountResponse represents a count result
type CountResponse struct {
	Count int64 `json:"count"`
}

// AggregateResponse represents an aggregation result
type AggregateResponse struct {
	Field      string      `json:"field"`
	Operation  string      `json:"operation"`
	Result     interface{} `json:"result"`
	Collection string      `json:"collection"`
}

// GroupByResponse represents a group by result
type GroupByItem struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// SchemaResponse represents inferred collection schema
type SchemaResponse struct {
	Collection            string                      `json:"collection"`
	DocumentCount         int64                       `json:"document_count"`
	AnalyzedDocuments     int                         `json:"analyzed_documents"`
	Fields                map[string]FieldInfo        `json:"fields"`
	DetectedRelationships map[string]RelationshipInfo `json:"detected_relationships"`
	UsageHint             string                      `json:"usage_hint"`
}

// FieldInfo represents information about a field
type FieldInfo struct {
	Types          []string      `json:"types"`
	PrimaryType    string        `json:"primary_type"`
	Nullable       bool          `json:"nullable"`
	NullPercentage float64       `json:"null_percentage"`
	Samples        []interface{} `json:"samples"`
}

// RelationshipInfo represents detected relationship information
type RelationshipInfo struct {
	Type             string `json:"type"`
	Field            string `json:"field"`
	TargetCollection string `json:"target_collection"`
	Confidence       string `json:"confidence"`
}
