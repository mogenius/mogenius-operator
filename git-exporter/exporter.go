package gitexporter

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	punq "github.com/mogenius/punq/structs"
	log "github.com/sirupsen/logrus"
	v1Core "k8s.io/api/core/v1"
)

// 1. Create git repository locally
// 2. create a folder for every incoming resource
// 3. Clean the workload from unnecessary fields, and metadata
// 4. store a file for every incoming workload
// 5. commit the changes
// 6. push the changes periodically

const (
	GIT_VAULT_FOLDER = "git-vault"
)

func Init() {
	// Create a git repository
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		err := os.MkdirAll(folder, 0755)
		if err != nil {
			log.Errorf("Error creating folder for git repository (in %s): %s", folder, err.Error())
		}
	}

	err := utils.ExecuteShellCommandSilent("git init", fmt.Sprintf("cd %s; git init", folder))
	if err != nil {
		log.Errorf("Error creating git repository: %s", err.Error())
	}
}

func CheckEvent(event *v1Core.Event) {
	switch event.Type {
	case "Normal":
		if event.Reason == "Created" || event.Reason == "ScalingReplicaSet" {
			createFolderForResource(event.InvolvedObject.Kind)
			saveResourceYaml(event)
		}
		if event.Reason == "Updated" {
			saveResourceYaml(event)
		}
		if event.Reason == "Deleted" || event.Reason == "Killing" {
			deleteResourceYaml(event)
		}
		log.Infof("Normal event received: %s%s: %s", event.ObjectMeta.Namespace, event.ReportingController, event.Message)

	case "Warning":
		log.Warnf("Warning event received: %s%s: %s", event.ObjectMeta.Namespace, event.ReportingController, event.Message)
	}
}

func getResourceYaml(event *v1Core.Event) string {
	data := punq.ExecuteShellCommandWithResponse("Get resource yaml", fmt.Sprintf("kubectl get %s -n %s %s -o yaml", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name))
	var dataMap map[string]interface{}
	err := yaml.Unmarshal([]byte(data), &dataMap)
	if err != nil {
		log.Errorf("Error unmarshalling yaml: %s", err.Error())
	}

	removeFieldAtPath(dataMap, "uid", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "selfLink", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "generation", []string{"metadata"}, []string{})
	removeFieldAtPath(dataMap, "managedFields", []string{"metadata"}, []string{})

	removeFieldAtPath(dataMap, "creationTimestamp", []string{}, []string{})
	removeFieldAtPath(dataMap, "resourceVersion", []string{}, []string{})
	removeFieldAtPath(dataMap, "status", []string{}, []string{})

	cleanedYaml, err := yaml.Marshal(dataMap)
	if err != nil {
		log.Errorf("Error marshalling yaml: %s", err.Error())
	}
	return string(cleanedYaml)
}

func saveResourceYaml(event *v1Core.Event) error {
	data := getResourceYaml(event)
	err := os.WriteFile(fileNameForResource(event), []byte(data), 0755)
	return err
}

func deleteResourceYaml(event *v1Core.Event) error {
	err := os.Remove(fileNameForResource(event))
	return err
}

func fileNameForResource(event *v1Core.Event) string {
	name := fmt.Sprintf("%s/%s/%s/%s-%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
	return name
}

// Create a folder for every incoming resource
func createFolderForResource(resource string) error {
	basePath := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	resourceFolder := fmt.Sprintf("%s/%s", basePath, resource)

	if _, err := os.Stat(resourceFolder); os.IsNotExist(err) {
		err := os.Mkdir(resourceFolder, 0755)
		if err != nil {
			log.Errorf("Error creating folder for resource: %s", err.Error())
			return err
		}
	}
	return nil
}

// removeFieldAtPath recursively searches through the data structure.
// If the current path matches the target path, it removes the specified field.
func removeFieldAtPath(data map[string]interface{}, field string, targetPath []string, currentPath []string) {
	// Check if the current path matches the target path for removal.
	if len(currentPath) >= len(targetPath) && strings.Join(currentPath[len(currentPath)-len(targetPath):], "/") == strings.Join(targetPath, "/") {
		delete(data, field)
	}
	// Continue searching within the map.
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			removeFieldAtPath(v, field, targetPath, append(currentPath, key))
		case []interface{}:
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					// Construct a new path for each item in the list.
					newPath := append(currentPath, fmt.Sprintf("%s[%d]", key, i))
					removeFieldAtPath(itemMap, field, targetPath, newPath)
				}
			}
		}
	}
}
