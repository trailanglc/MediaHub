package storage

// ObjectInfo describes a stored object (from HeadObject / GetObject).
type ObjectInfo struct {
	// Size is the number of bytes in the (possibly partial) response body.
	Size int64
	// TotalSize is the full object size. Equals Size for a non-range response.
	TotalSize   int64
	ContentType string
	ETag        string
	// ContentRange is the raw Content-Range header for a 206 partial response (empty otherwise).
	ContentRange string
}
