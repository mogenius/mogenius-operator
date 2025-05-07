package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/structs"
	"net/http"
	"sort"
	"time"

	realJson "encoding/json"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yaml2 "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	yaml1 "k8s.io/apimachinery/pkg/util/yaml"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

type NamespaceRestoreResponse struct {
	NamespaceName string   `json:"namespaceName"`
	Messages      []string `json:"messages"`
}

type NamespaceBackupResponse struct {
	NamespaceName string   `json:"namespaceName"`
	Data          string   `json:"data"`
	Messages      []string `json:"messages"`
}

func RestoreNamespace(inputYaml string, namespaceName string) (NamespaceRestoreResponse, error) {
	// Parse and prepare the data
	result := NamespaceRestoreResponse{
		NamespaceName: namespaceName,
	}
	var unstructList []unstructured.Unstructured
	yamlBytes := []byte(inputYaml)
	decoder := yaml1.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), len(yamlBytes))
	for {
		var objRaw runtime.RawExtension
		err := decoder.Decode(&objRaw)
		if err != nil {
			if err == io.EOF {
				break
			}
			return result, err
		}
		if len(objRaw.Raw) == 0 {
			continue
		}
		obj, _, err := yaml2.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(objRaw.Raw, nil, nil)
		if err != nil {
			result.Messages = append(result.Messages, err.Error())
			break
		}
		unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		unstructuredObj := unstructured.Unstructured{Object: unstrRaw}
		unstructuredObj.SetNamespace(namespaceName) // overwrite namespace
		if err != nil {
			return result, err
		}
		unstructList = append(unstructList, unstructuredObj)
	}

	sortWithPreference(unstructList)

	// SEND DATA TO K8S
	clientset := clientProvider.K8sClientSet()
	//create namespace not existing
	if len(unstructList) > 0 {
		namespaceClient := clientset.CoreV1().Namespaces()
		ns, err := namespaceClient.Get(context.TODO(), namespaceName, v1.GetOptions{})
		if err != nil || ns == nil {
			newNs := applyconfcore.Namespace(namespaceName)
			_, err = namespaceClient.Apply(context.TODO(), newNs, v1.ApplyOptions{FieldManager: GetOwnDeploymentName(config)})
			time.Sleep(3 * time.Second) // Wait for 3 for namespace to be created
			if err != nil {
				k8sLogger.Error(err.Error())
			}
		}

		client := dynamic.New(clientset.RESTClient())
		groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
		if err != nil {
			return result, err
		}
		rm := restmapper.NewDiscoveryRESTMapper(groupResources)
		for index, obj := range unstructList {
			_, err := ApplyUnstructured(context.TODO(), client, rm, obj, &result)
			if err != nil {
				aResult := fmt.Sprintf("%d) (%s) FAILED  : %s/%s '%s'", index+1, namespaceName, obj.GetKind(), obj.GetName(), err.Error())
				result.Messages = append(result.Messages, aResult)
				k8sLogger.Error(aResult)
			} else {
				aResult := fmt.Sprintf("%d) (%s) SUCCESS : %s/%s", index+1, namespaceName, obj.GetKind(), obj.GetName())
				result.Messages = append(result.Messages, aResult)
				k8sLogger.Info(aResult)
			}
		}
	}
	return result, nil
}

// Taken from https://github.com/pytimer/k8sutil
func ApplyUnstructured(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, unstructuredObj unstructured.Unstructured, result *NamespaceRestoreResponse) (*unstructured.Unstructured, error) {
	if len(unstructuredObj.GetName()) == 0 {
		metadata, _ := meta.Accessor(unstructuredObj)
		generateName := metadata.GetGenerateName()
		if len(generateName) > 0 {
			return nil, fmt.Errorf("from %s: cannot use generate name with apply", generateName)
		}
	}

	b, err := unstructuredObj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	_, gvk, err := yaml2.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(b, nil, nil)
	if err != nil {
		return nil, err
	}

	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	var dri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if unstructuredObj.GetNamespace() == "" {
			unstructuredObj.SetNamespace(config.Get("MO_OWN_NAMESPACE"))
		}
		dri = dynamicClient.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
	} else {
		dri = dynamicClient.Resource(mapping.Resource)
	}

	if _, ok := unstructuredObj.GetAnnotations()[corev1.LastAppliedConfigAnnotation]; ok {
		annotations := unstructuredObj.GetAnnotations()
		delete(annotations, corev1.LastAppliedConfigAnnotation)
		unstructuredObj.SetAnnotations(annotations)
	}
	unstructuredObj.SetManagedFields(nil)

	force := true
	opts := v1.PatchOptions{FieldManager: GetOwnDeploymentName(config), Force: &force}
	if _, err := dri.Patch(ctx, unstructuredObj.GetName(), types.ApplyPatchType, b, opts); err != nil {
		if isIncompatibleServerError(err) {
			err = fmt.Errorf("server-side apply not available on the server: (%v)", err)
			result.Messages = append(result.Messages, err.Error())
		}
		return nil, err
	}
	return nil, nil
}

