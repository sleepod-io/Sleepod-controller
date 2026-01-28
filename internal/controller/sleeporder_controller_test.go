package controller

import (
	"context"
	"strconv"
	"testing"
	"time"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/logic"
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
	replicas := int32(5)

	tests := []struct {
		name           string
		object         client.Object
		expectReplicas int32
	}{
		{
			name:           "Test deployment with replicas 5",
			expectReplicas: replicas,
			object: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
			},
		},
		{
			name:           "Test statefulset with replicas 5",
			expectReplicas: replicas,
			object: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &replicas,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Fake Client with this object pre-loaded
			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.object).
				Build()

			// Create the Reconciler with the fake client
			r := &SleepOrderReconciler{
				Client: cl,
				Scheme: scheme,
			}

			ctx := context.Background()
			currentReplicas, err := r.snapshotReplicas(ctx, tt.object)
			if err != nil {
				t.Fatalf("snapshotReplicas failed: %v", err)
			}
			// Verify that the annotation was added
			switch tt.object.(type) {
			case *appsv1.Deployment:
				updatedDeployment := tt.object.(*appsv1.Deployment)
				err = cl.Get(ctx, client.ObjectKeyFromObject(tt.object), updatedDeployment)
				if err != nil {
					t.Fatalf("failed to get updated deployment: %v", err)
				}
				val, ok := updatedDeployment.Annotations[originalReplicasAnnotationKey]
				if !ok {
					t.Error("annotation " + originalReplicasAnnotationKey + " not found")
				}
				if val != strconv.Itoa(int(tt.expectReplicas)) {
					t.Errorf("expected annotation '%d', got '%s'", tt.expectReplicas, val)
				}
				if currentReplicas != tt.expectReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectReplicas, currentReplicas)
				}
			case *appsv1.StatefulSet:
				updatedStatefulSet := tt.object.(*appsv1.StatefulSet)
				err = cl.Get(ctx, client.ObjectKeyFromObject(tt.object), updatedStatefulSet)
				if err != nil {
					t.Fatalf("failed to get updated statefulset: %v", err)
				}
				val, ok := updatedStatefulSet.Annotations[originalReplicasAnnotationKey]
				if !ok {
					t.Error("annotation " + originalReplicasAnnotationKey + " not found")
				}
				if val != strconv.Itoa(int(tt.expectReplicas)) {
					t.Errorf("expected annotation '%d', got '%s'", tt.expectReplicas, val)
				}
				if currentReplicas != tt.expectReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectReplicas, currentReplicas)
				}
			default:
				t.Fatalf("unknown resource type: %T", tt.object)
			}
		})
	}
}

