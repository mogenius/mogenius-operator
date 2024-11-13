package utils_test

import (
	"mogenius-k8s-manager/utils"
	"testing"
)

func TestUtilsConfig(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	conf, err := utils.PrintCurrentCONFIG()
	if err != nil {
		t.Errorf("Error printing CONFIG: %s", err.Error())
	} else {
		t.Logf("\n%s", conf)
	}
}
