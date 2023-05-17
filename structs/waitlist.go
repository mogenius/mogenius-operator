package structs

import "time"

type WaitListEntry struct {
	Job            Job
	WaitForKind    string
	WaitForReason  string
	WaitForMessage string
	CreatedAt      time.Time
	Ttl            time.Duration
}

func CreateWaitListEntry(job Job, kind string, reason string, message string, ttlInMinutes time.Duration) WaitListEntry {
	return WaitListEntry{
		Job:            job,
		WaitForKind:    kind,
		WaitForReason:  reason,
		WaitForMessage: message,
		CreatedAt:      time.Now(),
		Ttl:            time.Second * ttlInMinutes,
	}
}

func (e *WaitListEntry) IsExpired() bool {
	return time.Since(e.CreatedAt) > e.Ttl
}
