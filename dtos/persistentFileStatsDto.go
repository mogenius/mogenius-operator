package dtos

type PersistentFileStatsDto struct {
	Objects     interface{} `json:"objects"`
	Sum         int         `json:"sum"`
	DurationMs  int         `json:"durationMs,omitempty"`
	GeneratedAt string      `json:"generatedAt,omitempty"`
}
