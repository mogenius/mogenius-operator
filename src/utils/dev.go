package utils

// SEEDED BY COMPILER FLAG
//
// go build -X 'mogenius-k8s-manager/src/utils.DevBuild=yes' ./...
var DevBuild string = "no"

// Used to enable features strictly for dev-builds.
func IsDevBuild() bool {
	return DevBuild == "yes"
}
