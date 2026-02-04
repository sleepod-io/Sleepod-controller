package controller

import (
	"context"
	"time"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/config"
	"github.com/sleepod-io/sleepod-controller/internal/logic"
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

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
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

	// check if namespace is excluded from config or from annotations
	// check if sleepPolicy resources exists in the exclude ns and if yes delete them
	if r.Config.IsNamespaceExcludedFromConfig(namespace.Name) || r.isNamespaceExcludedFromAnnotations(namespace) {
		log.Info("Namespace is in excluded list, skipping reconciliation", "namespace", namespace.Name)
		sleepPolicyList := &sleepodv1alpha1.SleepPolicyList{}
		err = r.List(ctx, sleepPolicyList, &client.ListOptions{Namespace: namespace.Name})
		if err == nil {
			if len(sleepPolicyList.Items) > 0 {
				log.V(1).Info("SleepPolicy exists in excluded namespace, deleting", "namespace", namespace.Name)
				for _, sleepPolicy := range sleepPolicyList.Items {
					err = r.Delete(ctx, &sleepPolicy)
					if err != nil {
						log.Error(err, "Failed to delete SleepPolicy", "namespace", namespace.Name)
						return ctrl.Result{RequeueAfter: namespaceDelay}, nil
					}
				}
			}
		}
		return ctrl.Result{}, nil
	}

	// check if expiration date of the namespace is exists
	if r.Config.NamespaceConfig.TTLEnabled {
		if expirationDateStr, ok := namespace.Annotations[namespaceExpirationDate]; ok {
			requeueDuration, err := r.calculateRequeueDuration(expirationDateStr, r.Config.DefaultTimezone)
			if err != nil {
				log.Error(err, "Failed to calculate requeue duration", "namespace", namespace.Name)
				return ctrl.Result{}, nil
			}
			if requeueDuration > 0 {
				log.V(1).Info("Namespace expiration date is not today, requeueing", "namespace", namespace.Name, "requeueDuration", requeueDuration)
				return ctrl.Result{RequeueAfter: requeueDuration}, nil
			}
			log.V(1).Info("Namespace expiration date is today, deleting", "namespace", namespace.Name)
			err = r.Delete(ctx, namespace)
			if err != nil {
				log.Error(err, "Failed to delete namespace", "namespace", namespace.Name)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, nil
		}
		log.V(1).Info("Namespace expiration date is not exists, adding annotation", "namespace", namespace.Name)
		tz, err := time.LoadLocation(r.Config.DefaultTimezone)
		if err != nil {
			log.Error(err, "Failed to load timezone", "timezone", r.Config.DefaultTimezone)
			return ctrl.Result{}, nil
		}
		namespaceExpirationDateStr := time.Now().In(tz).AddDate(0, 0, r.Config.NamespaceConfig.ExpirationTTLInDays).Format("02/01/2006")
		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations[namespaceExpirationDate] = namespaceExpirationDateStr
		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to update namespace", "namespace", namespace.Name)
			return ctrl.Result{}, nil
		}
	} else {
		log.V(1).Info("Namespace TTL feature is disabled", "namespace", namespace.Name)
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

func (r *NamespaceReconciler) calculateRequeueDuration(expirationDate string, timezone string) (time.Duration, error) {
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, err
	}
	date, err := logic.ParseDateFromStr(expirationDate)
	if err != nil {
		return 0, err
	}
	today := time.Now().In(tz)
	if today.Before(date) {
		nextRequeue := date.Sub(today)
		return nextRequeue, nil
	}
	return 0, nil
}

func (r *NamespaceReconciler) isNamespaceExcludedFromAnnotations(namespace *corev1.Namespace) bool {
	if namespace.Annotations == nil {
		return false
	}
	if _, ok := namespace.Annotations[namespaceExcludeAnnotation]; ok {
		return true
	}
	return false
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
					Enable: r.Config.DefaultPolicyEnabled,
				},
			},
			StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
				"default": {
					Enable: r.Config.DefaultPolicyEnabled,
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
