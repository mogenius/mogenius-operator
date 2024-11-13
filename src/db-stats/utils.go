package dbstats

import "time"

func isMoreThan14DaysOld(startTimeStr string) bool {
	// Parse the start time string into a time.Time object
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return true
	}

	// Get the current time
	currentTime := time.Now()

	// Check if startTime is more than 14 days ago
	return startTime.Before(currentTime.Add(-14 * (time.Hour * 24)))
}
