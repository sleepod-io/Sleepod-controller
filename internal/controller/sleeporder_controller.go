/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/logic"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SleepOrderReconciler reconciles a SleepOrder object
type SleepOrderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeporders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeporders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeporders/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;update;patch

// snapshotReplicas saves the current replica count to an annotation on the target object.
func (r *SleepOrderReconciler) snapshotReplicas(ctx context.Context, target client.Object) (int32, error) {
	// Get replicas from target
	replicas, err := getReplicas(target)
	if err != nil {
		return 0, err
	}

	err = r.Get(ctx, client.ObjectKeyFromObject(target), target)
	if err != nil {
		return 0, err
	}
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[originalReplicasAnnotationKey] = strconv.Itoa(int(replicas))
	target.SetAnnotations(annotations)
	return replicas, r.Update(ctx, target)
}

// restoreReplicas restores the replica count from an annotation on the target object.
func (r *SleepOrderReconciler) restoreReplicas(ctx context.Context, target client.Object) (int32, error) {
	// Get replicas from annotation
	err := r.Get(ctx, client.ObjectKeyFromObject(target), target)
	if err != nil {
		return 0, err
	}
	annotations := target.GetAnnotations()
	if annotations == nil {
		return 0, fmt.Errorf("annotation %s not found", originalReplicasAnnotationKey)
	}
	val, ok := annotations[originalReplicasAnnotationKey]
	if !ok || val == "" {
		// If annotation matches the key but is empty, or doesn't exist, we return 0 to avoid parsing error
		// and allow the process to continue (e.g. for finalizer removal).
		return 0, nil
	}
	replicas, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("failed to parse annotation %s: %w", originalReplicasAnnotationKey, err)
	}
	replicasInt32 := int32(replicas)
	// Update replicas
	switch t := target.(type) {
	case *appsv1.Deployment:
		t.Spec.Replicas = &replicasInt32
	case *appsv1.StatefulSet:
		t.Spec.Replicas = &replicasInt32
	default:
		return 0, fmt.Errorf("unsupported resource type: %T", target)
	}
	// Remove annotation
	delete(target.GetAnnotations(), originalReplicasAnnotationKey)
	target.SetAnnotations(target.GetAnnotations())

	return replicasInt32, r.Update(ctx, target)
}

func (r *SleepOrderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// TODO: add logs (?).
	log := logf.FromContext(ctx)

	// fetch resources
	SleepOrderObj, err := FetchSleepOrderOrContinue(ctx, r, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if SleepOrderObj == nil {
		return ctrl.Result{}, nil
	}

	log.V(1).Info("SleepOrder fetched", "SleepOrder", SleepOrderObj)

	sleepOrderSpec := SleepOrderObj.Spec
	targetObj, err := r.getTargetObject(ctx, req.Namespace, sleepOrderSpec.TargetRef)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Target object not found", "target", sleepOrderSpec.TargetRef.Name)
			// Retry in 1 minute to see if the target appears
			nextRetry := time.Now().Add(time.Minute)
			// remove the finalizer
			controllerutil.RemoveFinalizer(SleepOrderObj, sleepOrderFinalizer)
			if err := r.Update(ctx, SleepOrderObj); err != nil {
				return ctrl.Result{RequeueAfter: time.Minute}, err
			}
			return r.updateStatus(ctx, SleepOrderObj, "Error TargetNotFound", nextRetry, nil)
		}
		return ctrl.Result{}, err
	}

	// calculate Time State
	shouldSleep, nextEvent, _, err := logic.GetTimeState(time.Now(), sleepOrderSpec.WakeAt, sleepOrderSpec.SleepAt, sleepOrderSpec.Timezone)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Calculated time state", "shouldSleep", shouldSleep, "nextEvent", nextEvent)

	currentReplicas, err := getReplicas(targetObj)
	if err != nil {
		return ctrl.Result{}, err
	}

	if done, err := r.handleFinalizer(ctx, SleepOrderObj, targetObj, currentReplicas); done {
		return ctrl.Result{}, err
	}

	if shouldSleep {
		return r.handleSleep(ctx, SleepOrderObj, targetObj, nextEvent)
	}
	return r.handleWake(ctx, SleepOrderObj, targetObj, currentReplicas, nextEvent)

}

