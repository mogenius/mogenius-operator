package reconciler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// githubAppDefaultPrivateKeyKey is the key the private key is read from when
// the PlatformConfig names none.
const githubAppDefaultPrivateKeyKey = "privateKey"

// githubAppConflictingReadKeys are credentials of other kinds that both engines
// prefer over the app keys when present.
var githubAppConflictingReadKeys = []string{"username", "password", "bearerToken", "sshPrivateKey", "identity"}

// githubAppCredentials materializes and prunes the engine credentials of
// repositories that authenticate as a GitHub App. The engine turns them into
// tokens itself; the platform asks the operator to mint one per commit
// (MintGitHubAppToken in core), so no token is ever stored.
//
// The credential is written directly rather than shipped as an extra
// object of the engine's release, so the private key never ends up in Helm
// values or an Application manifest.
type githubAppCredentials struct {
	client            k8s.Interface
	dynamic           dynamic.Interface
	logger            *slog.Logger
	operatorNamespace string
	engine            string
	engineNamespace   string
	vaults            []v1alpha1.ExternalSecretVault
	// applyObject writes an ExternalSecret under the platform's ownership guard.
	applyObject func(ctx context.Context, resource utils.ResourceDescriptor, namespace string, object map[string]any) error
}

func (d *reconcilerModule) reconcileGitHubAppCredentials(
	ctx context.Context,
	spec v1alpha1.PlatformConfigSpec,
	engine string,
	engineNamespace string,
) *ReconcileResult {
	credentials := githubAppCredentials{
		client:            d.clientProvider.K8sClientSet(),
		dynamic:           d.clientProvider.DynamicClient(),
		logger:            d.logger,
		operatorNamespace: d.config.Get("MO_OWN_NAMESPACE"),
		engine:            engine,
		engineNamespace:   engineNamespace,
		applyObject:       d.applyPlatformObject,
	}
	if spec.ExternalSecretsOperator != nil {
		credentials.vaults = spec.ExternalSecretsOperator.Vaults
	}
	if err := credentials.reconcile(ctx, spec.GitOps); err != nil {
		return &ReconcileResult{Err: err}
	}
	return nil
}

// githubAppRepository is a repository entry that declares a GitHub App, under
// the name its credentials are derived from.
type githubAppRepository struct {
	name string
	repo v1alpha1.GitOpsRepositoryConfig
}

func (r githubAppRepository) readSecretName() string {
	return r.name + repositorySecretSuffix
}

func githubAppRepositories(gitOps *v1alpha1.GitOpsConfig) []githubAppRepository {
	if gitOps == nil {
		return nil
	}
	var repositories []githubAppRepository
	for _, repo := range gitOps.Repositories {
		if repo.GitHubApp == nil {
			continue
		}
		repositories = append(repositories, githubAppRepository{name: gitOpsRepositoryName(repo), repo: repo})
	}
	return repositories
}

func (c githubAppCredentials) reconcile(ctx context.Context, gitOps *v1alpha1.GitOpsConfig) error {
	repositories := githubAppRepositories(gitOps)

	// Pruning goes first: an ExternalSecret left over from a switch to a plain
	// Secret has to be gone before the Secret it targeted is written over.
	errs := []error{c.prune(ctx, repositories)}

	for _, repository := range repositories {
		if err := c.materializeReadCredential(ctx, repository); err != nil {
			errs = append(errs, fmt.Errorf("repository %q: %w", repository.name, err))
		}
	}
	return errors.Join(errs...)
}

// prune deletes every labelled credential no repository declares anymore:
// githubApp removed, repository dropped, private key moved to another source
// or engine moved to another namespace.
func (c githubAppCredentials) prune(ctx context.Context, repositories []githubAppRepository) error {
	wantSecrets := map[string]bool{}
	wantExternalSecrets := map[string]bool{}
	for _, repository := range repositories {
		wantSecrets[c.engineNamespace+"/"+repository.readSecretName()] = true
		if repository.repo.GitHubApp.PrivateKey.ExternalSecret != nil {
			wantExternalSecrets[c.engineNamespace+"/"+repository.readSecretName()] = true
		}
	}

	selector := metav1.ListOptions{LabelSelector: gitops.GitHubAppLabelKey}
	var errs []error

	// ExternalSecrets before Secrets, orphaning their target: deleting with
	// the default propagation would garbage-collect a Secret that is about to
	// be written from another source.
	externalSecrets := c.dynamic.Resource(kubernetes.CreateGroupVersionResource(utils.ExternalSecretResource.ApiVersion, utils.ExternalSecretResource.Plural))
	esList, err := externalSecrets.Namespace(metav1.NamespaceAll).List(ctx, selector)
	switch {
	case apierrors.IsNotFound(err):
		// External Secrets Operator not installed: nothing of ours to prune.
	case err != nil:
		errs = append(errs, fmt.Errorf("list github app external secrets: %w", err))
	default:
		orphan := metav1.DeletePropagationOrphan
		for _, item := range esList.Items {
			if wantExternalSecrets[item.GetNamespace()+"/"+item.GetName()] {
				continue
			}
			err := externalSecrets.Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{PropagationPolicy: &orphan})
			if err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("delete external secret %s/%s: %w", item.GetNamespace(), item.GetName(), err))
				continue
			}
			c.logger.Info("pruned github app external secret", "namespace", item.GetNamespace(), "name", item.GetName())
		}
	}

	secrets, err := c.client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, selector)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("list github app secrets: %w", err))...)
	}
	for _, secret := range secrets.Items {
		key := secret.Namespace + "/" + secret.Name
		// An ExternalSecret target stays while its ExternalSecret is wanted;
		// ESO owns it.
		if wantSecrets[key] || wantExternalSecrets[key] {
			continue
		}
		err := c.client.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete secret %s: %w", key, err))
			continue
		}
		c.logger.Info("pruned github app secret", "namespace", secret.Namespace, "name", secret.Name)
	}
	return errors.Join(errs...)
}

