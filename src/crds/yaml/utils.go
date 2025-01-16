package yaml

import (
	"embed"
	_ "embed"
	"mogenius-k8s-manager/src/assert"
	"strings"
)

//go:embed *
var content embed.FS

type CRD struct {
	Filename string
	Content  string
}

func GetCRDs() []CRD {
	yamls := []CRD{}

	files, err := content.ReadDir(".")
	assert.Assert(err == nil, "embedded directory should be readable", err)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "yaml") {
			content, err := content.ReadFile(file.Name())
			assert.Assert(err == nil, "embedded file should be readable", file.Name(), err)
			crd := string(content)
			yamls = append(yamls, CRD{Filename: file.Name(), Content: crd})
		}
	}

	return yamls
}
