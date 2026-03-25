package utils

import (
	"sync"
)

// Pool for reusing map allocations in document conversions
var DocumentMapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]interface{}, 16) // Pre-allocate for common field count
	},
}

// Pool for reusing byte slices in JSON marshaling
var ByteSlicePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 512) // 512 byte initial capacity
		return &b
	},
}

// GetDocumentMap gets a map from the pool
func GetDocumentMap() map[string]interface{} {
	return DocumentMapPool.Get().(map[string]interface{})
}

// PutDocumentMap returns a map to the pool
func PutDocumentMap(m map[string]interface{}) {
	// Clear the map before returning to pool
	for k := range m {
		delete(m, k)
	}
	DocumentMapPool.Put(m)
}

// GetByteSlice gets a byte slice from the pool
func GetByteSlice() *[]byte {
	return ByteSlicePool.Get().(*[]byte)
}

// PutByteSlice returns a byte slice to the pool
func PutByteSlice(b *[]byte) {
	*b = (*b)[:0] // Reset slice but keep capacity
	ByteSlicePool.Put(b)
}
