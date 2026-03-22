package internal

import "os"

const (
	DefaultTagPath     = "./tags"
	DefaultCachePath   = "./cache"
	DefaultTagExport   = "tags-export.json"
	DefaultStatsExport = "stats-export.json"
	StashDBURL         = "https://stashdb.org/graphql"
)

var ExcludePrefixes = []string{"r:", "c:", ".", "stashdb", "Figure", "["}

type Config struct {
	StashAPIKey     string
	StashURL        string
	TagPath         string
	CachePath       string
	TagExportPath   string
	StatsExportPath string
	Port            string
	StashDBAPIKey   string
}

func LoadConfig() *Config {
	cachePath := getEnv("CACHE_PATH", DefaultCachePath)
	return &Config{
		StashAPIKey:     os.Getenv("STASH_APIKEY"),
		StashURL:        os.Getenv("STASH_URL"),
		TagPath:         getEnv("TAG_PATH", DefaultTagPath),
		CachePath:       cachePath,
		TagExportPath:   getEnv("TAG_EXPORT_PATH", cachePath+"/"+DefaultTagExport),
		StatsExportPath: getEnv("STATS_EXPORT_PATH", cachePath+"/"+DefaultStatsExport),
		Port:            getEnv("PORT", "3000"),
		StashDBAPIKey:   os.Getenv("STASHDB_APIKEY"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