func TestRestoreReplicas(t *testing.T) {
	// Setup Scheme (to know about Deployment types)
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	replicas := int32(0)

	tests := []struct {
		name           string
		object         client.Object
		expectReplicas int32
	}{
		{
			name: "Test deployment with replicas 0",
			// TODO: make replicas 5 as a global value.
			expectReplicas: 5,
			object: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Annotations: map[string]string{
						originalReplicasAnnotationKey: "5",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
			},
		},
		{
			name:           "Test statefulset with replicas 0",
			expectReplicas: 5,
			object: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "default",
					Annotations: map[string]string{
						originalReplicasAnnotationKey: "5",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &replicas,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Fake Client with this object pre-loaded
			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.object).
				Build()

			// Create the Reconciler with the fake client
			r := &SleepOrderReconciler{
				Client: cl,
				Scheme: scheme,
			}
			ctx := context.Background()

			originalReplicas, err := r.restoreReplicas(ctx, tt.object)
			if err != nil {
				t.Fatalf("restoreReplicas failed: %v", err)
			}
			// Verify that the replicas were restored
			switch tt.object.(type) {
			case *appsv1.Deployment:
				updatedDeployment := tt.object.(*appsv1.Deployment)
				err = cl.Get(ctx, client.ObjectKeyFromObject(tt.object), updatedDeployment)
				if err != nil {
					t.Fatalf("failed to get updated deployment: %v", err)
				}
				if tt.expectReplicas != originalReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectReplicas, originalReplicas)
				}
				// Verify that the annotation was removed
				if _, ok := updatedDeployment.Annotations[originalReplicasAnnotationKey]; ok {
					t.Errorf("annotation '%s' should have been removed", originalReplicasAnnotationKey)
				}
			case *appsv1.StatefulSet:
				updatedStatefulSet := tt.object.(*appsv1.StatefulSet)
				err = cl.Get(ctx, client.ObjectKeyFromObject(tt.object), updatedStatefulSet)
				if err != nil {
					t.Fatalf("failed to get updated statefulset: %v", err)
				}
				if tt.expectReplicas != originalReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectReplicas, originalReplicas)
				}
				// Verify that the annotation was removed
				if _, ok := updatedStatefulSet.Annotations[originalReplicasAnnotationKey]; ok {
					t.Errorf("annotation '%s' should have been removed", originalReplicasAnnotationKey)
				}
			default:
				t.Fatalf("unknown resource type: %T", tt.object)
			}
		})
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
		expectedReplicas int32
	}{
		{
			name: "Deployment currently awake with 3 replicas",
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
			expectedReplicas: 0,
		},
		{
			name: "StatefulSet currently awake with 3 replicas",
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
			expectedReplicas: 0,
		},
		{
			name: "Deployment currently sleep",
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
			expectedReplicas: 0,
		},
		{
			name: "StatefulSet currently sleep",
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
				"",
			)
			if err != nil {
				t.Fatalf("GetNextEvent failed: %v", err)
			}

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

			// Validate that the RequeueAfter is close to the nextEvent +- 1 second
			diff := result.RequeueAfter - time.Until(nextEvent)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("expected RequeueAfter '%v', got '%v'", time.Until(nextEvent), result.RequeueAfter)
			}

			// Validate sleepOrder status
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
	// Setup Scheme (to know about Deployment types)
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = sleepodv1alpha1.AddToScheme(scheme)

	wakeAt := time.Now().UTC().Add(-1 * time.Hour).Format("15:04")
	sleepTime := time.Now().UTC().Add(time.Hour)
	sleepAt := sleepTime.Format("15:04")

	tests := []struct {
		name                              string
		targetObj                         client.Object
		sleepOrder                        *sleepodv1alpha1.SleepOrder
		expectedReplicas                  int32
		expectedStatusOfCurrentState      string
		expectedStatusOfNextOperationTime *metav1.Time
	}{
		{
			name: "Deployment currently awake with 3 replicas",
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
					Timezone: defaultTimezone,
				},
			},
			expectedReplicas:                  3,
			expectedStatusOfCurrentState:      awakeState,
			expectedStatusOfNextOperationTime: &metav1.Time{Time: sleepTime},
		},
		{
			name: "StatefulSet currently awake with 3 replicas",
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
					Timezone: defaultTimezone,
				},
			},
			expectedReplicas:                  3,
			expectedStatusOfCurrentState:      awakeState,
			expectedStatusOfNextOperationTime: &metav1.Time{Time: sleepTime},
		},
		{
			name: "Deployment currently sleep",
			targetObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-0",
					Namespace: "default",
					Annotations: map[string]string{
						originalReplicasAnnotationKey: "5",
					},
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
					Timezone: defaultTimezone,
				},
			},
			expectedReplicas:                  5,
			expectedStatusOfCurrentState:      awakeState,
			expectedStatusOfNextOperationTime: &metav1.Time{Time: sleepTime},
		},
		{
			name: "StatefulSet currently sleep",
			targetObj: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset-0",
					Namespace: "default",
					Annotations: map[string]string{
						originalReplicasAnnotationKey: "5",
					},
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
					Timezone: defaultTimezone,
				},
			},
			expectedReplicas:                  5,
			expectedStatusOfCurrentState:      awakeState,
			expectedStatusOfNextOperationTime: &metav1.Time{Time: sleepTime},
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
				"",
			)
			if err != nil {
				t.Fatalf("GetNextEvent failed: %v", err)
			}

			var objectReplicas *int32
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
				objectReplicas = updatedDeployment.Spec.Replicas
			case *appsv1.StatefulSet:
				updatedStatefulSet := &appsv1.StatefulSet{}
				err = cl.Get(context.Background(), client.ObjectKeyFromObject(target), updatedStatefulSet)
				if err != nil {
					t.Fatalf("failed to get updated statefulset: %v", err)
				}
				if *updatedStatefulSet.Spec.Replicas != tt.expectedReplicas {
					t.Errorf("expected replicas '%d', got '%d'", tt.expectedReplicas, *updatedStatefulSet.Spec.Replicas)
				}
				objectReplicas = updatedStatefulSet.Spec.Replicas
			}

			// Validate that the RequeueAfter is close to the nextEvent +- 1 second
			// TODO: consider make this diff to separate func.
			diff := result.RequeueAfter - time.Until(nextEvent)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("expected RequeueAfter '%v', got '%v'", time.Until(nextEvent), result.RequeueAfter)
			}

			// Validate sleepOrder status
			updatedSleepOrder := &sleepodv1alpha1.SleepOrder{}
			err = cl.Get(context.Background(), client.ObjectKeyFromObject(tt.sleepOrder), updatedSleepOrder)
			if err != nil {
				t.Fatalf("failed to get updated sleeporder: %v", err)
			}
			if updatedSleepOrder.Status.CurrentState != tt.expectedStatusOfCurrentState {
				t.Errorf("expected CurrentState '%s', got '%v'", tt.expectedStatusOfCurrentState, updatedSleepOrder.Status.CurrentState)
			}
			diffBetweenNextOperationTimeAndExpected := updatedSleepOrder.Status.NextOperationTime.Sub(tt.expectedStatusOfNextOperationTime.Time)
			if diffBetweenNextOperationTimeAndExpected < -time.Minute || diffBetweenNextOperationTimeAndExpected > time.Minute {
				t.Errorf("expected NextOperationTime '%v', got '%v'", tt.expectedStatusOfNextOperationTime, updatedSleepOrder.Status.NextOperationTime)
			}
			if *objectReplicas != *updatedSleepOrder.Status.OriginalReplicas {
				t.Errorf("expected OriginalReplicas '%v', got '%v'", objectReplicas, updatedSleepOrder.Status.OriginalReplicas)
			}
		})
	}
}

