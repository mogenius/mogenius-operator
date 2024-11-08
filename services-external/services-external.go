package servicesExternal

import "mogenius-k8s-manager/interfaces"

var config interfaces.ConfigModule

func Setup(configModule interfaces.ConfigModule) {
	config = configModule
}
