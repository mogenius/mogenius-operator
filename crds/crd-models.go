package crds

import (
	"fmt"
	"mogenius-k8s-manager/logging"
	"reflect"
	"unicode"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var CrdLogger = logging.CreateLogger("crds")

const (
	MogeniusGroup                  = "mogenius.io"
	MogeniusVersion                = "v1"
	MogeniusResourceProject        = "projects"
	MogeniusKindProject            = "Project"
	MogeniusResourceEnvironment    = "environments"
	MogeniusKindEnvironment        = "Environment"
	MogeniusResourceApplicationKit = "applicationkits"
	MogeniusKindApplicationKit     = "ApplicationKit"
)

type CrdProject struct {
	Id              string        `json:"id"`
	ProjectName     string        `json:"projectName"`
	DisplayName     string        `json:"displayName"`
	CreatedBy       string        `json:"createdBy"`
	ProductId       string        `json:"productId"`
	ClusterId       string        `json:"clusterId"`
	EnvironmentRefs []string      `json:"environmentRefs"`
	Limits          ProjectLimits `json:"limits"`
}

type ProjectLimits struct {
	LimitMemoryMB      int     `json:"limitMemoryMB"`
	LimitCpuCores      float64 `json:"limitCpuCores"`
	EphemeralStorageMB int     `json:"ephemeralStorageMB"`
	MaxVolumeSizeGb    int     `json:"maxVolumeSizeGb"`
}

func CrdProjectExampleData() CrdProject {
	return CrdProject{
		Id:              "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:     "displayName",
		CreatedBy:       "createdBy",
		ProductId:       "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ClusterId:       "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		EnvironmentRefs: []string{},
	}
}

func (p *CrdProject) ToUnstructuredProject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: topLevelStructToMap(nil, name, MogeniusKindProject, p),
	}
}

type CrdEnvironment struct {
	Id                 string   `json:"id" validate:"required"`
	DisplayName        string   `json:"displayName" validate:"required"`
	Name               string   `json:"name" validate:"required"`
	CreatedBy          string   `json:"createdBy"`
	ApplicationKitRefs []string `json:"applicationKitRefs"`
}

func (p *CrdEnvironment) ToUnstructuredEnvironment(namespace string, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: topLevelStructToMap(&namespace, name, MogeniusKindEnvironment, p),
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
		Object: topLevelStructToMap(&namespace, name, MogeniusKindApplicationKit, p),
	}
}

type CrdGitConnection struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func topLevelStructToMap(namespace *string, name string, kind string, data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": fmt.Sprintf("%s/%s", MogeniusGroup, MogeniusVersion),
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": structToMap(data),
	}
}

func structToMap(obj interface{}) map[string]interface{} {
	mapObj := make(map[string]interface{})
	val := reflect.ValueOf(obj)

	// Check if the passed object is a pointer and resolve its value
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := field.Type()

		if fieldType.Kind() == reflect.Struct {
			// Recursively handle nested structs
			mapObj[lowerFirst(typ.Field(i).Name)] = structToMap(field.Interface())
		} else {
			// Directly set the value
			mapObj[lowerFirst(typ.Field(i).Name)] = field.Interface()
		}
	}

	return mapObj
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
