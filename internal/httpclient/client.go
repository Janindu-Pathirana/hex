package httpclient

import (
	"net/http"
	"time"
)

// Default is the shared HTTP client for outbound requests.
var Default = &http.Client{Timeout: 30 * time.Second}
