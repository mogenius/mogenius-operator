package iacmanager

import (
	"bytes"
	"fmt"
	"mogenius-k8s-manager/utils"
	"os"
	"os/exec"
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
// git remote add origin https://github.com/beneiltis/iactest.git
// git branch -M main
// git push -u origin main

const (
	GIT_VAULT_FOLDER = "git-vault"
)

var ProcessedObjects = 0
var commitMutex sync.Mutex

var changedFiles []string

var syncInProcess = false

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

	// Set up the remote repository
	if !gitHasRemotes() {
		SetupRemote()
	}

	// START SYNCING CHANGES
	SyncChangesTimer()
}

func SyncChangesTimer() {
	ticker := time.NewTicker(time.Duration(utils.CONFIG.Iac.PollingIntervalSecs) * time.Second)
	quit := make(chan struct{}) // Create a channel to signal the ticker to stop

	go func() {
		for {
			select {
			case <-ticker.C:
				SyncChanges()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func SyncChanges() {
	if gitHasRemotes() {
		if !syncInProcess {
			syncInProcess = true
			updatedFiles, deletedFiles := pullChanges()
			for _, v := range updatedFiles {
				kubernetesApplyResource(v)
			}
			for _, v := range deletedFiles {
				kubernetesDeleteResource(v)
			}
			pushChanges()
			syncInProcess = false
		}
	}
}

func SetupRemote() error {
	if utils.CONFIG.Iac.RepoUrl == "" {
		return fmt.Errorf("Repository URL is empty. Please set the repository URL in the configuration file or as env var.")
	}
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	remoteCmdStr := fmt.Sprintf("cd %s; git remote add origin %s", folder, utils.CONFIG.Iac.RepoUrl)
	err := utils.ExecuteShellCommandSilent(remoteCmdStr, remoteCmdStr)
	if err != nil {
		log.Errorf("Error setting up remote: %s", err.Error())
		return err
	}
	return nil
}

func CheckRepoAccess() bool {
	if utils.CONFIG.Iac.RepoUrl == "" {
		log.Warn("Repository URL is empty. Please set the repository URL in the configuration file or as env var.")
		return false
	}
	if utils.CONFIG.Iac.RepoPat == "" {
		log.Warn("Repository PAT is empty. Please set the repository PAT in the configuration file or as env var.")
	}
	// Insert the PAT into the repository URL
	repoURLWithPAT := insertPATIntoURL(utils.CONFIG.Iac.RepoUrl, utils.CONFIG.Iac.RepoPat)

	// Prepare the `git ls-remote` command
	cmd := exec.Command("git", "ls-remote", repoURLWithPAT)

	// Execute the command
	if err := cmd.Run(); err != nil {
		// If there's an error, access is likely not available
		return false
	}

	// If the command succeeds, access is available
	return true
}

// insertPATIntoURL inserts the PAT into the Git repository URL for authentication
func insertPATIntoURL(gitRepoURL, pat string) string {
	if pat == "" {
		return gitRepoURL
	}
	if !strings.HasPrefix(gitRepoURL, "https://") {
		return gitRepoURL // Non-HTTPS URLs are not handled here
	}
	return strings.Replace(gitRepoURL, "https://", "https://"+pat+"@", 1)
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
		log.Errorf("Error marshaling to YAML: %s\n", err.Error())
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
	name := fmt.Sprintf("%s/%s/%s/%s_%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, namespace, resourceName)
	if namespace == "" {
		name = fmt.Sprintf("%s/%s/%s/%s.yaml", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER, kind, resourceName)
	}
	return name
}

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

func gitHasRemotes() bool {
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = folder
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Errorf("Error checking git remotes: %s %s %s", err.Error(), out.String(), stderr.String())
		return false
	}
	if len(out.String()) > 0 {
		return true
	}
	return false
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
		changedFiles = append(changedFiles, v)
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
	return nil
}

func pullChanges() (updatedFiles []string, deletedFiles []string) {
	if !utils.CONFIG.Iac.AllowPull {
		return
	}
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	// Pull changes from the remote repository
	cmd := exec.Command("git", "pull", "origin", "main")
	cmd.Env = append(os.Environ(), "GIT_ASKPASS=echo", fmt.Sprintf("GIT_PASSWORD=%s", utils.CONFIG.Iac.RepoPat)) // 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
	cmd.Dir = folder
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if out.String() == "Already up to date.\n" {
		log.Infof("Pulled changes from the remote repository (Modified: %d / Deleted: %d). 🔄🔄🔄", len(updatedFiles), len(deletedFiles))
		return updatedFiles, deletedFiles
	}
	if err != nil {
		if !strings.Contains(stderr.String(), "Your local changes to the following files would be overwritten by merge") {
			log.Errorf("Error running git diff: %s %s %s", err.Error(), out.String(), stderr.String())
		}
		return updatedFiles, deletedFiles
	}

	// Wait for the changes to be pulled
	time.Sleep(1 * time.Second)

	//Get the list of updated or newly added files since the last pull
	updatedFiles, err = getGitFiles(folder, "HEAD@{1}", "HEAD", "--name-only", "--diff-filter=AM")
	if err != nil {
		log.Errorf("Error getting added/updated files: %s", err.Error())
		return updatedFiles, deletedFiles
	}

	// Get the list of deleted files since the last pull
	deletedFiles, err = getGitFiles(folder, "HEAD@{1}", "HEAD", "--name-only", "--diff-filter=D")
	if err != nil {
		log.Errorf("Error getting deleted files: %s", err.Error())
		return
	}

	log.Infof("Pulled changes from the remote repository (Modified: %d / Deleted: %d). 🔄🔄🔄", len(updatedFiles), len(deletedFiles))
	if utils.CONFIG.Misc.Debug {
		log.Infof("Added/Updated Files (%d):", len(updatedFiles))
		for _, file := range updatedFiles {
			log.Info(file)
		}

		log.Infof("Deleted Files (%d):", len(deletedFiles))
		for _, file := range deletedFiles {
			log.Info(file)
		}
	}
	return updatedFiles, deletedFiles
}

func pushChanges() {
	if !utils.CONFIG.Iac.AllowPush {
		return
	}
	if len(changedFiles) <= 0 {
		return
	}

	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)

	// Push changes to the remote repository
	cmd := exec.Command("git", "push", "origin", "main")
	//cmd.Env = append(os.Environ(), "GIT_ASKPASS=echo", "GIT_PASSWORD=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")
	cmd.Dir = folder
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stderr.String() != "Everything up-to-date\n" {
		log.Infof("Pushed %d changes to remote repository. 🔄🔄🔄", len(changedFiles))
		if utils.CONFIG.Misc.Debug {
			for _, file := range changedFiles {
				log.Info(file)
			}
		}
	}
	if err != nil {
		log.Errorf("Error running git push: %s %s %s", err.Error(), out.String(), stderr.String())
	}
	changedFiles = []string{}
}

func getGitFiles(workDir string, ref string, options ...string) ([]string, error) {
	args := []string{"diff", ref}
	args = append(args, options...)
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Error running git diff: %s %s %s", err.Error(), out.String(), stderr.String())
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

func kubernetesDeleteResource(file string) {
	parts := strings.Split(file, "/")
	filename := strings.Replace(parts[len(parts)-1], ".yaml", "", -1)
	partsName := strings.Split(filename, "_")
	kind := parts[0]
	namespace := ""
	name := ""
	if len(partsName) > 1 {
		namespace = fmt.Sprintf("--namespace=%s ", partsName[0])
		name = partsName[1]
	}

	if kind == "" {
		log.Errorf("Kind cannot be empty. File: %s", file)
		return
	}
	if name == "" {
		log.Errorf("Name cannot be empty. File: %s", file)
		return
	}

	delCmd := fmt.Sprintf("kubectl delete %s %s%s", kind, namespace, name)
	err := utils.ExecuteShellCommandRealySilent(delCmd, delCmd)
	if err != nil {
		log.Errorf("Error deleting resource: %s", err.Error())
	} else {
		log.Infof("✅ Deleted resource %s%s%s", kind, namespace, name)
	}
}

func kubernetesApplyResource(file string) {
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Misc.DefaultMountPath, GIT_VAULT_FOLDER)
	delCmd := fmt.Sprintf("kubectl apply -f %s/%s", folder, file)
	err := utils.ExecuteShellCommandRealySilent(delCmd, delCmd)
	if err != nil {
		log.Errorf("Error applying file %s: %s", file, err.Error())
	} else {
		log.Infof("✅ Applied file: %s", file)
	}
}