func (r *SleepOrderReconciler) getTargetObject(ctx context.Context, namespace string, targetRef sleepodv1alpha1.TargetRef) (client.Object, error) {
	objectMeta := metav1.ObjectMeta{
		Name: targetRef.Name,
	}
	var targetObj client.Object
	switch targetRef.Kind {
	case kindDeployment:
		targetObj = &appsv1.Deployment{ObjectMeta: objectMeta}
	case kindStatefulSet:
		targetObj = &appsv1.StatefulSet{ObjectMeta: objectMeta}
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", targetRef.Kind)
	}

	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: targetRef.Name}, targetObj)
	if err != nil {
		return nil, err
	}
	return targetObj, nil
}

func (r *SleepOrderReconciler) handleFinalizer(ctx context.Context, sleepOrder *sleepodv1alpha1.SleepOrder, targetObj client.Object, currentReplicas int32) (bool, error) {
	var shouldStop bool
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the latest version of SleepOrder to avoid conflicts
		if err := r.Get(ctx, client.ObjectKeyFromObject(sleepOrder), sleepOrder); err != nil {
			if errors.IsNotFound(err) {
				shouldStop = true
				return nil
			}
			return err
		}

		if sleepOrder.DeletionTimestamp.IsZero() {
			// The object is not being deleted.
			if !controllerutil.ContainsFinalizer(sleepOrder, sleepOrderFinalizer) {
				controllerutil.AddFinalizer(sleepOrder, sleepOrderFinalizer)
				if err := r.Update(ctx, sleepOrder); err != nil {
					return client.IgnoreNotFound(err)
				}
				// Continue to logic as original behavior
				shouldStop = false
				return nil
			}
			shouldStop = false
			return nil
		}

		// The object is being deleted
		if controllerutil.ContainsFinalizer(sleepOrder, sleepOrderFinalizer) {
			if currentReplicas == 0 {
				replicasFromAnnotation, err := r.restoreReplicas(ctx, targetObj)
				if err != nil {
					return err
				}
				err = setReplicas(targetObj, replicasFromAnnotation)
				if err != nil {
					return err
				}
				err = r.Update(ctx, targetObj)
				if err != nil {
					return client.IgnoreNotFound(err)
				}
			}

			// remove finalizer and update it.
			controllerutil.RemoveFinalizer(sleepOrder, sleepOrderFinalizer)
			if err := r.Update(ctx, sleepOrder); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		shouldStop = true
		return nil
	})
	return shouldStop, err
}

func (r *SleepOrderReconciler) handleSleep(ctx context.Context, sleepOrder *sleepodv1alpha1.SleepOrder, targetObj client.Object, nextEvent time.Time) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var originalReplicas int32
	annotations := targetObj.GetAnnotations()
	if val, ok := annotations[originalReplicasAnnotationKey]; ok {
		// if annotation exists, use it
		valInt, err := strconv.Atoi(val)
		if err != nil {
			log.Error(err, "Failed to parse annotation")
			return ctrl.Result{}, err
		}
		originalReplicas = int32(valInt)
	} else {
		// if annotation does not exist, snapshot
		var err error
		originalReplicas, err = r.snapshotReplicas(ctx, targetObj)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("reconcile sleep: scaling down", "target", targetObj.GetName(), "originalReplicas", originalReplicas)

	err := setReplicas(targetObj, 0)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.Update(ctx, targetObj)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.updateStatus(ctx, sleepOrder, sleepingState, nextEvent, &originalReplicas)
}

