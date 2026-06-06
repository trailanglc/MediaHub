package rediscache

import "time"

const (
	PrefixAuthUser    = "cache:auth:user:"
	PrefixAPIKeyHash  = "cache:apikey:hash:"
	PrefixStreamVideo = "cache:stream:video:"

	KeySettingsV1        = "cache:settings:v1"
	KeyHomepagePublicV1  = "cache:homepage:public:v1"

	// ChannelStreamInvalidate broadcasts video public IDs whose cached stream data
	// (including per-process playlist bodies) must be dropped across all replicas.
	ChannelStreamInvalidate = "cache:stream:invalidate"

	TTLAuthUser    = 60 * time.Second
	TTLAPIKey      = 5 * time.Minute
	TTLSettings    = 30 * time.Second
	TTLStreamVideo = 45 * time.Second
	// TTLHomepagePublic — 0 = no expiry; invalidated when homepage settings change.
	TTLHomepagePublic = 0

	maxScanIterations = 100
	scanCount         = 100
)

func KeyAuthUser(publicID string) string {
	return PrefixAuthUser + publicID
}

func KeyAPIKeyHash(keyHash string) string {
	return PrefixAPIKeyHash + keyHash
}

func KeyStreamVideo(videoPublicID string) string {
	return PrefixStreamVideo + videoPublicID
}
