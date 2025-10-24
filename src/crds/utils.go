package crds

import (
	"embed"
	"mogenius-k8s-manager/src/assert"
	"path"
	"strings"
)

//go:embed yaml/*
var content embed.FS

type CRD struct {
	Filename string
	Content  string
}

func GetCRDs() []CRD {
	yamlDir := "yaml"
	yamls := []CRD{}

	files, err := content.ReadDir(yamlDir)
	assert.Assert(err == nil, "embedded directory should be readable", err)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "yaml") {
			filepath := path.Join(yamlDir, file.Name())
			content, err := content.ReadFile(filepath)
			assert.Assert(err == nil, "embedded file should be readable", file.Name(), err)
			crd := string(content)
			yamls = append(yamls, CRD{Filename: file.Name(), Content: crd})
		}
	}

	return yamls
}