func isIncompatibleServerError(err error) bool {
	if _, ok := err.(*apierrors.StatusError); !ok {
		return false
	}
	return err.(*apierrors.StatusError).Status().Code == http.StatusUnsupportedMediaType
}

func BackupNamespace(namespace string) (NamespaceBackupResponse, error) {
	result := NamespaceBackupResponse{
		NamespaceName: namespace,
	}
	skippedGroups := structs.NewUniqueStringArray()
	allResources := structs.NewUniqueStringArray()
	usedResources := structs.NewUniqueStringArray()

	// Get a list of all resource types in the cluster
	clientset := clientProvider.K8sClientSet()
	resourceList, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		return result, err
	}

	output := ""
	if namespace != "" {
		output = namespaceString(namespace)
	}
	// Iterate over each resource type and backup all resources in the namespace
	for _, resource := range resourceList {
		gv, _ := schema.ParseGroupVersion(resource.GroupVersion)
		if len(resource.APIResources) <= 0 {
			continue
		}

		for _, aApiResource := range resource.APIResources {
			allResources.Add(aApiResource.Name)
			if !aApiResource.Namespaced && namespace != "" {
				skippedGroups.Add(aApiResource.Name)
				continue
			}

			resourceId := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: aApiResource.Name,
			}
			// Get the REST client for this resource type
			restClient := dynamic.New(clientset.RESTClient()).Resource(resourceId).Namespace(namespace)

			// Get a list of all resources of this type in the namespace
			list, err := restClient.List(context.Background(), v1.ListOptions{})
			if err != nil {
				result.Messages = append(result.Messages, fmt.Sprintf("(LIST) %s: %s", resourceId.Resource, err.Error()))
				continue
			}

			if len(list.Items) > 0 {
				usedResources.Add(aApiResource.Name)
			}

			// Iterate over each resource and write it to a file
			for _, obj := range list.Items {
				if obj.GetKind() == "Event" {
					continue
				}
				output = output + "---\n"
				result.Messages = append(result.Messages, fmt.Sprintf("(SUCCESS) %s: %s/%s", resourceId.Resource, obj.GetNamespace(), obj.GetName()))

				obj = cleanBackupResources(obj)

				json, err := realJson.Marshal(obj.Object)
				if err != nil {
					return result, err
				}
				data, err := yaml.JSONToYAML(json)
				if err != nil {
					return result, err
				}
				output = output + string(data)
			}
		}
	}
	result.Data = output

	//os.WriteFile("/Users/bene/Desktop/omg.yaml", []byte(output), 0777)

	k8sLogger.Info("ALL", "resources", allResources.Display())
	k8sLogger.Info("SKIPPED", "resources", skippedGroups.Display())
	k8sLogger.Info("USED", "resources", usedResources.Display())

	return result, nil
}

// Some Kinds must be executed before other kinds. The order is important.
func sortWithPreference(objs []unstructured.Unstructured) {
	sort.Slice(objs, func(i, j int) bool {
		if objs[i].GetKind() == "ServiceAccount" {
			return true
		} else if objs[i].GetKind() == "ServiceAccount" {
			return false
		} else {
			return objs[i].GetKind() < objs[j].GetKind() // sort remaining elements in ascending order
		}
	})
}

func namespaceString(ns string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
	name: %s
`, ns)
}

func cleanBackupResources(obj unstructured.Unstructured) unstructured.Unstructured {
	obj.SetManagedFields(nil)
	delete(obj.Object, "status")
	obj.SetUID("")
	obj.SetResourceVersion("")
	obj.SetCreationTimestamp(v1.Time{})

	if obj.GetKind() == "PersistentVolumeClaim" {
		if nested, ok := obj.Object["spec"].(map[string]interface{}); ok {
			delete(nested, "volumeName")
			obj.Object["spec"] = nested
		}

		cleanedAnnotations := obj.GetAnnotations()
		delete(cleanedAnnotations, "pv.kubernetes.io/bind-completed")
		obj.SetAnnotations(cleanedAnnotations)
	}

	return obj
}
