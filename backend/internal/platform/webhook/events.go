package webhook

const (
	EventUploadCompleted   = "media.upload.completed"
	EventConvertStarted    = "video.convert.started"
	EventConvertCompleted  = "video.convert.completed"
	EventConvertFailed     = "video.convert.failed"
	EventMediaDeleted      = "media.deleted"
)

var AllEvents = []string{
	EventUploadCompleted,
	EventConvertStarted,
	EventConvertCompleted,
	EventConvertFailed,
	EventMediaDeleted,
}

func ValidEvent(name string) bool {
	for _, e := range AllEvents {
		if e == name {
			return true
		}
	}
	return false
}
