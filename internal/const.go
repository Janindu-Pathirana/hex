package internal

import "time"

const CacheDir = "hex"

type CacheConfig struct {
	CacheFile string
	CacheTTL  time.Duration
}

var AvailableCompaniesCache = CacheConfig{
	CacheFile: "available_companies.txt",
	CacheTTL:  24 * time.Hour,
}
