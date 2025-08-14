package core

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"net/http"
	"strings"

	sealedsecretv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type SealedSecretManager interface {
	CreateSealedSecretFromExisting(secretName, namespace string) (*unstructured.Unstructured, error)
	GetMainSecret() (*v1.Secret, error)
}

type sealedSecretManager struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
}

func NewSealedSecretManager(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProvider k8sclient.K8sClientProvider,
) SealedSecretManager {
	self := &sealedSecretManager{}
	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider

	return self
}

var sealedSecretGVR = schema.GroupVersionResource{
	Group:    "bitnami.com",
	Version:  "v1alpha1",
	Resource: "sealedsecrets",
}

var secretGVR = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "secrets",
}

func (s *sealedSecretManager) fetchPublicKeyViaHTTP() (*rsa.PublicKey, error) {
	// here you can add a certificate for testing
	publicKeyData := []byte("")
	if len(publicKeyData) == 0 {
		namespace, serviceName, port, err := kubernetes.FindSealedSecretsService()
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/v1/cert.pem", serviceName, namespace, port)

		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch public key: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch public key, status: %d", resp.StatusCode)
		}

		publicKeyData, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
	}

	block, _ := pem.Decode(publicKeyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	rsaPublicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate does not contain an RSA public key")
	}

	return rsaPublicKey, nil
}

func (s *sealedSecretManager) CreateSealedSecretFromExisting(secretName, namespace string) (*unstructured.Unstructured, error) {
	publicKey, err := s.fetchPublicKeyViaHTTP()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}
	// Get the existing secret
	secretUnstr, err := s.clientProvider.DynamicClient().Resource(secretGVR).Namespace(namespace).Get(
		context.TODO(),
		secretName,
		metav1.GetOptions{},
	)
	if err != nil || secretUnstr == nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}
	s.logger.Debug("Found secret", "name", secretName, "namespace", namespace)

	// Create SealedSecret directly as unstructured object
	secret := &corev1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(secretUnstr.Object, secret); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured secret to typed secret: %w", err)
	}
	sealedSecret, err := s.createUnstructuredSealedSecretFromSecret(secret, publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create sealed secret: %w", err)
	}

	// Apply the SealedSecret
	createdSealedSecret, err := s.clientProvider.DynamicClient().Resource(sealedSecretGVR).Namespace(namespace).Create(
		context.TODO(),
		sealedSecret,
		metav1.CreateOptions{},
	)
	if err != nil || createdSealedSecret == nil {
		return nil, fmt.Errorf("failed to create sealed secret: %w", err)
	}

	s.logger.Debug("Created SealedSecret", "name", sealedSecret.GetName(), "namespace", namespace)

	// update the original secret
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	secret.Annotations["sealedsecrets.bitnami.com/managed"] = "true"
	_, err = s.clientProvider.K8sClientSet().CoreV1().Secrets(namespace).Update(
		context.TODO(),
		secret,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update original secret with annotations %s/%s: %w", namespace, secretName, err)
	}

	// clean created SealedSecret
	unstructured.RemoveNestedField(createdSealedSecret.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(createdSealedSecret.Object, "spec", "template", "metadata", "managedFields")

	return createdSealedSecret, nil
}

func (s *sealedSecretManager) GetMainSecret() (*v1.Secret, error) {
	clientset := s.clientProvider.K8sClientSet()
	client := clientset.CoreV1().Secrets("")
	secretsList, err := client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %v", err)
	}
	for _, secret := range secretsList.Items {
		if strings.HasPrefix(secret.Name, "sealed-secrets-key") {
			return &secret, nil
		}
	}
	return nil, fmt.Errorf("sealed-secrets secret not found in any namespace")
}

func (s *sealedSecretManager) createUnstructuredSealedSecretFromSecret(secret *corev1.Secret, publicKey *rsa.PublicKey) (*unstructured.Unstructured, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add core v1 to scheme: %w", err)
	}
	codecFactory := serializer.NewCodecFactory(scheme)
	newSealedSecret, err := sealedsecretv1alpha1.NewSealedSecret(codecFactory, publicKey, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to seal secret: %w", err)
	}

	// add missing fields to the SealedSecret
	newSealedSecret.Kind = "SealedSecret"
	newSealedSecret.APIVersion = "bitnami.com/v1alpha1"

	sealedSecretMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newSealedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create unstructured object: %w", err)
	}
	return &unstructured.Unstructured{Object: sealedSecretMap}, nil
}
