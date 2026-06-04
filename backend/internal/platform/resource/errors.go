package resource

import "errors"

// ErrDeferred indicates work should be retried later due to resource pressure.
var ErrDeferred = errors.New("resource deferred")
