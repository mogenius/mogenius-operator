package gitexporter

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	punqKubernetes "github.com/mogenius/punq/kubernetes"
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
	initAll()
}

func initAll() {
	workloadCounter := 0

	namespaces := punqKubernetes.ListAllNamespace(nil)
	for _, v := range namespaces {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	pods := punqKubernetes.AllPods("", nil)
	for _, v := range pods {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	secrets := punqKubernetes.AllSecrets("", nil)
	for _, v := range secrets {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	services := punqKubernetes.AllServices("", nil)
	for _, v := range services {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	deployments := punqKubernetes.AllDeployments("", nil)
	for _, v := range deployments {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	configmaps := punqKubernetes.AllConfigmaps("", nil)
	for _, v := range configmaps {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	replicasets := punqKubernetes.AllReplicasets("", nil)
	for _, v := range replicasets {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	daemonsets := punqKubernetes.AllDaemonsets("", nil)
	for _, v := range daemonsets {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	ingresses := punqKubernetes.AllIngresses("", nil)
	for _, v := range ingresses {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	certs := punqKubernetes.AllCertificates("", nil)
	for _, v := range certs {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	crbs := punqKubernetes.AllClusterRoleBindings(nil)
	for _, v := range crbs {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	cr := punqKubernetes.AllClusterRoles(nil)
	for _, v := range cr {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	ci := punqKubernetes.AllClusterIssuers(nil)
	for _, v := range ci {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	ingClass := punqKubernetes.AllIngressClasses(nil)
	for _, v := range ingClass {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	clusterIssuers := punqKubernetes.AllClusterIssuers(nil)
	for _, v := range clusterIssuers {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	cronJobs := punqKubernetes.AllCronjobs("", nil)
	for _, v := range cronJobs {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	jobs := punqKubernetes.AllJobs("", nil)
	for _, v := range jobs {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	netpol := punqKubernetes.AllNetworkPolicies("", nil)
	for _, v := range netpol {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	pvs := punqKubernetes.AllPersistentVolumesRaw(nil)
	for _, v := range pvs {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	pvcs := punqKubernetes.AllPersistentVolumeClaims("", nil)
	for _, v := range pvcs {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	prioClass := punqKubernetes.AllPriorityClasses(nil)
	for _, v := range prioClass {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	rq := punqKubernetes.AllResourceQuotas("", nil)
	for _, v := range rq {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	roles := punqKubernetes.AllRoles("", nil)
	for _, v := range roles {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	rb := punqKubernetes.AllRoleBindings("", nil)
	for _, v := range rb {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	serviceAcc := punqKubernetes.AllServiceAccounts("", nil)
	for _, v := range serviceAcc {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	statefullsets := punqKubernetes.AllStatefulSets("", nil)
	for _, v := range statefullsets {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	storageClasses := punqKubernetes.AllStorageClasses(nil)
	for _, v := range storageClasses {
		writeResourceYaml(v.Kind, v.Namespace, v.Name, v, &workloadCounter)
	}

	log.Infof("Initialized git repository with %d workloads. 💪", workloadCounter)
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

func getResourceYamlRaw(kind string, namespace string, resourceName string) string {
	data := punq.ExecuteShellCommandWithResponse("Get resource yaml", fmt.Sprintf("kubectl get %s -n %s %s -o yaml", kind, namespace, resourceName))
	cleanedData := cleanYaml(data)
	return cleanedData
}

func getResourceYaml(event *v1Core.Event) string {
	data := punq.ExecuteShellCommandWithResponse("Get resource yaml", fmt.Sprintf("kubectl get %s -n %s %s -o yaml", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name))
	cleanedData := cleanYaml(data)
	return cleanedData
}

func cleanYaml(data string) string {
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

func writeResourceYaml(kind string, namespace string, resourceName string, dataInf interface{}, workloadCounter *int) {
	if kind == "" {
		log.Errorf("Kind is empty for resource %s:%s/%s", kind, namespace, resourceName)
		return
	}
	yamlData, err := yaml.Marshal(dataInf)
	if err != nil {
		fmt.Printf("Error marshaling to YAML: %v\n", err)
		return
	}
	createFolderForResource(kind)
	data := cleanYaml(string(yamlData))
	filename := fileNameForRaw(kind, namespace, resourceName)
	err = os.WriteFile(filename, []byte(data), 0755)
	if err != nil {
		log.Errorf("Error writing resource %s:%s/%s file: %s", kind, namespace, resourceName, err.Error())
		return
	}
	*workloadCounter++
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

func fileNameForRaw(kind string, namespace string, resourceName string) string {
	name := fmt.Sprintf("%s/%s/%s/%s-%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, namespace, resourceName)
	if namespace == "" {
		name = fmt.Sprintf("%s/%s/%s/%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, resourceName)
	}
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
