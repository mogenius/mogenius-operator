package structs

type JobStateEnum string

const (
	JobStateFailed    JobStateEnum = "FAILED"
	JobStateSucceeded JobStateEnum = "SUCCEEDED"
	JobStateStarted   JobStateEnum = "STARTED"
	JobStatePending   JobStateEnum = "PENDING"
	JobStateCanceled  JobStateEnum = "CANCELED"
	JobStateTimeout   JobStateEnum = "TIMEOUT"
)

type HelmGetEnum string

const (
	HelmGetAll      HelmGetEnum = "all"
	HelmGetHooks    HelmGetEnum = "hooks"
	HelmGetManifest HelmGetEnum = "manifest"
	HelmGetNotes    HelmGetEnum = "notes"
	HelmGetValues   HelmGetEnum = "values"
)

