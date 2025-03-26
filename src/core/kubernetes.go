package core

type MoKubernetes interface {
	Initialize()
	Run()
}

type moKubernetes struct{}

func NewMoKubernetes() MoKubernetes {
	self := &moKubernetes{}

	return self
}

func (self *moKubernetes) Initialize() {}

func (self *moKubernetes) Run() {}
