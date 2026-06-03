package helm

import (
	"fmt"
	"strings"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	releaser "helm.sh/helm/v4/pkg/release"
	release "helm.sh/helm/v4/pkg/release/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	// Helm stores ownership metadata as annotations on every managed object.
	// See https://github.com/helm/helm/blob/v4.1.4/pkg/action/validate.go
	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
)

// AdoptionCandidate is an existing cluster resource that will be pulled into
// the target Helm release because it carries no Helm ownership metadata.
// These are safe to adopt — they are the exact symptom of a previously
// incomplete `helm uninstall`.
type AdoptionCandidate struct {
	Kind      string
	Namespace string
	Name      string
}

// OwnershipConflict is an existing cluster resource that is already owned by
// a different Helm release. Installing over it would silently steal it, so
// the operator must abort and let the user reconcile.
type OwnershipConflict struct {
	Kind         string
	Namespace    string
	Name         string
	OwnerRelease string
	OwnerNS      string
}

// PreflightResult is the outcome of scanning the chart against the live cluster.
type PreflightResult struct {
	Adoptable []AdoptionCandidate
	Conflicts []OwnershipConflict
}

// HasConflicts returns true when the scan found foreign-owned resources.
func (r *PreflightResult) HasConflicts() bool {
	return len(r.Conflicts) > 0
}

// ConflictError renders a user-facing error that enumerates every conflict
// so operators can decide how to reconcile (uninstall the other release,
// rename the resource, etc.).
func (r *PreflightResult) ConflictError(release, namespace string) error {
	if !r.HasConflicts() {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b,
		"cannot install release %q in namespace %q: %d resource(s) are owned by a different Helm release:\n",
		release, namespace, len(r.Conflicts),
	)
	for _, c := range r.Conflicts {
		fmt.Fprintf(&b, "  - %s %s/%s owned by release %q in namespace %q\n",
			c.Kind, c.Namespace, c.Name, c.OwnerRelease, c.OwnerNS)
	}
	b.WriteString("Uninstall the conflicting release or rename the resources before retrying.")
	return fmt.Errorf("%s", b.String())
}