func (r *SleepOrderReconciler) handleWake(ctx context.Context, sleepOrder *sleepodv1alpha1.SleepOrder, targetObj client.Object, currentReplicas int32, nextEvent time.Time) (ctrl.Result, error) {
	if currentReplicas == 0 {
		log := logf.FromContext(ctx)
		restoredReplicas, err := r.restoreReplicas(ctx, targetObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		log.Info("reconcile wake: scaling up", "target", targetObj.GetName(), "restoredReplicas", restoredReplicas)
		err = setReplicas(targetObj, restoredReplicas)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Update(ctx, targetObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		currentReplicas = restoredReplicas
	}
	return r.updateStatus(ctx, sleepOrder, awakeState, nextEvent, &currentReplicas)
}

func (r *SleepOrderReconciler) updateStatus(ctx context.Context, sleepOrder *sleepodv1alpha1.SleepOrder, state string, nextEvent time.Time, originalReplicas *int32) (ctrl.Result, error) {
	// Refetch the latest version of the object to avoid conflicts
	latestSleepOrder := &sleepodv1alpha1.SleepOrder{}
	err := r.Get(ctx, client.ObjectKeyFromObject(sleepOrder), latestSleepOrder)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Prepare new values
	newStatus := latestSleepOrder.Status.DeepCopy()
	newStatus.CurrentState = state
	newStatus.NextOperationTime = &metav1.Time{Time: nextEvent}
	newStatus.OriginalReplicas = originalReplicas

	// Only update LastTransitionTime if the state has changed
	if latestSleepOrder.Status.CurrentState != state {
		newStatus.LastTransitionTime = &metav1.Time{Time: time.Now()}
	}

	// Check if status actually changed (avoid flapping)
	if latestSleepOrder.Status.CurrentState == newStatus.CurrentState &&
		equalTimePtr(latestSleepOrder.Status.NextOperationTime, newStatus.NextOperationTime) &&
		equalInt32Ptr(latestSleepOrder.Status.OriginalReplicas, newStatus.OriginalReplicas) {
		// No change needed
		return ctrl.Result{RequeueAfter: time.Until(nextEvent)}, nil
	}

	latestSleepOrder.Status = *newStatus
	err = r.Status().Update(ctx, latestSleepOrder)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Until(nextEvent)}, nil
}

func equalTimePtr(t1, t2 *metav1.Time) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.Equal(t2)
}

func equalInt32Ptr(i1, i2 *int32) bool {
	if i1 == nil && i2 == nil {
		return true
	}
	if i1 == nil || i2 == nil {
		return false
	}
	return *i1 == *i2
}

// SetupWithManager sets up the controller with the Manager.
func (r *SleepOrderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sleepodv1alpha1.SleepOrder{}).
		Named("sleeporder").
		Complete(r)
}

func getReplicas(target client.Object) (int32, error) {
	switch t := target.(type) {
	case *appsv1.Deployment:
		if t.Spec.Replicas != nil {
			return *t.Spec.Replicas, nil
		}
		return 1, nil
	case *appsv1.StatefulSet:
		if t.Spec.Replicas != nil {
			return *t.Spec.Replicas, nil
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported resource type: %T", target)
	}
}

func setReplicas(target client.Object, replicas int32) error {
	switch t := target.(type) {
	case *appsv1.Deployment:
		t.Spec.Replicas = &replicas
	case *appsv1.StatefulSet:
		t.Spec.Replicas = &replicas
	default:
		return fmt.Errorf("unsupported resource type: %T", target)
	}
	return nil
}

func FetchSleepOrderOrContinue(ctx context.Context, r *SleepOrderReconciler, req ctrl.Request) (*sleepodv1alpha1.SleepOrder, error) {
	sleepOrder := &sleepodv1alpha1.SleepOrder{}
	err := r.Get(ctx, req.NamespacedName, sleepOrder)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, it must have been deleted.
			return nil, nil
		}
		return nil, err
	}
	return sleepOrder, nil
}
