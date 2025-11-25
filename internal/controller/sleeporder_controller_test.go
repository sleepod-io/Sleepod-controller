package controller

import (
	"context"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSnapshotReplicas(t *testing.T) {
	// Setup Scheme (to know about Deployment types)
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)

	// Create a "Fake" Deployment
	replicas := int32(5)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	// Create a "Fake" StatefulSet
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	// Create a Fake Client with this object pre-loaded
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, statefulset).
		Build()

	// Create the Reconciler with the fake client
	r := &SleepOrderReconciler{
		Client: cl,
		Scheme: scheme,
	}

	ctx := context.Background()
	tests := []struct {
		name   string
		object client.Object
	}{
		{"Deployment", deployment},
		{"StatefulSet", statefulset},
	}

	for _, tt := range tests {
		currentReplicas, err := r.snapshotReplicas(ctx, tt.object)
		if err != nil {
			t.Fatalf("snapshotReplicas failed: %v", err)
		}
		// Check that the annotation was added
		switch tt.object.(type) {
		case *appsv1.Deployment:
			updatedDeployment := tt.object.(*appsv1.Deployment)
			err = cl.Get(ctx, client.ObjectKeyFromObject(deployment), updatedDeployment)
			if err != nil {
				t.Fatalf("failed to get updated deployment: %v", err)
			}
			val, ok := updatedDeployment.Annotations["sleepod.io/original-replicas"]
			if !ok {
				t.Error("annotation 'sleepod.io/original-replicas' not found")
			}
			if val != strconv.Itoa(int(currentReplicas)) {
				t.Errorf("expected annotation '%d', got '%s'", currentReplicas, val)
			}
		case *appsv1.StatefulSet:
			updatedStatefulSet := tt.object.(*appsv1.StatefulSet)
			err = cl.Get(ctx, client.ObjectKeyFromObject(statefulset), updatedStatefulSet)
			if err != nil {
				t.Fatalf("failed to get updated statefulset: %v", err)
			}
			val, ok := updatedStatefulSet.Annotations["sleepod.io/original-replicas"]
			if !ok {
				t.Error("annotation 'sleepod.io/original-replicas' not found")
			}
			if val != strconv.Itoa(int(currentReplicas)) {
				t.Errorf("expected annotation '%d', got '%s'", currentReplicas, val)
			}
		default:
			t.Fatalf("unknown resource type: %T", tt.object)
		}
	}
}

func TestRestoreReplicas(t *testing.T) {
	// Setup Scheme (to know about Deployment types)
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)

	// Create a "Fake" Deployment
	replicas := int32(0)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				annotationKey: "5",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	// Create a "Fake" StatefulSet
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
			Annotations: map[string]string{
				annotationKey: "5",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	// Create a Fake Client with this object pre-loaded
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, statefulset).
		Build()

	// Create the Reconciler with the fake client
	r := &SleepOrderReconciler{
		Client: cl,
		Scheme: scheme,
	}

	ctx := context.Background()
	tests := []struct {
		name   string
		object client.Object
	}{
		{"Deployment", deployment},
		{"StatefulSet", statefulset},
	}

	for _, tt := range tests {
		originalReplicas, err := r.restoreReplicas(ctx, tt.object)
		if err != nil {
			t.Fatalf("restoreReplicas failed: %v", err)
		}
		// Verify that the replicas were restored
		switch tt.object.(type) {
		case *appsv1.Deployment:
			updatedDeployment := tt.object.(*appsv1.Deployment)
			err = cl.Get(ctx, client.ObjectKeyFromObject(deployment), updatedDeployment)
			if err != nil {
				t.Fatalf("failed to get updated deployment: %v", err)
			}
			if *updatedDeployment.Spec.Replicas != originalReplicas {
				t.Errorf("expected replicas '%d', got '%d'", originalReplicas, *updatedDeployment.Spec.Replicas)
			}
			// Verify that the annotation was removed
			if _, ok := updatedDeployment.Annotations[annotationKey]; ok {
				t.Errorf("annotation '%s' should have been removed", annotationKey)
			}
		case *appsv1.StatefulSet:
			updatedStatefulSet := tt.object.(*appsv1.StatefulSet)
			err = cl.Get(ctx, client.ObjectKeyFromObject(statefulset), updatedStatefulSet)
			if err != nil {
				t.Fatalf("failed to get updated statefulset: %v", err)
			}
			if *updatedStatefulSet.Spec.Replicas != originalReplicas {
				t.Errorf("expected replicas '%d', got '%d'", originalReplicas, *updatedStatefulSet.Spec.Replicas)
			}
			// Verify that the annotation was removed
			if _, ok := updatedStatefulSet.Annotations[annotationKey]; ok {
				t.Errorf("annotation '%s' should have been removed", annotationKey)
			}
		default:
			t.Fatalf("unknown resource type: %T", tt.object)
		}
	}
}

func TestReconcile_sleepFlow(t *testing.T) {
	// TODO: Test Reconcile sleep flow
}

func TestReconcile_wakeFlow(t *testing.T) {
	// TODO: Test Reconcile wake flow
}
