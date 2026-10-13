package httpclient

import (
	"net/http"
	"time"
)

// Default is the shared HTTP client for outbound requests.
var Default = &http.Client{Timeout: 30 * time.Second}

// Download is the shared HTTP client for large files such as report PDFs.
var Download = &http.Client{Timeout: 5 * time.Minute}
