package utils_test

import (
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestUtilsConfig(t *testing.T) {
	t.Parallel()
	conf, err := utils.PrintCurrentCONFIG()
	if err != nil {
		t.Errorf("Error printing CONFIG: %s", err.Error())
	} else {
		t.Logf("\n%s", conf)
	}
}
