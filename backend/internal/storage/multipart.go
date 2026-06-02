package storage

// CompletedPart is one part of a finished multipart upload.
type CompletedPart struct {
	PartNumber int32
	ETag       string
}
