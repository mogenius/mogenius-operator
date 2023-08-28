package kubernetes

import (
	"context"
	"strings"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateNamespaceCertificate(namespaceName string, hostNames []string) {
	if utils.CONFIG.Misc.Debug {
		logger.Log.Noticef("Updating Ingress for [%s] ...", strings.Join(hostNames, ", "))
	}
	if len(hostNames) <= 0 {
		return
	}

	foundChanges := false
	createNew := false

	// 1. Get Certificate for Namespace (NAMESPACE AND RESOURCE NAME ARE IDENTICAL)
	cert, err := punq.GetCertificate(namespaceName, namespaceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			createNew = true
		}
		if apierrors.IsForbidden(err) {
			logger.Log.Errorf("UpdateNamespaceCertificate ERROR: %s", err.Error())
			return
		}
	}

	// 2. Check if new Names have been added
	if cert != nil {
		diff := punqUtils.Diff(hostNames, cert.Spec.DNSNames)
		if len(diff) > 0 {
			foundChanges = true
		}
	} else {
		// cert is missing so create a new one
		foundChanges = true
	}

	// 3. Update the certificate if new Hostnames are beeing added.
	if foundChanges {
		provider := punq.NewKubeProviderCertManager()
		if createNew {
			cert := utils.InitCertificate()
			cert.Name = namespaceName
			cert.Namespace = namespaceName
			cert.Spec.DNSNames = hostNames
			cert.Spec.SecretName = namespaceName
			provider.ClientSet.CertmanagerV1().Certificates(namespaceName).Create(context.TODO(), &cert, metav1.CreateOptions{})

			if utils.CONFIG.Misc.Debug {
				logger.Log.Info("Certificate has been created because something changed.")
			}
			return
		} else {
			if cert == nil {
				cert := utils.InitCertificate()
				cert.Name = namespaceName
				cert.Namespace = namespaceName
				cert.Spec.SecretName = namespaceName
				cert.Spec.DNSNames = hostNames
			} else {
				cert.Spec.DNSNames = hostNames
			}
			provider.ClientSet.CertmanagerV1().Certificates(namespaceName).Update(context.TODO(), cert, metav1.UpdateOptions{})

			if utils.CONFIG.Misc.Debug {
				logger.Log.Info("Certificate has been updated because something changed.")
			}
			return
		}
	}
	if utils.CONFIG.Misc.Debug {
		logger.Log.Info("Certificate has NOT been updated/created because nothing changed.")
	}
}