func (c githubAppCredentials) materializeReadCredential(ctx context.Context, repository githubAppRepository) error {
	labels, data := githubAppReadCredential(repository.repo, c.engine)
	privateKey := repository.repo.GitHubApp.PrivateKey

	switch {
	case privateKey.Secret != nil:
		key := privateKey.Secret.Key
		if key == "" {
			key = githubAppDefaultPrivateKeyKey
		}
		source, err := c.client.CoreV1().Secrets(c.operatorNamespace).Get(ctx, privateKey.Secret.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("read github app private key secret %s/%s: %w", c.operatorNamespace, privateKey.Secret.Name, err)
		}
		pem, ok := source.Data[key]
		if !ok || len(pem) == 0 {
			return fmt.Errorf("github app private key secret %s/%s has no key %q", c.operatorNamespace, privateKey.Secret.Name, key)
		}

		desired := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: repository.readSecretName(), Namespace: c.engineNamespace, Labels: labels},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{gitops.GitHubAppPrivateKeyKey: pem},
		}
		for k, v := range data {
			desired.Data[k] = []byte(v)
		}
		return c.writeReadCredential(ctx, desired)

	case privateKey.ExternalSecret != nil:
		es := *privateKey.ExternalSecret
		if es.Vault == "" {
			if len(c.vaults) == 0 {
				return errors.New("provide githubApp.privateKey.externalSecret.vault or define a vault in spec.externalSecretsOperator")
			}
			es.Vault = c.vaults[0].Name
		}
		if es.Key == "" {
			es.Key = githubAppDefaultPrivateKeyKey
		}
		data[gitops.GitHubAppPrivateKeyKey] = fmt.Sprintf("{{ .%s }}", es.Key)
		object := externalSecretResource(repository.readSecretName(), c.engineNamespace, es, labels, data)
		// The ExternalSecret itself carries the label too, so prune finds it.
		object["metadata"].(map[string]any)["labels"] = map[string]any{gitops.GitHubAppLabelKey: gitops.GitHubAppLabelValue}
		return c.applyObject(ctx, utils.ExternalSecretResource, c.engineNamespace, object)

	default:
		return errors.New("githubApp.privateKey needs either secret or externalSecret")
	}
}

// githubAppReadCredential is the engine's credential without the private key:
// its labels and its plain data keys.
func githubAppReadCredential(repo v1alpha1.GitOpsRepositoryConfig, engine string) (map[string]string, map[string]string) {
	app := repo.GitHubApp
	labels := map[string]string{
		gitops.ManagedByLabelKey:    gitops.ManagedByLabelValue,
		gitops.ComponentK8sLabelKey: gitops.ComponentK8sValue,
		gitops.PartOfLabelKey:       gitops.PartOfLabelValue,
		gitops.GitHubAppLabelKey:    gitops.GitHubAppLabelValue,
	}
	data := map[string]string{
		gitops.GitHubAppIDKey:             strconv.FormatInt(app.AppID, 10),
		gitops.GitHubAppInstallationIDKey: strconv.FormatInt(app.InstallationID, 10),
	}

	if engine == gitOpsEngineArgoCD {
		labels[gitops.ArgoCDSecretTypeLabelKey] = gitops.ArgoCDSecretTypeLabelValue
		data["type"] = "git"
		data["url"] = repo.URL
		if app.EnterpriseBaseURL != "" {
			data[gitops.ArgoCDGitHubAppEnterpriseBaseURLKey] = app.EnterpriseBaseURL
		}
		return labels, data
	}

	if app.EnterpriseBaseURL != "" {
		data[gitops.FluxGitHubAppBaseURLKey] = app.EnterpriseBaseURL
	}
	return labels, data
}

// readCredentialAdoptable reports whether an existing Secret under the read
// credential's name may be written: one the platform wrote, through this
// operator, the bootstrap chart or a token rotation. Anything else belongs to
// someone who did not hand it over.
func readCredentialAdoptable(secret *corev1.Secret) bool {
	if _, ok := secret.Labels[gitops.GitHubAppLabelKey]; ok {
		return true
	}
	switch secret.Labels[gitops.ManagedByLabelKey] {
	case gitops.ManagedByLabelValue, "mogenius-operator":
		return true
	}
	return secret.Labels[gitops.WriteManagedByLabelKey] == gitops.WriteManagedByLabelValue
}

// writeReadCredential merges desired into the read credential and drops the
// credential keys of other kinds, which both engines prefer over the app's.
// Unrelated keys someone added, such as a TLS client certificate or a proxy,
// survive.
func (c githubAppCredentials) writeReadCredential(ctx context.Context, desired *corev1.Secret) error {
	secrets := c.client.CoreV1().Secrets(desired.Namespace)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := secrets.Get(ctx, desired.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = secrets.Create(ctx, desired, metav1.CreateOptions{})
			return err
		}
		if err != nil {
			return err
		}
		if !readCredentialAdoptable(current) {
			return fmt.Errorf("secret %s/%s exists and was not created by mogenius; label it %s=%s to hand it over",
				desired.Namespace, desired.Name, gitops.GitHubAppLabelKey, gitops.GitHubAppLabelValue)
		}

		if current.Labels == nil {
			current.Labels = map[string]string{}
		}
		for key, value := range desired.Labels {
			current.Labels[key] = value
		}
		if current.Data == nil {
			current.Data = map[string][]byte{}
		}
		for _, key := range githubAppConflictingReadKeys {
			delete(current.Data, key)
		}
		for key, value := range desired.Data {
			current.Data[key] = value
		}
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
}
