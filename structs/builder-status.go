package structs

type BuilderStatus struct {
	TotalBuilds      int `json:"totalBuilds"`
	TotalBuildTimeMs int `json:"totalBuildTime"`
	QueuedBuilds     int `json:"queuedBuilds"`
	FailedBuilds     int `json:"FailedBuilds"`
	FinishedBuilds   int `json:"finishedBuilds"`
	CanceledBuilds   int `json:"canceledBuilds"`
	TotalScans       int `json:"totalScans"`
}
