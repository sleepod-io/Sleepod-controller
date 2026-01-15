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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/config"
)

// SleepPolicyReconciler reconciles a SleepPolicy object
type SleepPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeppolicies/finalizers,verbs=update

func (r *SleepPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sleepPolicyObj, err := FetchSleepPolicyOrContinue(ctx, r.Client, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if sleepPolicyObj == nil {
		return ctrl.Result{}, nil
	}
	resourceNeedToBeUpdate, err := r.checkAndBuildValidResource(ctx, sleepPolicyObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	if resourceNeedToBeUpdate {
		if err := r.Update(ctx, sleepPolicyObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	log.Info("SleepPolicy is valid")

	if sleepPolicyObj.DeletionTimestamp != nil {
		// TODO: remove the policy from the namespace (validate that the sleepOrders resources are deleted)
		log.Info("SleepPolicy is being deleted", "name", sleepPolicyObj.Name)
		controllerutil.RemoveFinalizer(sleepPolicyObj, sleepPolicyFinalizer)
		if err := r.Update(ctx, sleepPolicyObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// check if the finalizer is present
	if !controllerutil.ContainsFinalizer(sleepPolicyObj, sleepPolicyFinalizer) {
		controllerutil.AddFinalizer(sleepPolicyObj, sleepPolicyFinalizer)
		if err := r.Update(ctx, sleepPolicyObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// build the desired state.
	desiredState, err := r.buildTheDesiredState(ctx, sleepPolicyObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.deleteUndesiredResources(ctx, req.Namespace, desiredState)
	if err != nil {
		return ctrl.Result{}, err
	}

	// apply the desired state.
	for _, params := range desiredState {
		needToDeploySleepOrder, action, err := r.needToDeploySleepOrder(ctx, sleepPolicyObj.Name, params)
		if err != nil {
			return ctrl.Result{}, err
		}
		if needToDeploySleepOrder {
			err := r.DeploySleepOrderResource(ctx, sleepPolicyObj, params, action)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// update the status of the SleepPolicy resource if needed.
	if !reflect.DeepEqual(sleepPolicyObj.Status.State, desiredState) {
		sleepPolicyObj.Status.State = desiredState
		if err := r.Status().Update(ctx, sleepPolicyObj); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SleepPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sleepodv1alpha1.SleepPolicy{}).
		Named("sleeppolicy").
		Complete(r)
}

func FetchSleepPolicyOrContinue(ctx context.Context, r client.Client, req ctrl.Request) (*sleepodv1alpha1.SleepPolicy, error) {
	SleepPolicyObj := &sleepodv1alpha1.SleepPolicy{}
	err := r.Get(ctx, req.NamespacedName, SleepPolicyObj)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return SleepPolicyObj, nil
}
