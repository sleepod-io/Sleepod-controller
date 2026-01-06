package controller

import (
	"context"
	"time"

	sleepodv1alpha1 "github.com/shaygef123/SleePod-controller/api/v1alpha1"
	"github.com/shaygef123/SleePod-controller/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=sleepod.sleepod.io,resources=sleeppolicies,verbs=get;list;watch;create
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Namespace", "name", req.Name)

	// fetching namespace
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace not found, it must have been deleted
			log.V(1).Info("Namespace not found, nothing to reconcile")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch Namespace")
		return ctrl.Result{}, err
	}

	namespaceDelay := r.Config.GetNamespaceDelay()
	age := time.Since(namespace.CreationTimestamp.Time)
	if age < namespaceDelay {
		log.V(1).Info("Namespace age is less then the delay time", "age", age)
		return ctrl.Result{RequeueAfter: namespaceDelay - age}, nil
	}

	// check if namespace is in excluded list
	if r.Config.IsNamespaceExcluded(namespace.Name) {
		log.V(1).Info("Namespace is in excluded list, nothing to reconcile")
		return ctrl.Result{}, nil
	}

	// check if sleepPolicy already exists:
	sleepPolicyList := &sleepodv1alpha1.SleepPolicyList{}
	err = r.List(ctx, sleepPolicyList, &client.ListOptions{Namespace: namespace.Name})
	if err != nil {
		log.Error(err, "Failed to list SleepPolicy resources, requeueing", "namespace", namespace.Name)
		return ctrl.Result{RequeueAfter: namespaceDelay}, nil
	}
	if len(sleepPolicyList.Items) > 0 {
		log.V(1).Info("SleepPolicy already exists", "namespace", namespace.Name)
		return ctrl.Result{}, nil
	}
	log.Info("SleepPolicy not found, creating default sleepPolicy", "namespace", namespace.Name)
	// create default sleepPolicy
	err = r.createDefaultSleepPolicy(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to create default SleepPolicy", "namespace", namespace.Name)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Default SleepPolicy created", "namespace", namespace.Name)
	return ctrl.Result{}, nil
}

func (r *NamespaceReconciler) createDefaultSleepPolicy(ctx context.Context, namespace *corev1.Namespace) error {
	defaultSleepPolicy := sleepodv1alpha1.SleepPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      DefaultSleepPolicyName,
		},
		Spec: sleepodv1alpha1.SleepPolicySpec{
			Deployments: map[string]sleepodv1alpha1.PolicyConfig{
				"default": {
					Enable: true,
				},
			},
			StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
				"default": {
					Enable: true,
				},
			},
		},
	}
	return r.Create(ctx, &defaultSleepPolicy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
