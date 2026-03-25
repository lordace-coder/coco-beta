package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/patrick/cocobase/internal/models"
	"gorm.io/gorm"
)

// NewQueryContext creates a context with timeout in milliseconds
func NewQueryContext(timeoutMs int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
}

// ExecuteQueryFast just executes the query fast without any count
// This is what you want 99% of the time - no unnecessary COUNT queries!
func ExecuteQueryFast(ctx context.Context, query *gorm.DB) ([]models.Document, error) {
	var documents []models.Document
	if err := query.WithContext(ctx).Find(&documents).Error; err != nil {
		return nil, err
	}
	return documents, nil
}

// ConcurrentRelationshipPopulator handles parallel relationship population
type ConcurrentRelationshipPopulator struct {
	db          *gorm.DB
	workerCount int
	timeout     time.Duration
}

// NewConcurrentRelationshipPopulator creates a new concurrent populator
func NewConcurrentRelationshipPopulator(db *gorm.DB, workerCount int) *ConcurrentRelationshipPopulator {
	if workerCount <= 0 {
		workerCount = 4 // Default to 4 workers
	}
	return &ConcurrentRelationshipPopulator{
		db:          db,
		workerCount: workerCount,
		timeout:     30 * time.Second, // 30 second timeout per relationship
	}
}

// PopulateJob represents a single populate task
type PopulateJob struct {
	Documents       []map[string]interface{}
	ProjectID       string
	PopulateRequest PopulateRequest
	ResultChan      chan error
}

// PopulateAllConcurrently populates multiple relationships in parallel
func (p *ConcurrentRelationshipPopulator) PopulateAllConcurrently(documents []map[string]interface{}, projectID string, populateRequests []PopulateRequest) error {
	if len(populateRequests) == 0 {
		return nil
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout*time.Duration(len(populateRequests)))
	defer cancel()

	// Create job channel
	jobs := make(chan PopulateJob, len(populateRequests))
	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go p.worker(ctx, &wg, jobs)
	}

	// Send jobs
	resultChans := make([]chan error, len(populateRequests))
	for i, req := range populateRequests {
		resultChan := make(chan error, 1)
		resultChans[i] = resultChan

		jobs <- PopulateJob{
			Documents:       documents,
			ProjectID:       projectID,
			PopulateRequest: req,
			ResultChan:      resultChan,
		}
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	// Collect errors (non-blocking)
	var lastError error
	for _, resultChan := range resultChans {
		select {
		case err := <-resultChan:
			if err != nil {
				lastError = err // Store last error but continue
			}
		default:
			// No error received
		}
	}

	return lastError
}

// worker processes populate jobs from the job channel
func (p *ConcurrentRelationshipPopulator) worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan PopulateJob) {
	defer wg.Done()

	resolver := &RelationshipResolver{db: p.db}

	for job := range jobs {
		select {
		case <-ctx.Done():
			job.ResultChan <- ctx.Err()
			return
		default:
			// Execute the populate task
			err := resolver.populateField(job.Documents, job.ProjectID, job.PopulateRequest)
			job.ResultChan <- err
		}
	}
}

// BatchIDFetcher fetches related data in batches using goroutines
type BatchIDFetcher struct {
	db        *gorm.DB
	batchSize int
}

// NewBatchIDFetcher creates a new batch ID fetcher
func NewBatchIDFetcher(db *gorm.DB, batchSize int) *BatchIDFetcher {
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}
	return &BatchIDFetcher{
		db:        db,
		batchSize: batchSize,
	}
}

// FetchInBatches fetches data in batches concurrently
func (f *BatchIDFetcher) FetchInBatches(ids []string, fetchFunc func([]string) ([]interface{}, error)) ([]interface{}, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Calculate number of batches
	numBatches := (len(ids) + f.batchSize - 1) / f.batchSize

	// Create channels for results
	type batchResult struct {
		data []interface{}
		err  error
	}
	resultChan := make(chan batchResult, numBatches)
	var wg sync.WaitGroup

	// Process each batch concurrently
	for i := 0; i < numBatches; i++ {
		start := i * f.batchSize
		end := start + f.batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		wg.Add(1)
		go func(batchIDs []string) {
			defer wg.Done()
			data, err := fetchFunc(batchIDs)
			resultChan <- batchResult{data: data, err: err}
		}(batch)
	}

	// Close result channel after all goroutines finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var allResults []interface{}
	var lastError error

	for result := range resultChan {
		if result.err != nil {
			lastError = result.err
			continue
		}
		allResults = append(allResults, result.data...)
	}

	return allResults, lastError
}

// ParallelDocumentProcessor processes documents in parallel chunks
type ParallelDocumentProcessor struct {
	workerCount int
}

// NewParallelDocumentProcessor creates a new parallel processor
func NewParallelDocumentProcessor(workerCount int) *ParallelDocumentProcessor {
	if workerCount <= 0 {
		workerCount = 4
	}
	return &ParallelDocumentProcessor{
		workerCount: workerCount,
	}
}

// ProcessChunk represents a chunk of documents to process
type ProcessChunk struct {
	Documents []models.Document
	StartIdx  int
	EndIdx    int
}

// ProcessDocumentsParallel processes documents in parallel using a worker pool
func (p *ParallelDocumentProcessor) ProcessDocumentsParallel(
	documents []models.Document,
	processFunc func(*models.Document) map[string]interface{},
) []map[string]interface{} {
	if len(documents) == 0 {
		return nil
	}

	// Pre-allocate result slice
	results := make([]map[string]interface{}, len(documents))
	var wg sync.WaitGroup

	// Calculate chunk size
	chunkSize := (len(documents) + p.workerCount - 1) / p.workerCount

	// Process chunks in parallel
	for i := 0; i < p.workerCount; i++ {
		start := i * chunkSize
		if start >= len(documents) {
			break
		}
		end := start + chunkSize
		if end > len(documents) {
			end = len(documents)
		}

		wg.Add(1)
		go func(startIdx, endIdx int) {
			defer wg.Done()
			for j := startIdx; j < endIdx; j++ {
				results[j] = processFunc(&documents[j])
			}
		}(start, end)
	}

	wg.Wait()
	return results
}

