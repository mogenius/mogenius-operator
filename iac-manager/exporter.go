package iacmanager

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

	log "github.com/sirupsen/logrus"
)

// 1. Create git repository locally
// 2. create a folder for every incoming resource
// 3. Clean the workload from unnecessary fields, and metadata
// 4. store a file for every incoming workload
// 5. commit the changes
// 6. push the changes periodically

// Runtime tasks:
// 1. Check for incoming events

const (
	GIT_VAULT_FOLDER = "git-vault"
)

var ProcessedObjects = 0
var commitMutex sync.Mutex

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

	// namespaces := punqKubernetes.ListAllNamespace(nil)
	// for _, v := range namespaces {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// pods := punqKubernetes.AllPods("", nil)
	// for _, v := range pods {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// secrets := punqKubernetes.AllSecrets("", nil)
	// for _, v := range secrets {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// services := punqKubernetes.AllServices("", nil)
	// for _, v := range services {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// deployments := punqKubernetes.AllDeployments("", nil)
	// for _, v := range deployments {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// configmaps := punqKubernetes.AllConfigmaps("", nil)
	// for _, v := range configmaps {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// replicasets := punqKubernetes.AllReplicasets("", nil)
	// for _, v := range replicasets {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// daemonsets := punqKubernetes.AllDaemonsets("", nil)
	// for _, v := range daemonsets {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// ingresses := punqKubernetes.AllIngresses("", nil)
	// for _, v := range ingresses {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// certs := punqKubernetes.AllCertificates("", nil)
	// for _, v := range certs {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// crbs := punqKubernetes.AllClusterRoleBindings(nil)
	// for _, v := range crbs {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// cr := punqKubernetes.AllClusterRoles(nil)
	// for _, v := range cr {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// ci := punqKubernetes.AllClusterIssuers(nil)
	// for _, v := range ci {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// ingClass := punqKubernetes.AllIngressClasses(nil)
	// for _, v := range ingClass {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// clusterIssuers := punqKubernetes.AllClusterIssuers(nil)
	// for _, v := range clusterIssuers {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// cronJobs := punqKubernetes.AllCronjobs("", nil)
	// for _, v := range cronJobs {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// jobs := punqKubernetes.AllJobs("", nil)
	// for _, v := range jobs {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// netpol := punqKubernetes.AllNetworkPolicies("", nil)
	// for _, v := range netpol {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// pvs := punqKubernetes.AllPersistentVolumesRaw(nil)
	// for _, v := range pvs {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// pvcs := punqKubernetes.AllPersistentVolumeClaims("", nil)
	// for _, v := range pvcs {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// prioClass := punqKubernetes.AllPriorityClasses(nil)
	// for _, v := range prioClass {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// rq := punqKubernetes.AllResourceQuotas("", nil)
	// for _, v := range rq {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// roles := punqKubernetes.AllRoles("", nil)
	// for _, v := range roles {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// rb := punqKubernetes.AllRoleBindings("", nil)
	// for _, v := range rb {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// serviceAcc := punqKubernetes.AllServiceAccounts("", nil)
	// for _, v := range serviceAcc {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// statefullsets := punqKubernetes.AllStatefulSets("", nil)
	// for _, v := range statefullsets {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	// storageClasses := punqKubernetes.AllStorageClasses(nil)
	// for _, v := range storageClasses {
	// 	WriteResourceYaml(v.Kind, v.Namespace, v.Name, v)
	// }

	log.Infof("Initialized git repository with %d workloads. 💪", workloadCounter)
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

func WriteResourceYaml(kind string, namespace string, resourceName string, dataInf interface{}) {
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
	ProcessedObjects++
	log.Infof("Detected %s change. Updated %s/%s. 🧹", kind, namespace, resourceName)
	CommitChanges("", fmt.Sprintf("Updated [%s] %s/%s", kind, namespace, resourceName), []string{filename})
}

func DeleteResourceYaml(kind string, namespace string, resourceName string) error {
	filename := fileNameForRaw(kind, namespace, resourceName)
	err := os.Remove(filename)
	log.Infof("Detected %s deletion. Removed %s/%s. ♻️", kind, namespace, resourceName)
	CommitChanges("", fmt.Sprintf("Deleted [%s] %s/%s", kind, namespace, resourceName), []string{filename})

	return err
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

func CommitChanges(author string, message string, filePaths []string) error {
	commitMutex.Lock()
	defer commitMutex.Unlock()

	if author == "" {
		author = fmt.Sprintf("%s <%s>", utils.CONFIG.Git.GitUserName, utils.CONFIG.Git.GitUserEmail)
	}

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	for _, v := range filePaths {
		addCmd := fmt.Sprintf("cd %s; git add %s", folder, v)
		err := utils.ExecuteShellCommandRealySilent(addCmd, addCmd)
		if err != nil {
			log.Errorf("Error adding files to git repository: %s", err.Error())
			return err
		}
	}

	commitCmd := fmt.Sprintf("cd %s; git commit -m '%s' --author '%s'", folder, message, author)
	err := utils.ExecuteShellCommandRealySilent(commitCmd, commitCmd)
	if err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") &&
			!strings.Contains(err.Error(), "nothing added to commit") &&
			!strings.Contains(err.Error(), "no changes added to commit") {
			log.Errorf("Error committing files to git repository: %s", err.Error())
			return err
		}
	}

	// TODO: if repo is known we can push the changes
	// pushCmd := fmt.Sprintf("cd %s; git commit -m \"%s\" --author=\"%s\"", folder, message, author)
	// err = utils.ExecuteShellCommandSilent(commitCmd, commitCmd)
	// if err != nil {
	// 	log.Errorf("Error committing files to git repository: %s", err.Error())
	// 	return err
	// }
	return nil
}

func DebounceFunc(interval time.Duration, function func()) func() {
	var timer *time.Timer
	var mu sync.Mutex

	return func() {
		mu.Lock()
		defer mu.Unlock()

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(interval, function)
	}
}
