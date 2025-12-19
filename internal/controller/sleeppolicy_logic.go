package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	sleepodv1alpha1 "github.com/shaygef123/SleePod-controller/api/v1alpha1"
	"github.com/shaygef123/SleePod-controller/internal/config/utils"
)

// checkAndBuildValidResource validates the SleepPolicy spec and ensures mandatory defaults and cluster sync.
// It returns true if the policy was modified and needs to be updated.
func (r *SleepPolicyReconciler) checkAndBuildValidResource(ctx context.Context, policy *sleepodv1alpha1.SleepPolicy) bool {
	changed := false
	log := logf.FromContext(ctx)

	// Ensure Maps Exist
	if policy.Spec.Deployments == nil {
		policy.Spec.Deployments = make(map[string]sleepodv1alpha1.PolicyConfig)
	}
	if policy.Spec.StatefulSets == nil {
		policy.Spec.StatefulSets = make(map[string]sleepodv1alpha1.PolicyConfig)
	}

	// get the resources in the namespace
	var deploymentList appsv1.DeploymentList
	if err := r.List(ctx, &deploymentList, client.InNamespace(policy.Namespace)); err == nil {
		// Logic: If there are deployments in the cluster that are NOT in the policy,
		// need a add 'default' to cover them.
		needsDefault := false
		for _, dep := range deploymentList.Items {
			if _, ok := policy.Spec.Deployments[dep.Name]; !ok {
				needsDefault = true
				break
			}
		}
		if needsDefault {
			if _, ok := policy.Spec.Deployments["default"]; !ok {
				policy.Spec.Deployments["default"] = sleepodv1alpha1.PolicyConfig{Enable: true}
				changed = true
			}
		}
	} else {
		log.Error(err, "Failed to list Deployments for validation")
	}

	var stsList appsv1.StatefulSetList
	if err := r.List(ctx, &stsList, client.InNamespace(policy.Namespace)); err == nil {
		needsDefault := false
		for _, sts := range stsList.Items {
			if _, ok := policy.Spec.StatefulSets[sts.Name]; !ok {
				needsDefault = true
				break
			}
		}
		if needsDefault {
			if _, ok := policy.Spec.StatefulSets["default"]; !ok {
				policy.Spec.StatefulSets["default"] = sleepodv1alpha1.PolicyConfig{Enable: true}
				changed = true
			}
		}
	} else {
		log.Error(err, "Failed to list StatefulSets for validation")
	}

	// Always ensure 'default' exists.
	if len(policy.Spec.Deployments) == 0 {
		policy.Spec.Deployments["default"] = sleepodv1alpha1.PolicyConfig{Enable: true}
		changed = true
	}
	// Always Enforce Default is Enabled if present
	// TODO: consider add some "turn of" mechanism. maybe global enable, or allow default to be false?
	if def, ok := policy.Spec.Deployments["default"]; ok {
		if !def.Enable {
			def.Enable = true
			policy.Spec.Deployments["default"] = def
			changed = true
		}
	}

	if len(policy.Spec.StatefulSets) == 0 {
		policy.Spec.StatefulSets["default"] = sleepodv1alpha1.PolicyConfig{Enable: true}
		changed = true
	}
	if def, ok := policy.Spec.StatefulSets["default"]; ok {
		if !def.Enable {
			def.Enable = true
			policy.Spec.StatefulSets["default"] = def
			changed = true
		}
	}

	// Check if resources in Policy actually exist in Cluster
	for depName := range policy.Spec.Deployments {
		if depName == "default" {
			continue
		}
		// Check existence
		found := false
		for _, d := range deploymentList.Items {
			if d.Name == depName {
				found = true
				break
			}
		}
		if !found {
			// TODO: Handle this case, add this to the status.
			log.Info("Warning: Policy specifies Deployment that does not exist in cluster", "name", depName)
		}
	}

	for stsName := range policy.Spec.StatefulSets {
		if stsName == "default" {
			continue
		}
		found := false
		for _, s := range stsList.Items {
			if s.Name == stsName {
				found = true
				break
			}
		}
		if !found {
			log.Info("Warning: Policy specifies StatefulSet that does not exist in cluster", "name", stsName)
		}
	}

	return changed
}

// buildTheDesiredState calculates the effective sleep parameters for all managed resources in the namespace.
func (r *SleepPolicyReconciler) buildTheDesiredState(ctx context.Context, policy *sleepodv1alpha1.SleepPolicy) (map[string]sleepodv1alpha1.ResourceSleepParams, error) {
	desiredState := make(map[string]sleepodv1alpha1.ResourceSleepParams)

	// Process Deployments
	var deploymentList appsv1.DeploymentList
	if err := r.List(ctx, &deploymentList, client.InNamespace(policy.Namespace)); err != nil {
		return nil, err
	}
	for _, dep := range deploymentList.Items {
		if params := r.getEffectiveParams(policy.Namespace, dep.Name, "Deployment", policy.Spec.Deployments); params != nil {
			desiredState[dep.Name] = *params
		}
	}

	// Process StatefulSets
	var stsList appsv1.StatefulSetList
	if err := r.List(ctx, &stsList, client.InNamespace(policy.Namespace)); err != nil {
		return nil, err
	}
	for _, sts := range stsList.Items {
		if params := r.getEffectiveParams(policy.Namespace, sts.Name, "StatefulSet", policy.Spec.StatefulSets); params != nil {
			desiredState[sts.Name] = *params
		}
	}

	return desiredState, nil
}

