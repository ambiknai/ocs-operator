package storagecluster

import (
	"context"
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
)

//IBMCloud COS[Cloud Object Storage] secret name
const ibmCloudCosSecretName = "ibm-cloud-cos-creds"

// AvoidObjectStorePlatforms is a list of all PlatformTypes where CephObjectStores will not be deployed.
var AvoidObjectStorePlatforms = []configv1.PlatformType{
	configv1.AWSPlatformType,
	configv1.GCPPlatformType,
	configv1.AzurePlatformType,
	configv1.IBMCloudPlatformType,
}

// TuneFastPlatforms is a list of all PlatformTypes where TuneFastDeviceClass has to be set True.
var TuneFastPlatforms = []configv1.PlatformType{
	configv1.OvirtPlatformType,
	configv1.IBMCloudPlatformType,
	configv1.AzurePlatformType,
}

// Platform is used to get the CloudPlatformType of the running cluster in a thread-safe manner
type Platform struct {
	platform configv1.PlatformType
	mux      sync.Mutex
}

// GetPlatform is used to get the CloudPlatformType of the running cluster
func (p *Platform) GetPlatform(c client.Client) (configv1.PlatformType, error) {
	// if 'platform' is already set just return it
	if p.platform != "" {
		return p.platform, nil
	}
	p.mux.Lock()
	defer p.mux.Unlock()

	return p.getPlatform(c)
}

func (p *Platform) getPlatform(c client.Client) (configv1.PlatformType, error) {
	infrastructure := &configv1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
	err := c.Get(context.TODO(), types.NamespacedName{Name: infrastructure.ObjectMeta.Name}, infrastructure)
	if err != nil {
		return "", fmt.Errorf("could not get infrastructure details to determine cloud platform: %v", err)
	}

	p.platform = infrastructure.Status.Platform //nolint:staticcheck
	return p.platform, nil
}

func (r *StorageClusterReconciler) avoidObjectStore(p configv1.PlatformType, namespace string, c client.Client) (bool, error) {
	for _, platform := range AvoidObjectStorePlatforms {
		if p == platform {
			if p == configv1.IBMCloudPlatformType {
				isSecretPresent := false
				isIBM, err := IsIBMPlatform(p, c)
				if err != nil {
					return false, err
				}
				if isIBM {
					isSecretPresent, err = IsCosSecretPresent(namespace, c)
					if err != nil {
						return false, err
					}
					if isSecretPresent {
						return true, nil
					}
					// IsIBM but secret not present
					return false, nil
				}
			}
			// platform is something other than IBMCloud in AvoidObjectStorePlatforms
			return true, nil
		}
	}
	return false, nil
}

// IsIBMPlatform returns true if this cluster is running on IBM Cloud
// IBM Satellite clusters are not considered as IBMCloudPlatform
func IsIBMPlatform(p configv1.PlatformType, c client.Client) (bool, error) {
	isIBM := true
	nodeList := &corev1.NodeList{}
	err := c.List(context.TODO(), nodeList)
	if err != nil {
		return isIBM, fmt.Errorf("failed to list kubernetes nodes: %s", err)
	}
	// In case of Satellite, cluster is deployed in user provided infrastructure
	if strings.Contains(nodeList.Items[0].Spec.ProviderID, "/sat-") {
		isIBM = false
	}

	return isIBM, nil
}

// IsCosSecretPresent checks for ibm-cos-cred secret in the concerned namespace
// if platform is IBMCloud, enable CephObjectStore only if ibm-cloud-cos-creds secret is not present
// in the target namespace
func IsCosSecretPresent(namespace string, c client.Client) (bool, error) {
	foundSecret := &corev1.Secret{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: ibmCloudCosSecretName, Namespace: namespace}, foundSecret)
	if err != nil && errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// Secret is present.
	return true, nil
}

func (r *StorageClusterReconciler) DevicesDefaultToFastForThisPlatform() (bool, error) {
	c := r.Client
	platform, err := r.platform.GetPlatform(c)
	if err != nil {
		return false, err
	}

	for _, tfplatform := range TuneFastPlatforms {
		if platform == tfplatform {
			return true, nil
		}
	}

	return false, nil
}
