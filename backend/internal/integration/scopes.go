package integration

const (
	ScopeStream       = "stream"
	ScopeMediaUpload  = "media:upload"
	ScopeMediaRead    = "media:read"
	ScopeMediaConvert = "media:convert"
	ScopeMediaDelete  = "media:delete"
)

var KnownScopes = []string{
	ScopeStream,
	ScopeMediaUpload,
	ScopeMediaRead,
	ScopeMediaConvert,
	ScopeMediaDelete,
}

func HasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

func HasAnyScope(scopes []string, wants ...string) bool {
	for _, w := range wants {
		if HasScope(scopes, w) {
			return true
		}
	}
	return false
}
