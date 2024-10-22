package kubernetes

// import (
// 	"fmt"

// 	"gopkg.in/yaml.v2"
// 	"k8s.io/client-go/discovery"
// 	"k8s.io/client-go/rest"
// 	"k8s.io/kube-openapi/pkg/util/proto"
// )

// // GetUnstructuredExample generates a YAML example for a given Kubernetes resource.
// func GetUnstructuredExample(group, version, kind string, namespaced bool) (string, error) {
// 	// Create Kubernetes client config
// 	provider, err := NewKubeProvider(nil)
// 	if provider == nil || err != nil {
// 		K8sLogger.Errorf("Error creating provider for watcher. Cannot continue: %s", err.Error())
// 		return "", err
// 	}

// 	// Get the OpenAPI schema models
// 	models, err := getOpenAPISchema(&provider.ClientConfig)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get OpenAPI schema: %v", err)
// 	}

// 	fmt.Println(models.ListModels())

// 	// Look up the model for the given kind
// 	modelName := fmt.Sprintf("%s.%s.%s", group, version, kind)
// 	println(modelName)
// 	println("io.k8s.api.core.v1.Pod")
// 	model := models.LookupModel(modelName)
// 	if model == nil {
// 		return "", fmt.Errorf("model for %s not found in the OpenAPI schema", modelName)
// 	}

// 	// Generate the example YAML
// 	exampleObj := map[string]interface{}{
// 		"apiVersion": fmt.Sprintf("%s/%s", group, version),
// 		"kind":       kind,
// 		"metadata": map[string]interface{}{
// 			"name": "example-name",
// 		},
// 	}

// 	if namespaced {
// 		exampleObj["metadata"].(map[string]interface{})["namespace"] = "default"
// 	}

// 	// Populate the example object based on the schema
// 	if err := populateExample(model, exampleObj); err != nil {
// 		return "", fmt.Errorf("failed to populate example: %v", err)
// 	}

// 	// Convert to YAML
// 	yamlBytes, err := yaml.Marshal(exampleObj)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to marshal YAML: %v", err)
// 	}

// 	return string(yamlBytes), nil
// }

// // Retrieves the OpenAPI schema models
// func getOpenAPISchema(cfg *rest.Config) (proto.Models, error) {
// 	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
// 	if err != nil {
// 		return nil, err
// 	}
// 	openAPISchema, err := disco.OpenAPISchema()
// 	if err != nil {
// 		return nil, err
// 	}
// 	models, err := proto.NewOpenAPIData(openAPISchema)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return models, nil
// }

// // Recursively populates the example object based on the schema
// func populateExample(model proto.Schema, parent map[string]interface{}) error {
// 	if model == nil {
// 		return nil
// 	}

// 	properties := map[string]interface{}{}

// 	switch s := model.(type) {
// 	case *proto.Kind:
// 		// Handle object properties if they exist
// 		for propName, propSchema := range s.Fields {
// 			var exampleValue interface{}
// 			if err := generateExampleValue(propSchema, &exampleValue); err != nil {
// 				return err
// 			}
// 			properties[propName] = exampleValue
// 		}
// 	default:
// 		// For other types, we can add specific handling as needed
// 	}

// 	if len(properties) > 0 {
// 		parent["spec"] = properties
// 	}

// 	return nil
// }

// // Generates example values based on the property schema
// func generateExampleValue(schema proto.Schema, value *interface{}) error {
// 	switch s := schema.(type) {
// 	case *proto.Primitive:
// 		*value = getPrimitiveExampleValue(s)
// 	case *proto.Array:
// 		var items []interface{}
// 		var item interface{}
// 		if err := generateExampleValue(s.SubType, &item); err != nil {
// 			return err
// 		}
// 		items = append(items, item)
// 		*value = items
// 	case *proto.Map:
// 		m := map[string]interface{}{
// 			"key": nil,
// 		}
// 		var keyValue interface{}
// 		if err := generateExampleValue(s.SubType, &keyValue); err != nil {
// 			return err
// 		}
// 		m["key"] = keyValue
// 		*value = m
// 	case *proto.Kind:
// 		obj := map[string]interface{}{}
// 		for propName, propSchema := range s.Fields {
// 			var propValue interface{}
// 			if err := generateExampleValue(propSchema, &propValue); err != nil {
// 				return err
// 			}
// 			obj[propName] = propValue
// 		}
// 		*value = obj
// 	default:
// 		*value = nil
// 	}
// 	return nil
// }

// // Returns example values for primitive types
// func getPrimitiveExampleValue(p *proto.Primitive) interface{} {
// 	switch p.Type {
// 	case "string":
// 		return "example-string"
// 	case "integer", "int32", "int64":
// 		return 0
// 	case "boolean":
// 		return false
// 	case "number", "float", "double":
// 		return 0.0
// 	default:
// 		return nil
// 	}
// }
