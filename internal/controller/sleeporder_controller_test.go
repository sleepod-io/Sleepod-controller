package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSnapshotReplicas(t *testing.T) {
	// 1. Setup Scheme (to know about Deployment types)
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)

	// 2. Create a "Fake" Deployment
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

	// 2. Create a "Fake" StatefulSet
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	// 3. Create a Fake Client with this object pre-loaded
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment, statefulset).
		Build()

	// 4. Create the Reconciler with the fake client
	r := &SleepOrderReconciler{
		Client: cl,
		Scheme: scheme,
	}

	// 5. Call the function (we haven't written it yet!)
	ctx := context.Background()
	// We need to fetch the object first to pass it, or the function fetches it?
	// Let's assume the function takes the object as an argument for now,
	// or we pass the key. Let's pass the object to keep it simple.
	tests := []struct{
		name string
		object client.Object
	}{
		{"Deployment", deployment},
		{"StatefulSet", statefulset},
	}

	for _, tt := range tests {
		err := r.snapshotReplicas(ctx, tt.object)
		if err != nil {
			t.Fatalf("snapshotReplicas failed: %v", err)
		}
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
			if val != "5" {
				t.Errorf("expected annotation '5', got '%s'", val)
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
			if val != "5" {
				t.Errorf("expected annotation '5', got '%s'", val)
			}
		default:
			t.Fatalf("unknown resource type: %T", tt.object)
		}
	}
}
