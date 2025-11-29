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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sleepodv1alpha1 "github.com/shaygef123/SleePod-controller/api/v1alpha1"
	"github.com/shaygef123/SleePod-controller/internal/logic"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SleepOrderReconciler reconciles a SleepOrder object
type SleepOrderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	annotationKey string = "sleepod.io/original-replicas"
	sleepingState string = "Sleeping"
	awakeState    string = "Awake"
)

// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeporders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeporders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeporders/finalizers,verbs=update

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
	annotations[annotationKey] = strconv.Itoa(int(replicas))
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
		return 0, fmt.Errorf("annotation %s not found", annotationKey)
	}
	replicas, err := strconv.Atoi(annotations[annotationKey])
	if err != nil {
		return 0, err
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
	delete(target.GetAnnotations(), annotationKey)
	target.SetAnnotations(target.GetAnnotations())

	return replicasInt32, r.Update(ctx, target)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SleepOrder object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *SleepOrderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// TODO: add logs.
	// TODO: refactor the reconcile function.
	log := logf.FromContext(ctx)

	// fetch resources
	SleepOrderObj, err := FetchSleepOrderOrContinue(ctx, r, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if SleepOrderObj == nil {
		return ctrl.Result{}, nil
	}
	if SleepOrderObj.Status.LastTransitionTime != nil {
		diffBetweenLastTransitionTimeAndNow := time.Since(SleepOrderObj.Status.LastTransitionTime.Time)
		if diffBetweenLastTransitionTimeAndNow < 30*time.Second {
			return ctrl.Result{}, nil
		}
	}
	log.Info("SleepOrder fetched", "SleepOrder", SleepOrderObj)

	sleepOrderSpec := SleepOrderObj.Spec
	objectMeta := metav1.ObjectMeta{
		Name: sleepOrderSpec.TargetRef.Name,
	}
	var targetObj client.Object
	switch sleepOrderSpec.TargetRef.Kind {
	case "Deployment":
		deployment := &appsv1.Deployment{
			ObjectMeta: objectMeta,
		}
		err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: sleepOrderSpec.TargetRef.Name}, deployment)
		if err != nil {
			return ctrl.Result{}, err
		}
		targetObj = deployment
	case "StatefulSet":
		statefulSet := &appsv1.StatefulSet{
			ObjectMeta: objectMeta,
		}
		err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: sleepOrderSpec.TargetRef.Name}, statefulSet)
		if err != nil {
			return ctrl.Result{}, err
		}
		targetObj = statefulSet
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported resource type: %s", SleepOrderObj.Spec.TargetRef.Kind)
	}
	if targetObj == nil {
		return ctrl.Result{}, fmt.Errorf("target object not found")
	}

	// calculate Time State
	shouldSleep, err := logic.ShouldBeAsleep(time.Now(), sleepOrderSpec.WakeAt, sleepOrderSpec.SleepAt, sleepOrderSpec.Timezone)
	if err != nil {
		return ctrl.Result{}, err
	}
	nextEvent, _, err := logic.GetNextEvent(time.Now(), sleepOrderSpec.WakeAt, sleepOrderSpec.SleepAt, sleepOrderSpec.Timezone)
	if err != nil {
		return ctrl.Result{}, err
	}
	if shouldSleep {
		originalReplicas, err := r.snapshotReplicas(ctx, targetObj)
		log.Info("reconcile sleep", "replicas", originalReplicas)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = setReplicas(targetObj, 0)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Update(ctx, targetObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		// TODO: make the currentState enum of SleepOrderStatus.
		SleepOrderObj.Status.CurrentState = sleepingState
		if SleepOrderObj.Status.LastTransitionTime == nil {
			SleepOrderObj.Status.LastTransitionTime = &metav1.Time{Time: time.Now()}
		}
		SleepOrderObj.Status.NextOperationTime = &metav1.Time{Time: nextEvent}
		SleepOrderObj.Status.OriginalReplicas = &originalReplicas
		err = r.Status().Update(ctx, SleepOrderObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Until(nextEvent)}, nil
	} else {
		currentReplicas, err := getReplicas(targetObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		if currentReplicas == 0 {
			currentReplicas, err = r.restoreReplicas(ctx, targetObj)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = setReplicas(targetObj, currentReplicas)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.Update(ctx, targetObj)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		SleepOrderObj.Status.CurrentState = awakeState
		if SleepOrderObj.Status.LastTransitionTime == nil {
			SleepOrderObj.Status.LastTransitionTime = &metav1.Time{Time: time.Now()}
		}
		SleepOrderObj.Status.NextOperationTime = &metav1.Time{Time: nextEvent}
		SleepOrderObj.Status.OriginalReplicas = &currentReplicas
		err = r.Status().Update(ctx, SleepOrderObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Until(nextEvent)}, nil
	}
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