// CachedRelationshipFetcher caches relationship queries with short TTL
type CachedRelationshipFetcher struct {
	cache    map[string]*relationshipCacheEntry
	mutex    sync.RWMutex
	cacheTTL time.Duration
	db       *gorm.DB
}

type relationshipCacheEntry struct {
	data      map[string]map[string]interface{}
	expiresAt time.Time
}

// NewCachedRelationshipFetcher creates a new cached fetcher
func NewCachedRelationshipFetcher(db *gorm.DB, ttl time.Duration) *CachedRelationshipFetcher {
	fetcher := &CachedRelationshipFetcher{
		cache:    make(map[string]*relationshipCacheEntry),
		cacheTTL: ttl,
		db:       db,
	}

	// Start background cleanup
	go fetcher.cleanupExpired()

	return fetcher
}

// cleanupExpired removes expired cache entries periodically
func (f *CachedRelationshipFetcher) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		f.mutex.Lock()
		for key, entry := range f.cache {
			if now.After(entry.expiresAt) {
				delete(f.cache, key)
			}
		}
		f.mutex.Unlock()
	}
}

// FetchWithCache fetches relationship data with caching
func (f *CachedRelationshipFetcher) FetchWithCache(
	cacheKey string,
	fetchFunc func() (map[string]map[string]interface{}, error),
) (map[string]map[string]interface{}, error) {
	// Check cache first
	f.mutex.RLock()
	if entry, exists := f.cache[cacheKey]; exists && time.Now().Before(entry.expiresAt) {
		f.mutex.RUnlock()
		return entry.data, nil
	}
	f.mutex.RUnlock()

	// Cache miss - fetch data
	data, err := fetchFunc()
	if err != nil {
		return nil, err
	}

	// Store in cache
	f.mutex.Lock()
	f.cache[cacheKey] = &relationshipCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(f.cacheTTL),
	}
	f.mutex.Unlock()

	return data, nil
}

// ParallelSubqueryBuilder builds multiple subqueries concurrently
type ParallelSubqueryBuilder struct {
	db *gorm.DB
}

// SubqueryJob represents a subquery building task
type SubqueryJob struct {
	RelKey    string
	RelValue  string
	Condition string
	Result    *gorm.DB
	Error     error
}

// BuildSubqueriesParallel builds relationship filter subqueries in parallel
func (b *ParallelSubqueryBuilder) BuildSubqueriesParallel(
	relationshipFilters map[string]string,
	collectionID string,
	projectID string,
) ([]*gorm.DB, error) {
	if len(relationshipFilters) == 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	resultChan := make(chan SubqueryJob, len(relationshipFilters))

	// Build each subquery concurrently
	for relKey, relValue := range relationshipFilters {
		wg.Add(1)
		go func(key, value string) {
			defer wg.Done()

			job := SubqueryJob{
				RelKey:   key,
				RelValue: value,
			}

			// Parse relationship key
			parts := strings.Split(key, ".")
			if len(parts) < 2 {
				resultChan <- job
				return
			}

			relationField := parts[0]
			targetField := parts[1]

			// Extract operator
			operator := "eq"
			fieldName := targetField
			for _, op := range []string{"_contains", "_startswith", "_endswith", "_gt", "_gte", "_lt", "_lte", "_ne"} {
				if strings.HasSuffix(targetField, op) {
					operator = strings.TrimPrefix(op, "_")
					fieldName = strings.TrimSuffix(targetField, op)
					break
				}
			}

			// Only handle user relationships for now
			if !IsUserField(relationField) {
				resultChan <- job
				return
			}

			idField := relationField + "_id"

			// Build subquery
			subquery := b.db.Table("app_users").
				Select("documents.id").
				Joins("JOIN documents ON documents.data->>? = app_users.id::text", idField).
				Where("documents.collection_id = ? AND app_users.client_id = ?", collectionID, projectID)

			// Apply filter based on operator
			switch operator {
			case "contains":
				subquery = subquery.Where("LOWER(app_users."+fieldName+") LIKE ?", "%"+strings.ToLower(value)+"%")
			case "startswith":
				subquery = subquery.Where("LOWER(app_users."+fieldName+") LIKE ?", strings.ToLower(value)+"%")
			case "endswith":
				subquery = subquery.Where("LOWER(app_users."+fieldName+") LIKE ?", "%"+strings.ToLower(value))
			case "eq":
				subquery = subquery.Where("app_users."+fieldName+" = ?", value)
			case "ne":
				subquery = subquery.Where("app_users."+fieldName+" != ?", value)
			case "gt":
				subquery = subquery.Where("app_users."+fieldName+" > ?", value)
			case "gte":
				subquery = subquery.Where("app_users."+fieldName+" >= ?", value)
			case "lt":
				subquery = subquery.Where("app_users."+fieldName+" < ?", value)
			case "lte":
				subquery = subquery.Where("app_users."+fieldName+" <= ?", value)
			}

			job.Result = subquery
			resultChan <- job
		}(relKey, relValue)
	}

	// Close channel after all goroutines finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var subqueries []*gorm.DB
	for job := range resultChan {
		if job.Result != nil {
			subqueries = append(subqueries, job.Result)
		}
	}

	return subqueries, nil
}
