package utils

import "testing"

func TestUtilsConfig(t *testing.T) {
	conf, err := PrintCurrentCONFIG()
	if err != nil {
		t.Errorf("Error printing CONFIG: %s", err.Error())
	} else {
		t.Logf("\n%s", conf)
	}
}
