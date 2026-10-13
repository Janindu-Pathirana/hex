package internal

import "time"

const CacheDir = "hex"

type CacheConfig struct {
	CacheFile string
	CacheTTL  time.Duration
}

// SecuritiesCache holds the parsed company symbol list.
var SecuritiesCache = CacheConfig{
	CacheFile: "securities.json",
	CacheTTL:  24 * time.Hour,
}

// legacyAvailableCompaniesCache is the pre-JSON symbol list file. It is only
// listed so that "hex clean all" removes it.
var legacyAvailableCompaniesCache = CacheConfig{CacheFile: "available_companies.txt"}

// AllCaches is every cache file hex writes; "hex clean all" removes them.
var AllCaches = []CacheConfig{SecuritiesCache, legacyAvailableCompaniesCache}
