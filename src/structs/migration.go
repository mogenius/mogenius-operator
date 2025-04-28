package structs

type Migration struct {
	Id        uint64 `json:"id"`
	Name      string `json:"name"`
	AppliedAt string `json:"appliedAt"`
}
