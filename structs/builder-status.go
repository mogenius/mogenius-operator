package structs

type BuilderStatus struct {
	TotalBuilds      int `json:"totalBuilds"`
	TotalBuildTimeMs int `json:"totalBuildTime"`
	QueuedBuilds     int `json:"queuedBuilds"`
}
