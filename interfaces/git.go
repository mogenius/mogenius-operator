package interfaces

type Git interface {
	GetLocalPath() string
	GetRemoteUrl() string
	SetRemoteUrl(remoteUrl string) error
	GetBranch() string
	SetBranch(branch string) error
	RestoreRemoteState() error
	Init() error
	Pull() error
	Push() error
}
