package controller

import (
	"context"
	"strconv"
	"testing"
	"time"

	sleepodv1alpha1 "github.com/shaygef123/SleePod-controller/api/v1alpha1"
	"github.com/shaygef123/SleePod-controller/internal/logic"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TODO: make consts for all the test values.
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
	// Setup Scheme (to know about Deployment types)
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = sleepodv1alpha1.AddToScheme(scheme)

	wakeAt := time.Now().UTC().Add(time.Hour).Format("15:04")
	sleepAt := time.Now().UTC().Add(-1 * time.Hour).Format("15:04")
	timezone := "UTC"

	tests := []struct {
		name             string
		targetObj        client.Object
		sleepOrder       *sleepodv1alpha1.SleepOrder
		initialReplicas  int32
		expectedReplicas int32
	}{
		{
			name: "Deployment with 3 replicas",
			targetObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-3",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { i := int32(3); return &i }(),
				},
			},
			sleepOrder: &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-sleeporder-3",
					Namespace: "default",
				},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "Deployment",
						Name: "test-deployment-3",
					},
					WakeAt:   wakeAt,
					SleepAt:  sleepAt,
					Timezone: timezone,
				},
			},
			initialReplicas:  3,
			expectedReplicas: 0,
		},
		{
			name: "StatefulSet with 3 replicas",
			targetObj: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset-3",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(3); return &i }(),
				},
			},
			sleepOrder: &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset-sleeporder-3",
					Namespace: "default",
				},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "StatefulSet",
						Name: "test-statefulset-3",
					},
					WakeAt:   wakeAt,
					SleepAt:  sleepAt,
					Timezone: timezone,
				},
			},
			initialReplicas:  3,
			expectedReplicas: 0,
		},
		{
			name: "Deployment with 0 replicas",
			targetObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-0",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { i := int32(0); return &i }(),
				},
			},
			sleepOrder: &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-sleeporder-0",
					Namespace: "default",
				},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "Deployment",
						Name: "test-deployment-0",
					},
					WakeAt:   wakeAt,
					SleepAt:  sleepAt,
					Timezone: timezone,
				},
			},
			initialReplicas:  0,
			expectedReplicas: 0,
		},
		{
			name: "StatefulSet with 0 replicas",
			targetObj: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset-0",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(0); return &i }(),
				},
			},
			sleepOrder: &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset-sleeporder-0",
					Namespace: "default",
				},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "StatefulSet",
						Name: "test-statefulset-0",
					},
					WakeAt:   wakeAt,
					SleepAt:  sleepAt,
					Timezone: timezone,
				},
			},
			initialReplicas:  0,
			expectedReplicas: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Fake Client with this object pre-loaded
			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(tt.sleepOrder).
				WithObjects(tt.targetObj, tt.sleepOrder).
				Build()

			// Create the Reconciler with the fake client
			r := &SleepOrderReconciler{
				Client: cl,
				Scheme: scheme,
			}

			req := ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(tt.sleepOrder),
			}
			result, err := r.Reconcile(context.Background(), req)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}

			nextEvent, _, err := logic.GetNextEvent(
				time.Now().UTC(), // Format times in UTC
				tt.sleepOrder.Spec.WakeAt,
				tt.sleepOrder.Spec.SleepAt,
				tt.sleepOrder.Spec.Timezone,
			)
			if err != nil {
				t.Fatalf("GetNextEvent failed: %v", err)
			}

			// Check target object replicas
			switch target := tt.targetObj.(type) {
			case *appsv1.Deployment:
				updatedDeployment := &appsv1.Deployment{}
				err = cl.Get(context.Background(), client.ObjectKeyFromObject(target), updatedDeployment)
				if err != nil {
					t.Fatalf("failed to get updated deployment: %v", err)
				}
				if *updatedDeployment.Spec.Replicas != tt.expectedReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectedReplicas, *updatedDeployment.Spec.Replicas)
				}
			case *appsv1.StatefulSet:
				updatedStatefulSet := &appsv1.StatefulSet{}
				err = cl.Get(context.Background(), client.ObjectKeyFromObject(target), updatedStatefulSet)
				if err != nil {
					t.Fatalf("failed to get updated statefulset: %v", err)
				}
				if *updatedStatefulSet.Spec.Replicas != tt.expectedReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectedReplicas, *updatedStatefulSet.Spec.Replicas)
				}
			}

			// Check that the RequeueAfter is close to the next event +- 1 second
			diff := result.RequeueAfter - time.Until(nextEvent)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("expected RequeueAfter '%v', got '%v'", time.Until(nextEvent), result.RequeueAfter)
			}

			// Check sleepOrder status is sleeping
			updatedSleepOrder := &sleepodv1alpha1.SleepOrder{}
			err = cl.Get(context.Background(), client.ObjectKeyFromObject(tt.sleepOrder), updatedSleepOrder)
			if err != nil {
				t.Fatalf("failed to get updated sleeporder: %v", err)
			}
			if updatedSleepOrder.Status.CurrentState != sleepingState {
				t.Errorf("expected CurrentState '%s', got '%v'", sleepingState, updatedSleepOrder.Status.CurrentState)
			}
		})
	}
}

func TestReconcile_wakeFlow(t *testing.T) {
	// TODO: implement
}
