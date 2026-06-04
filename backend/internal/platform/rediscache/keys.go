package rediscache

import "time"

const (
	PrefixAuthUser   = "cache:auth:user:"
	PrefixStreamVideo = "cache:stream:video:"

	KeySettingsV1 = "cache:settings:v1"

	TTLAuthUser    = 60 * time.Second
	TTLSettings    = 30 * time.Second
	TTLStreamVideo = 45 * time.Second

	maxScanIterations = 100
	scanCount         = 100
)

func KeyAuthUser(publicID string) string {
	return PrefixAuthUser + publicID
}

func KeyStreamVideo(videoPublicID string) string {
	return PrefixStreamVideo + videoPublicID
}
