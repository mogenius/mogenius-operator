package helm

import (
	"errors"
	"fmt"
	"strings"

	"helm.sh/helm/v4/pkg/action"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
)

const (
	// InstalledViaLabel is stamped on every release this operator installs. It
	// is what makes an existing release attributable later: a release without
	// it was put there by someone else.
	InstalledViaLabel = "mogenius.com/installed-via"
	// InstalledViaValue is the label's value for releases the operator owns.
	InstalledViaValue = "mogenius-operator"
)

// ReleaseOwnership is the answer to "may this operator write to that release?".
type ReleaseOwnership int

const (
	// ReleaseAbsent means no release of that name exists in the namespace.
	ReleaseAbsent ReleaseOwnership = iota
	// ReleaseOwnedByOperator means this operator installed it and may upgrade it.
	ReleaseOwnedByOperator
	// ReleaseForeign means a release exists that the operator did not install.
	// Upgrading it would overwrite someone else's values.
	ReleaseForeign
)

// LookupReleaseOwnership reports whether a release exists and whether this
// operator installed it.
//
// Anything other than a clean "not found" is returned as an error rather than
// as ReleaseAbsent: treating an unreachable API server as absence would install
// a second release next to one that is already there.
func LookupReleaseOwnership(namespace string, releaseName string) (ReleaseOwnership, error) {
	settings := NewCli()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, ""); err != nil {
		return ReleaseAbsent, fmt.Errorf("init helm config for %s/%s: %w", namespace, releaseName, err)
	}

	rel, err := action.NewGet(actionConfig).Run(releaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) || strings.Contains(err.Error(), "release: not found") {
			return ReleaseAbsent, nil
		}
		return ReleaseAbsent, fmt.Errorf("get release %s/%s: %w", namespace, releaseName, err)
	}

	re, ok := rel.(*release.Release)
	if !ok || re == nil {
		return ReleaseAbsent, nil
	}

	if re.Labels[InstalledViaLabel] == InstalledViaValue {
		return ReleaseOwnedByOperator, nil
	}
	return ReleaseForeign, nil
}
