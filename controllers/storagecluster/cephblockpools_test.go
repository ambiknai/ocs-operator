package storagecluster

import (
	"context"
	"testing"

	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/openshift/ocs-operator/api/v1"
)

func TestCephBlockPools(t *testing.T) {
	//cases for testing
	var cases = []struct {
		label                string
		createRuntimeObjects bool
	}{
		{
			label:                "case 1",
			createRuntimeObjects: false,
		},
	}
	for _, eachPlatform := range allPlatforms {
		cp := &Platform{platform: eachPlatform}
		for _, c := range cases {
			var objects []runtime.Object
			t, reconciler, cr, request := initStorageClusterResourceCreateUpdateTestWithPlatform(
				t, cp, objects)
			if c.createRuntimeObjects {
				isavoidObjectStore, _ := avoidObjectStore(cp.platform, cr.Namespace, reconciler.Client)
				objects = createUpdateRuntimeObjects(cp, isavoidObjectStore)
				t, reconciler, cr, request = initStorageClusterResourceCreateUpdateTestWithPlatform(
					t, cp, objects)
			}
			assertCephBlockPools(t, reconciler, cr, request)
		}
	}
}

func assertCephBlockPools(t *testing.T, reconciler StorageClusterReconciler, cr *api.StorageCluster, request reconcile.Request) {
	actualCbp := &cephv1.CephBlockPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephblockpool",
		},
	}
	request.Name = "ocsinit-cephblockpool"
	err := reconciler.Client.Get(context.TODO(), request.NamespacedName, actualCbp)
	assert.NoError(t, err)

	expectedCbp, err := reconciler.newCephBlockPoolInstances(cr)
	assert.NoError(t, err)

	assert.Equal(t, len(expectedCbp[0].OwnerReferences), 1)

	assert.Equal(t, expectedCbp[0].ObjectMeta.Name, actualCbp.ObjectMeta.Name)
	assert.Equal(t, expectedCbp[0].Spec, actualCbp.Spec)
}
