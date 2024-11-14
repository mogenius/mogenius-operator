package servicesExternal

import "mogenius-k8s-manager/src/interfaces"

var config interfaces.ConfigModule

func Setup(configModule interfaces.ConfigModule) {
	config = configModule
}
