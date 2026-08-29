package uninstall

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	rbacv1api "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/controller"
)

func buildScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1api.AddToScheme(scheme)
	_ = appsv1api.AddToScheme(scheme)
	_ = rbacv1api.AddToScheme(scheme)
	_ = apiextv1.AddToScheme(scheme)
	_ = apiextv1beta1.AddToScheme(scheme)
	_ = velerov1api.AddToScheme(scheme)
	_ = velerov2alpha1api.AddToScheme(scheme)
	return scheme
}

func TestRun(t *testing.T) {
	scheme := buildScheme()
	namespace := "velero-custom"

	tests := []struct {
		name          string
		initialObjs   []kbclient.Object
		expectedError bool
	}{
		{
			name:          "Namespace does not exist",
			initialObjs:   []kbclient.Object{},
			expectedError: false,
		},
		{
			name: "CRDs missing but namespace exists",
			initialObjs: []kbclient.Object{
				&corev1api.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.initialObjs...).Build()
			err := Run(context.Background(), client, namespace)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestForcedlyDeleteResources(t *testing.T) {
	scheme := buildScheme()
	namespace := "velero"

	restoreWithFinalizer := &velerov1api.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-restore",
			Namespace: namespace,
			Finalizers: []string{
				controller.ExternalResourcesFinalizer,
			},
		},
	}

	deploy := &appsv1api.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "velero",
			Namespace: namespace,
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restoreWithFinalizer).WithObjects(restoreWithFinalizer, deploy).Build()

	resToDelete := []kbclient.ObjectList{
		&velerov1api.RestoreList{},
	}

	err := forcedlyDeleteResources(context.Background(), client, namespace, resToDelete)
	require.NoError(t, err)

	// Verify finalizer is removed
	err = client.Get(context.Background(), types.NamespacedName{Name: "test-restore", Namespace: namespace}, restoreWithFinalizer)
	require.NoError(t, err)
	assert.Empty(t, restoreWithFinalizer.ObjectMeta.Finalizers)
}

func TestCheckResources(t *testing.T) {
	scheme := buildScheme()

	v1crd := &apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "restores.velero.io",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(v1crd).Build()

	resToDelete, err := checkResources(context.Background(), client)
	require.NoError(t, err)
	assert.Len(t, resToDelete, 1)
	_, ok := resToDelete[0].(*velerov1api.RestoreList)
	assert.True(t, ok)
}