// RunOwnershipPreflight renders the chart client-side (no cluster mutation)
// and then classifies every rendered resource against the live cluster.
//
// Per resource:
//   - Does not exist               → ignored
//   - Exists, no helm annotations  → AdoptionCandidate (safe with TakeOwnership)
//   - Exists, annotations match    → already owned, nothing to do
//   - Exists, foreign annotations  → OwnershipConflict (caller must abort)
//
// The caller is expected to call HasConflicts()/ConflictError() and refuse to
// proceed when conflicts are present.
//
// Returns (result, needsTakeOwnership, error). When needsTakeOwnership is true,
// the caller should set TakeOwnership=true on the Install/Upgrade action.
func RunOwnershipPreflight(
	actionConfig *action.Configuration,
	chartRequested chart.Charter,
	values map[string]any,
	release, namespace, version string,
) (*PreflightResult, error) {
	// install.Run with DryRunStrategy=DryRunClient enters Helm v4's mock block
	// (pkg/action/install.go:329-344) and reassigns cfg.KubeClient to a
	// PrintingKubeClient and cfg.Releases to an in-memory driver. Because cfg
	// is a pointer, that poisons the caller's actionConfig for the subsequent
	// real Install/Upgrade. Save and restore those two fields around the
	// dry-run so the mocks stay scoped to the preflight render.
	savedKubeClient := actionConfig.KubeClient
	savedReleases := actionConfig.Releases
	defer func() {
		actionConfig.KubeClient = savedKubeClient
		actionConfig.Releases = savedReleases
	}()

	dryRun := action.NewInstall(actionConfig)
	dryRun.ReleaseName = release
	dryRun.Namespace = namespace
	dryRun.Version = version
	dryRun.DryRunStrategy = action.DryRunClient

	rel, err := dryRun.Run(chartRequested, values)
	// Restore the real client/storage before we use them again below for the
	// scan. The deferred restore stays as a safety net for panic/early-return.
	actionConfig.KubeClient = savedKubeClient
	actionConfig.Releases = savedReleases

	if err != nil {
		return nil, fmt.Errorf("preflight render chart: %w", err)
	}

	manifest, err := manifestFromRelease(rel)
	if err != nil {
		return nil, fmt.Errorf("preflight read manifest: %w", err)
	}

	resources, err := actionConfig.KubeClient.Build(strings.NewReader(manifest), false)
	if err != nil {
		if !isCRDNotInstalledError(err) {
			return nil, fmt.Errorf("preflight parse manifest: %w", err)
		}
		// The chart renders custom resources whose CRDs it ships itself
		// (e.g. kube-prometheus-stack's monitoring.coreos.com kinds). Those
		// kinds cannot be REST-mapped before install — but they also cannot
		// already exist in the cluster (no CRD => no instances), so they are
		// irrelevant to the ownership scan. Helm installs the chart's crds/
		// before applying templates, so the real install still succeeds.
		// Build uses ContinueOnError internally and returns the resources
		// that DID map alongside the error, so we proceed with those.
		helmLogger.Warn("ownership preflight: ignoring rendered resources with not-yet-installed CRDs",
			"releaseName", release, "namespace", namespace, "error", err.Error())
	}

	result := &PreflightResult{}
	err = resources.Visit(func(info *resource.Info, visitErr error) error {
		if visitErr != nil {
			return visitErr
		}
		return scanResource(info, release, namespace, result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// manifestFromRelease extracts the rendered YAML manifest from the Releaser
// interface returned by action.Install.Run. Helm v4 hides the concrete type
// behind an empty interface; the concrete *release.Release carries a Manifest
// field that we need for the preflight scan.
func manifestFromRelease(rel releaser.Releaser) (string, error) {
	concrete, ok := rel.(*release.Release)
	if !ok || concrete == nil {
		return "", fmt.Errorf("unexpected release type %T", rel)
	}
	return concrete.Manifest, nil
}

// CheckOwnershipAndLog runs the preflight scan and logs adoptable resources.
// Returns (needsTakeOwnership, error). On conflict, returns a user-facing error.
func CheckOwnershipAndLog(
	actionConfig *action.Configuration,
	chartRequested chart.Charter,
	values map[string]any,
	release, namespace, version string,
) (bool, error) {
	preflight, err := RunOwnershipPreflight(actionConfig, chartRequested, values, release, namespace, version)
	if err != nil {
		return false, err
	}
	if preflight.HasConflicts() {
		return false, preflight.ConflictError(release, namespace)
	}
	for _, c := range preflight.Adoptable {
		helmLogger.Info("adopting orphaned resource",
			"releaseName", release,
			"namespace", namespace,
			"kind", c.Kind,
			"resourceNamespace", c.Namespace,
			"name", c.Name,
		)
	}
	return len(preflight.Adoptable) > 0, nil
}

// isCRDNotInstalledError reports whether err is the "no REST mapping" failure
// that occurs when a rendered manifest references a kind whose CRD is not yet
// installed in the cluster. Charts that bundle their own CRDs in crds/ (e.g.
// kube-prometheus-stack) trigger this during the client-side preflight render,
// because Helm only installs crds/ during the real install. KubeClient.Build
// aggregates these as meta NoMatch errors; the message also carries Helm's
// "ensure CRDs are installed first" hint.
func isCRDNotInstalledError(err error) bool {
	if err == nil {
		return false
	}
	if meta.IsNoMatchError(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "no matches for kind") ||
		strings.Contains(msg, "resource mapping not found") ||
		strings.Contains(msg, "ensure CRDs are installed first")
}

// scanResource classifies one rendered resource against the live cluster and
// appends to the correct bucket in result.
func scanResource(info *resource.Info, release, namespace string, result *PreflightResult) error {
	helper := resource.NewHelper(info.Client, info.Mapping)
	obj, err := helper.Get(info.Namespace, info.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get %s %s/%s: %w",
			info.Mapping.GroupVersionKind.Kind, info.Namespace, info.Name, err)
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("access metadata for %s %s/%s: %w",
			info.Mapping.GroupVersionKind.Kind, info.Namespace, info.Name, err)
	}
	annotations := accessor.GetAnnotations()
	existingRelease := annotations[helmReleaseNameAnnotation]
	existingNS := annotations[helmReleaseNamespaceAnnotation]

	kind := info.Mapping.GroupVersionKind.Kind

	switch {
	case existingRelease == "" && existingNS == "":
		result.Adoptable = append(result.Adoptable, AdoptionCandidate{
			Kind:      kind,
			Namespace: info.Namespace,
			Name:      info.Name,
		})
	case existingRelease == release && existingNS == namespace:
		// already owned by us — Helm will upgrade, no adoption needed
	default:
		result.Conflicts = append(result.Conflicts, OwnershipConflict{
			Kind:         kind,
			Namespace:    info.Namespace,
			Name:         info.Name,
			OwnerRelease: existingRelease,
			OwnerNS:      existingNS,
		})
	}
	return nil
}
