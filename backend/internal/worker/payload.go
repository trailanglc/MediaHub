package worker

const TypeVideoConvert = "video:convert"
const TypeDownloadFetch = "download:fetch"

type ConvertPayload struct {
	JobPublicID   string   `json:"job_public_id"`
	VideoPublicID string   `json:"video_public_id"`
	ObjectID      int64    `json:"object_id"`
	Variants      []string `json:"variants,omitempty"`
	RequestID     string   `json:"request_id,omitempty"`
}

type DownloadPayload struct {
	JobPublicID string `json:"job_public_id"`
	RequestID   string `json:"request_id,omitempty"`
}
