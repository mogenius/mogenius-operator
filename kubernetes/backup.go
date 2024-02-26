package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	realJson "encoding/json"

	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"

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
			fmt.Println(string(objRaw.Raw))
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
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		return result, err
	}
	//create namespace not existing
	if len(unstructList) > 0 {
		namespaceClient := provider.ClientSet.CoreV1().Namespaces()
		ns, err := namespaceClient.Get(context.TODO(), namespaceName, v1.GetOptions{})
		if err != nil || ns == nil {
			newNs := applyconfcore.Namespace(namespaceName)
			_, err = namespaceClient.Apply(context.TODO(), newNs, v1.ApplyOptions{FieldManager: DEPLOYMENTNAME})
			time.Sleep(3 * time.Second) // Wait for 3 for namespace to be created
			if err != nil {
				logger.Log.Error(err.Error())
			}
		}

		client := dynamic.New(provider.ClientSet.RESTClient())
		groupResources, err := restmapper.GetAPIGroupResources(provider.ClientSet.Discovery())
		if err != nil {
			return result, err
		}
		rm := restmapper.NewDiscoveryRESTMapper(groupResources)
		for index, obj := range unstructList {
			_, err := ApplyUnstructured(context.TODO(), client, rm, obj, &result)
			if err != nil {
				aResult := fmt.Sprintf("%d) (%s) FAILED  : %s/%s '%s'", index+1, namespaceName, obj.GetKind(), obj.GetName(), err.Error())
				result.Messages = append(result.Messages, aResult)
				logger.Log.Error(aResult)
			} else {
				aResult := fmt.Sprintf("%d) (%s) SUCCESS : %s/%s", index+1, namespaceName, obj.GetKind(), obj.GetName())
				result.Messages = append(result.Messages, aResult)
				logger.Log.Info(aResult)
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
			unstructuredObj.SetNamespace(utils.CONFIG.Kubernetes.OwnNamespace)
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
	opts := v1.PatchOptions{FieldManager: DEPLOYMENTNAME, Force: &force}
	if _, err := dri.Patch(ctx, unstructuredObj.GetName(), types.ApplyPatchType, b, opts); err != nil {
		if isIncompatibleServerError(err) {
			err = fmt.Errorf("server-side apply not available on the server: (%v)", err)
			result.Messages = append(result.Messages, err.Error())
		}
		return nil, err
	}
	return nil, nil

	// modified, err := util.GetModifiedConfiguration(obj, true, unstructured.UnstructuredJSONScheme)
	// if err != nil {
	// 	return nil, fmt.Errorf("retrieving modified configuration from:\n%s\nfor:%v", unstructuredObj.GetName(), err)
	// }

	// currentUnstr, err := dri.Get(ctx, unstructuredObj.GetName(), metav1.GetOptions{})
	// if err != nil {
	// 	if !apierrors.IsNotFound(err) {
	// 		return nil, fmt.Errorf("retrieving current configuration of:\n%s\nfrom server for:%v", unstructuredObj.GetName(), err)
	// 	}

	// 	logger.Log.Infof("The resource %s creating", unstructuredObj.GetName())
	// 	// Create the resource if it doesn't exist
	// 	// First, update the annotation such as kubectl apply
	// 	if err := util.CreateApplyAnnotation(&unstructuredObj, unstructured.UnstructuredJSONScheme); err != nil {
	// 		return nil, fmt.Errorf("creating %s error: %v", unstructuredObj.GetName(), err)
	// 	}

	// 	return dri.Create(ctx, &unstructuredObj, metav1.CreateOptions{})
	// }

	// metadata, _ := meta.Accessor(currentUnstr)
	// annotationMap := metadata.GetAnnotations()
	// if _, ok := annotationMap[corev1.LastAppliedConfigAnnotation]; !ok {
	// 	logger.Log.Warningf("[%s] apply should be used on resource created by either kubectl create --save-config or apply", metadata.GetName())
	// }

	// patchBytes, patchType, err := Patch(currentUnstr, modified, unstructuredObj.GetName(), *gvk)
	// if err != nil {
	// 	return nil, err
	// }
	// return dri.Patch(ctx, unstructuredObj.GetName(), patchType, patchBytes, metav1.PatchOptions{})
}

// Taken from https://github.com/pytimer/k8sutil
// func Patch(currentUnstr *unstructured.Unstructured, modified []byte, name string, gvk schema.GroupVersionKind) ([]byte, types.PatchType, error) {
// 	current, err := currentUnstr.MarshalJSON()
// 	if err != nil {
// 		return nil, "", fmt.Errorf("serializing current configuration from: %v, %v", currentUnstr, err)
// 	}

// 	original, err := util.GetOriginalConfiguration(currentUnstr)
// 	if err != nil {
// 		return nil, "", fmt.Errorf("retrieving original configuration from: %s, %v", name, err)
// 	}

// 	var patchType types.PatchType
// 	var patch []byte

// 	Scheme := runtime.NewScheme()
// 	versionedObject, err := Scheme.New(gvk)
// 	switch {
// 	case runtime.IsNotRegisteredError(err):
// 		patchType = types.MergePatchType
// 		preconditions := []mergepatch.PreconditionFunc{
// 			mergepatch.RequireKeyUnchanged("apiVersion"),
// 			mergepatch.RequireKeyUnchanged("kind"),
// 			mergepatch.RequireKeyUnchanged("name"),
// 		}
// 		patch, err = jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current, preconditions...)
// 		if err != nil {
// 			if mergepatch.IsPreconditionFailed(err) {
// 				return nil, "", fmt.Errorf("At least one of apiVersion, kind and name was changed")
// 			}
// 			return nil, "", fmt.Errorf("unable to apply patch, %v", err)
// 		}
// 	case err == nil:
// 		patchType = types.StrategicMergePatchType
// 		lookupPatchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
// 		if err != nil {
// 			return nil, "", err
// 		}
// 		patch, err = strategicpatch.CreateThreeWayMergePatch(original, modified, current, lookupPatchMeta, true)
// 		if err != nil {
// 			return nil, "", err
// 		}
// 	case err != nil:
// 		return nil, "", fmt.Errorf("getting instance of versioned object %v for: %v", gvk, err)
// 	}

// 	return patch, patchType, nil
// }

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
	skippedGroups := punqStructs.NewUniqueStringArray()
	allResources := punqStructs.NewUniqueStringArray()
	usedResources := punqStructs.NewUniqueStringArray()

	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		return result, err
	}

	// Get a list of all resource types in the cluster
	resourceList, err := provider.ClientSet.Discovery().ServerPreferredResources()
	if err != nil {
		return result, err
	}

	output := ""
	if namespace != "" {
		output = namespaceString(namespace)
	}
	// Iterate over each resource type and backup all resources in the namespace
	for _, resource := range resourceList {
		if punqUtils.Contains(utils.CONFIG.Misc.IgnoreResourcesBackup, resource.GroupVersion) {
			continue
		}
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
			if punqUtils.Contains(utils.CONFIG.Misc.IgnoreResourcesBackup, aApiResource.Name) {
				continue
			}

			resourceId := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: aApiResource.Name,
			}
			// Get the REST client for this resource type
			restClient := dynamic.New(provider.ClientSet.RESTClient()).Resource(resourceId).Namespace(namespace)

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

	os.WriteFile("/Users/bene/Desktop/omg.yaml", []byte(output), 0777)

	fmt.Printf("\nSKIP   : %s\n", strings.Join(utils.CONFIG.Misc.IgnoreResourcesBackup, ", "))
	fmt.Printf("\nALL    : %s\n", allResources.Display())
	fmt.Printf("\nSKIPPED: %s\n", skippedGroups.Display())
	fmt.Printf("\nUSED   : %s\n", usedResources.Display())

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