func TestSleepOrderReconciler_Delete(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = sleepodv1alpha1.AddToScheme(scheme)

	hourBeforeNow := time.Now().UTC().Add(-1 * time.Hour).Format("15:04")
	hourAfterNow := time.Now().UTC().Add(time.Hour).Format("15:04")

	tests := []struct {
		name             string
		targetObj        client.Object
		sleepOrder       *sleepodv1alpha1.SleepOrder
		expectedReplicas int32
	}{
		{
			name: "Deleting sleepOrder of deployment that is in awake status",
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
					WakeAt:   hourBeforeNow,
					SleepAt:  hourAfterNow,
					Timezone: defaultTimezone,
				},
			},
			expectedReplicas: 3,
		},
		{
			name: "Deleting sleepOrder of statefulset that is in sleep status",
			targetObj: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset-3",
					Namespace: "default",
					Annotations: map[string]string{
						originalReplicasAnnotationKey: "3",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(0); return &i }(),
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
					WakeAt:   hourAfterNow,
					SleepAt:  hourBeforeNow,
					Timezone: defaultTimezone,
				},
			},
			expectedReplicas: 3,
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
			_, err := r.Reconcile(context.Background(), req)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}

			// Verify finalizer was added
			updatedSleepOrder := &sleepodv1alpha1.SleepOrder{}
			err = cl.Get(context.Background(), client.ObjectKeyFromObject(tt.sleepOrder), updatedSleepOrder)
			if err != nil {
				t.Fatalf("failed to get updated sleeporder: %v", err)
			}

			if err := cl.Delete(context.Background(), tt.sleepOrder); err != nil {
				t.Fatalf("failed to delete sleeporder: %v", err)
			}

			_, err = r.Reconcile(context.Background(), req)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}

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
				_, ok := updatedDeployment.Annotations[originalReplicasAnnotationKey]
				if ok {
					t.Errorf("annotation '%s' should have been removed", originalReplicasAnnotationKey)
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
				_, ok := updatedStatefulSet.Annotations[originalReplicasAnnotationKey]
				if ok {
					t.Errorf("annotation '%s' should have been removed", originalReplicasAnnotationKey)
				}
			}

		})
	}
}
