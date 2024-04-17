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
	Id                 string   `json:"id"`
	DisplayName        string   `json:"displayName"`
	CreatedBy          string   `json:"createdBy"`
	ProductId          string   `json:"productId"`
	ClusterId          string   `json:"clusterId"`
	GitConnectionId    string   `json:"gitConnectionId"`
	ApplicationKitRefs []string `json:"applicationKitRefs"`
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
