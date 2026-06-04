package storage

// ObjectInfo describes a stored object (from HeadObject / GetObject).
type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}
