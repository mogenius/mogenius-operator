package dtos

import "time"

type PersistentFileStatsDto struct {
	Objects     interface{} `json:"objects"`
	Sum         int         `json:"sum"`
	DurationMs  int         `json:"durationMs,omitempty"`
	GeneratedAt string      `json:"generatedAt,omitempty"`
}

func PersistentFileStatsDtoExampleData() PersistentFileStatsDto {
	return PersistentFileStatsDto{
		Objects:     "objects",
		Sum:         1,
		DurationMs:  1,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
}
