package kubernetes

import (
	"context"
	"os/exec"
	"strings"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
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
	cert, err := GetCertificate(namespaceName, namespaceName)
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
		diff := utils.Diff(hostNames, cert.Spec.DNSNames)
		if len(diff) > 0 {
			foundChanges = true
		}
	} else {
		// cert is missing so create a new one
		foundChanges = true
	}

	// 3. Update the certificate if new Hostnames are beeing added.
	if foundChanges {
		provider := NewKubeProviderCertManager()
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

func AllCertificates(namespaceName string) []cmapi.Certificate {
	result := []cmapi.Certificate{}

	provider := NewKubeProviderCertManager()
	certificatesList, err := provider.ClientSet.CertmanagerV1().Certificates(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllCertificates ERROR: %s", err.Error())
		return result
	}

	for _, certificate := range certificatesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, certificate.ObjectMeta.Namespace) {
			result = append(result, certificate)
		}
	}
	return result
}

func GetCertificate(namespaceName string, resourceName string) (*cmapi.Certificate, error) {
	provider := NewKubeProviderCertManager()
	certificate, err := provider.ClientSet.CertmanagerV1().Certificates(namespaceName).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Errorf("GetCertificate ERROR: %s", err.Error())
		return nil, err
	}
	return certificate, nil
}

func AllK8sCertificates(namespaceName string) K8sWorkloadResult {
	result := []cmapi.Certificate{}

	provider := NewKubeProviderCertManager()
	certificatesList, err := provider.ClientSet.CertmanagerV1().Certificates(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllCertificates ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, certificate := range certificatesList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, certificate.ObjectMeta.Namespace) {
			result = append(result, certificate)
		}
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sCertificate(data cmapi.Certificate) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	certificateClient := kubeProvider.ClientSet.CertmanagerV1().Certificates(data.Namespace)
	_, err := certificateClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sCertificate(data cmapi.Certificate) K8sWorkloadResult {
	kubeProvider := NewKubeProviderCertManager()
	certificateClient := kubeProvider.ClientSet.CertmanagerV1().Certificates(data.Namespace)
	err := certificateClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sCertificate(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "certificate", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

func NewK8sCertificate() K8sNewWorkload {
	return NewWorkload(
		RES_CERTIFICATE,
		utils.InitCertificateYaml(),
		"A Certificate resource in cert-manager is used to request, manage, and store TLS certificates from certificate authorities. In this example, a Certificate named 'my-certificate' is created. It requests a TLS certificate for the domain names 'example.com' and 'www.example.com'. The certificate will be issued and managed by the ClusterIssuer named 'my-cluster-issuer'. The resulting certificate will be stored in a Secret named 'my-certificate-secret'. Please note that this is a simplified example, and the actual configuration may vary depending on the specific certificate authority and issuer being used. Always refer to the documentation for the certificate manager you are using and follow the guidelines provided by the certificate authority.")
}
