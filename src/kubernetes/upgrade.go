package kubernetes

import (
	"context"
	"fmt"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/websocket"
	"strings"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// restart deployments/daemonsets for operator
func ClusterForceReconnect() bool {
	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(config.Get("MO_OWN_NAMESPACE"))

	podsToKill := []string{}
	podsToKill = append(podsToKill, AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", GetOwnDeploymentName(config))...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod ...", "podName", podName)
		err := podClient.Delete(context.Background(), podName, metav1.DeleteOptions{})
		if err != nil {
			k8sLogger.Error("failed to delete pod", "podName", podName, "error", err)
		}
	}

	return true
}

// restart deployments/daemonsets for operator
func ClusterForceDisconnect() bool {
	clientset := clientProvider.K8sClientSet()
	namespace := config.Get("MO_OWN_NAMESPACE")
	podClient := clientset.CoreV1().Pods(namespace)

	// stop operator
	deployment := store.GetDeployment(namespace, GetOwnDeploymentName(config))
	if deployment == nil {
		k8sLogger.Error("ClusterForceDisconnect: own deployment not found in store")
		return false
	}
	deploymentClient := clientset.AppsV1().Deployments(namespace)
	deployment.Spec.Replicas = new(int32(0))
	_, err := deploymentClient.Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		k8sLogger.Error("Error updating deployment", "deployment", deployment, "error", err)
	}

	podsToKill := []string{}
	podsToKill = append(podsToKill, AllPodNamesForLabel(config.Get("MO_OWN_NAMESPACE"), "app", GetOwnDeploymentName(config))...)

	for _, podName := range podsToKill {
		k8sLogger.Warn("Restarting pod...", "pod", podName)
		err := podClient.Delete(context.Background(), podName, metav1.DeleteOptions{})

		if err != nil {
			k8sLogger.Error("failed to delete pod", "pod", podName, "error", err)
		}
	}

	return true
}

const (
	// OPERATOR_CHART_REF is where the operator is released. The upgrade always
	// installs from here; the platform only picks the version.
	OPERATOR_CHART_REF = "oci://ghcr.io/mogenius/helm-charts/mogenius-operator"
	// OPERATOR_RELEASE_NAME is the Helm release the install command creates.
	OPERATOR_RELEASE_NAME = "mogenius-operator"
	// legacyUpgradeConfigMapName held the shell script of older upgrades
	// (including the cluster API key); it is removed on the next upgrade.
	legacyUpgradeConfigMapName = "updater-cm"
)

// UpgradeArgs builds the helm arguments for upgrading the operator from
// currentVersion to targetVersion. The chart source is fixed and the values
// of the running release are kept (--reset-then-reuse-values: new chart
// defaults plus the values the release was installed with), so the caller
// only decides the version:
//   - empty targetVersion: the latest release, as the upgrade has always done
//   - otherwise a valid version that is not older than the running one, and a
//     pre-release only when the operator itself runs a pre-release
//
// A pre-release operator (develop build) upgrades from the -dev chart with
// --devel, like the platform's dev stage did.
func UpgradeArgs(currentVersion string, targetVersion string, namespace string) ([]string, error) {
	current, currentErr := semver.NewVersion(currentVersion)
	devChannel := currentErr == nil && current.Prerelease() != ""

	chart := OPERATOR_CHART_REF
	if devChannel {
		chart += "-dev"
	}
	args := []string{"upgrade", "--install", OPERATOR_RELEASE_NAME, chart, "-n", namespace, "--reset-then-reuse-values"}
	if devChannel {
		args = append(args, "--devel")
	}
	if targetVersion == "" {
		return args, nil
	}

	target, err := semver.StrictNewVersion(strings.TrimPrefix(targetVersion, "v"))
	if err != nil {
		return nil, fmt.Errorf("invalid target version %q: %w", targetVersion, err)
	}
	if target.Prerelease() != "" && !devChannel {
		return nil, fmt.Errorf("refusing pre-release %q for a release operator", targetVersion)
	}
	if currentErr != nil {
		return nil, fmt.Errorf("running version %q is unknown, cannot check %q against it", currentVersion, targetVersion)
	}
	// build metadata (+dev of local builds) does not order versions
	currentCore, _ := current.SetMetadata("")
	if target.LessThan(&currentCore) {
		return nil, fmt.Errorf("refusing downgrade from %s to %s", currentVersion, targetVersion)
	}
	return append(args, "--version", target.String()), nil
}

// UpgradeMyself runs helm with args (see UpgradeArgs) in a job under the
// operator's service account. The arguments go to helm directly, no shell.
func UpgradeMyself(eventClient websocket.WebsocketClient, job *structs.Job, args []string) (*structs.Job, error) {
	cmd := structs.CreateCommand(eventClient, "upgrade operator", "Upgrade mogenius platform ...", job)
	cmd.Start(eventClient, job, "Upgrade mogenius platform ...")

	clientset := clientProvider.K8sClientSet()
	jobClient := clientset.BatchV1().Jobs(config.Get("MO_OWN_NAMESPACE"))

	ownerReference, err := GetOwnDeploymentOwnerReference(clientset, config)
	if err != nil {
		k8sLogger.Error("Error getting owner reference for upgrade job", "error", err)
		return nil, err
	}

	err = clientset.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE")).Delete(context.Background(), legacyUpgradeConfigMapName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		k8sLogger.Warn("failed to remove the configmap of an earlier upgrade", "configmap", legacyUpgradeConfigMapName, "error", err)
	}

	k8sjob := utils.InitUpgradeJob()
	k8sjob.Namespace = config.Get("MO_OWN_NAMESPACE")
	k8sjob.Name = fmt.Sprintf("%s-%s", k8sjob.Name, utils.NanoIdSmallLowerCase())
	k8sjob.OwnerReferences = ownerReference
	k8sjob.Spec.Template.Spec.Containers[0].Command = []string{"helm"}
	k8sjob.Spec.Template.Spec.Containers[0].Args = args

	// JOB
	_, err = jobClient.Get(context.Background(), k8sjob.Name, metav1.GetOptions{})
	if err != nil {
		// CREATE
		_, err = jobClient.Create(context.Background(), &k8sjob, MoCreateOptions(config))
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (job) ERROR: %s", err.Error()))
			return nil, err
		}
	} else {
		// UPDATE
		_, err = jobClient.Update(context.Background(), &k8sjob, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpgradeMyself (update_job) ERROR: %s", err.Error()))
			return nil, err
		}
	}
	cmd.Success(eventClient, job, "Upgraded platform successfully.")
	return job, nil
}

func GetOwnDeploymentOwnerReference(clientset *kubernetes.Clientset, config cfg.ConfigModule) ([]metav1.OwnerReference, error) {
	ownDeploymentName := config.Get("OWN_DEPLOYMENT_NAME")
	assert.Assert(ownDeploymentName != "")

	namespace := config.Get("MO_OWN_NAMESPACE")
	assert.Assert("MO_OWN_NAMESPACE" != "")

	ownDeployment := store.GetDeployment(namespace, ownDeploymentName) // to cache it
	if ownDeployment == nil {
		return nil, fmt.Errorf("failed to find own deployment %s in namespace %s", ownDeploymentName, namespace)
	}
	reference := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       ownDeployment.GetName(),
			UID:        ownDeployment.GetUID(),
			Controller: new(true),
		},
	}

	return reference, nil
}
