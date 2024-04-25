package crds

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	MogeniusGroup                  = "mogenius.io"
	MogeniusVersion                = "v1"
	MogeniusResourceProject        = "projects"
	MogeniusKindProject            = "Project"
	MogeniusResourceApplicationKit = "applicationkits"
	MogeniusKindApplicationKit     = "ApplicationKit"
)

type CrdProject struct {
	Id                 string        `json:"id"`
	DisplayName        string        `json:"displayName"`
	CreatedBy          string        `json:"createdBy"`
	ProductId          string        `json:"productId"`
	ClusterId          string        `json:"clusterId"`
	GitConnectionId    string        `json:"gitConnectionId"`
	ApplicationKitRefs []string      `json:"applicationKitRefs"`
	Limits             ProjectLimits `json:"limits"`
}

type ProjectLimits struct {
	MemoryLimitInMb     int     `json:"memoryLimitInMb"`
	MemoryLimitInCores  float64 `json:"memoryLimitInCores"`
	EphmeralStorageInMb int     `json:"ephmeralStorageInMb"`
}

func CrdProjectExampleData() CrdProject {
	return CrdProject{
		Id:                 "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:        "displayName",
		CreatedBy:          "createdBy",
		ProductId:          "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ClusterId:          "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		GitConnectionId:    "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ApplicationKitRefs: []string{},
	}
}

func (p *CrdProject) ToUnstructuredProject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", MogeniusGroup, MogeniusVersion),
			"kind":       MogeniusKindProject,
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"id":                 p.Id,
				"displayName":        p.DisplayName,
				"createdBy":          p.CreatedBy,
				"productId":          p.ProductId,
				"clusterId":          p.ClusterId,
				"gitConnectionId":    p.GitConnectionId,
				"applicationKitRefs": p.ApplicationKitRefs,
			},
		},
	}
}

type CrdApplicationKit struct {
	Id          string `json:"id"`
	DisplayName string `json:"displayName"`
	CreatedBy   string `json:"createdBy"`
	Controller  string `json:"controller"`
	AppId       string `json:"appId"`
}

func (p *CrdApplicationKit) ToUnstructuredApplicationKit(namespace string, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", MogeniusGroup, MogeniusVersion),
			"kind":       MogeniusKindApplicationKit,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"id":          p.Id,
				"displayName": p.DisplayName,
				"createdBy":   p.CreatedBy,
				"controller":  p.Controller,
				"appId":       p.AppId,
			},
		},
	}
}

type CrdGitConnection struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}