func (r *SleepPolicyReconciler) getEffectiveParams(namespace, name, kind string, policyMap map[string]sleepodv1alpha1.PolicyConfig) *sleepodv1alpha1.ResourceSleepParams {
	// 1. Check for specific configuration
	if config, ok := policyMap[name]; ok {
		return r.resolveParams(namespace, name, kind, config)
	}
	// 2. Check for default configuration
	if config, ok := policyMap["default"]; ok {
		return r.resolveParams(namespace, name, kind, config)
	}
	return nil
}

func (r *SleepPolicyReconciler) resolveParams(namespace, name, kind string, policy sleepodv1alpha1.PolicyConfig) *sleepodv1alpha1.ResourceSleepParams {
	if !policy.Enable {
		return nil
	}

	wakeAt := policy.WakeAt
	if wakeAt == "" {
		wakeAt = r.Config.DefaultWakeAt
	}

	sleepAt := policy.SleepAt
	if sleepAt == "" {
		sleepAt = r.Config.DefaultSleepAt
	}

	timezone := policy.Timezone
	if timezone == "" {
		timezone = r.Config.DefaultTimezone
	}

	return &sleepodv1alpha1.ResourceSleepParams{
		Name:      name,
		Namespace: namespace,
		Kind:      kind,
		SleepAt:   sleepAt,
		WakeAt:    wakeAt,
		Timezone:  timezone,
	}
}

func (r *SleepPolicyReconciler) deleteUndesiredResources(ctx context.Context, namespace string, desiredState map[string]sleepodv1alpha1.ResourceSleepParams) error {
	log := logf.FromContext(ctx)
	var sleepOrderList sleepodv1alpha1.SleepOrderList
	if err := r.List(ctx, &sleepOrderList, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, sleepOrder := range sleepOrderList.Items {
		if _, ok := desiredState[sleepOrder.Spec.TargetRef.Name]; !ok {
			if err := r.Delete(ctx, &sleepOrder); err != nil {
				return err
			}
			log.Info("Deleted Undesired SleepOrder", "name", sleepOrder.Name)
		}
	}
	return nil
}

func (r *SleepPolicyReconciler) needToDeploySleepOrder(resourceDesiredState sleepodv1alpha1.ResourceSleepParams, sleepPolicy *sleepodv1alpha1.SleepPolicy) (bool, string) {
	// check if specific sleeporder exists.
	var sleepOrder sleepodv1alpha1.SleepOrder
	err := r.Get(context.Background(), client.ObjectKey{
		Namespace: resourceDesiredState.Namespace,
		Name:      resourceDesiredState.Namespace + "-" + resourceDesiredState.Name,
	}, &sleepOrder)
	if err == nil {
		// if yes, check if config hash is the same.
		if sleepOrder.Annotations[configHashAnnotationKey] != utils.ConfigHashCalc(resourceDesiredState) {
			return true, actionUpdate
		}
	} else {
		// otherwise, need to create.
		return true, actionCreate
	}
	return false, ""
}

func (r *SleepPolicyReconciler) DeploySleepOrderResource(ctx context.Context, policy *sleepodv1alpha1.SleepPolicy, resourceDesiredState sleepodv1alpha1.ResourceSleepParams, action string) error {
	log := logf.FromContext(ctx)
	sleepOrder := &sleepodv1alpha1.SleepOrder{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceDesiredState.Namespace + "-" + resourceDesiredState.Name,
			Namespace:   resourceDesiredState.Namespace,
			Annotations: make(map[string]string),
		},
		Spec: sleepodv1alpha1.SleepOrderSpec{
			TargetRef: sleepodv1alpha1.TargetRef{
				Kind: resourceDesiredState.Kind,
				Name: resourceDesiredState.Name,
			},
			WakeAt:   resourceDesiredState.WakeAt,
			SleepAt:  resourceDesiredState.SleepAt,
			Timezone: resourceDesiredState.Timezone,
		},
	}
	switch action {
	case actionCreate:
		sleepOrder.Annotations = map[string]string{
			configHashAnnotationKey: utils.ConfigHashCalc(resourceDesiredState),
		}
		if err := controllerutil.SetControllerReference(policy, sleepOrder, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, sleepOrder); err != nil {
			return err
		}
	case actionUpdate:
		// Fetch existing resource to get ResourceVersion
		current := &sleepodv1alpha1.SleepOrder{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(sleepOrder), current); err != nil {
			return err // Or handle not-found if needed, but 'update' implies existence
		}

		if current.Annotations != nil && current.Annotations[configHashAnnotationKey] == utils.ConfigHashCalc(resourceDesiredState) {
			log.Info("SleepOrder is up to date", "name", sleepOrder.Name)
			return nil
		}
		sleepOrder.ResourceVersion = current.ResourceVersion

		sleepOrder.Annotations[configHashAnnotationKey] = utils.ConfigHashCalc(resourceDesiredState)
		if err := controllerutil.SetControllerReference(policy, sleepOrder, r.Scheme); err != nil {
			return err
		}
		if err := r.Update(ctx, sleepOrder); err != nil {
			return err
		}
	default:
		return nil
	}
	return nil
}
