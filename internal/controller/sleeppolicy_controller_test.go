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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/config"
)

var _ = Describe("SleepPolicy Controller", func() {

	// Unit Tests for Logic
	Context("Logic: checkAndBuildValidResource", func() {
		var reconciler *SleepPolicyReconciler

		BeforeEach(func() {
			reconciler = &SleepPolicyReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				Config: &config.Config{
					DefaultWakeAt:   "08:00",
					DefaultSleepAt:  "20:00",
					DefaultTimezone: "UTC",
				},
			}
		})

		It("should add default deployment config if missing", func() {
			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments:  map[string]sleepodv1alpha1.PolicyConfig{},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{},
				},
			}

			changed, err := reconciler.checkAndBuildValidResource(context.Background(), policy)
			Expect(err).ToNot(HaveOccurred())

			Expect(changed).To(BeTrue())
			Expect(policy.Spec.Deployments).To(HaveKey("default"))
			Expect(policy.Spec.Deployments["default"].Enable).To(BeTrue())
		})

		It("should add default statefulset config if missing", func() {
			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{},
				},
			}

			changed, err := reconciler.checkAndBuildValidResource(context.Background(), policy)
			Expect(err).ToNot(HaveOccurred())

			Expect(changed).To(BeTrue())
			Expect(policy.Spec.StatefulSets).To(HaveKey("default"))
			Expect(policy.Spec.StatefulSets["default"].Enable).To(BeTrue())
		})

		It("should allow disabling the default config", func() {
			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: false},
					},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: false},
					},
				},
			}

			changed, err := reconciler.checkAndBuildValidResource(context.Background(), policy)
			Expect(err).ToNot(HaveOccurred())

			Expect(changed).To(BeFalse())
			Expect(policy.Spec.Deployments["default"].Enable).To(BeFalse())
			Expect(policy.Spec.StatefulSets["default"].Enable).To(BeFalse())
		})

		It("should return false if policy is already valid", func() {
			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
				},
			}

			changed, err := reconciler.checkAndBuildValidResource(context.Background(), policy)
			Expect(err).ToNot(HaveOccurred())

			Expect(changed).To(BeFalse())
		})

		It("should add default config if specific config exists but other resources exist in cluster", func() {
			// Scenario: Policy has specific config for 'app-a', but 'app-b' exists in cluster.
			// Logic should see 'app-b', realize it needs coverage, and ensure 'default' is present.
			// SETUP:
			// 1. Policy with only 'app-a'
			// 2. Cluster with 'app-a' and 'app-b' (Deployments)

			// Mock Client with existing resources
			existingDeploymentA := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-a", Namespace: "default"}}
			existingDeploymentB := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-b", Namespace: "default"}}

			fakeClient := fake.NewClientBuilder().
				WithObjects(existingDeploymentA, existingDeploymentB).
				Build()

			reconciler.Client = fakeClient

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-a": {Enable: true},
					},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
				},
			}

			changed, err := reconciler.checkAndBuildValidResource(context.Background(), policy)
			Expect(err).ToNot(HaveOccurred())

			Expect(changed).To(BeTrue())
			Expect(policy.Spec.Deployments).To(HaveKey("default"))
			Expect(policy.Spec.Deployments["default"].Enable).To(BeTrue())
			Expect(policy.Spec.Deployments).To(HaveKey("app-a"))
		})

		It("should log error but continue if specified resource does not exist in cluster", func() {
			// Scenario: Policy specifies 'app-ghost' which does not exist in cluster.
			// Logic should see mismatch, log error (implied), but continue processing defaults.
			// Also, should we remove it? User said "log error message but continue".
			// Assuming 'continue' means we don't crash and we fix defaults.

			// Mock Client with ONE other resource to ensure 'default' is needed
			existingDeploymentOther := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-other", Namespace: "default"}}
			fakeClient := fake.NewClientBuilder().
				WithObjects(existingDeploymentOther).
				Build()
			reconciler.Client = fakeClient

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-ghost": {Enable: true},
					},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{},
				},
			}

			changed, err := reconciler.checkAndBuildValidResource(context.Background(), policy)
			Expect(err).ToNot(HaveOccurred())

			Expect(changed).To(BeTrue())
			Expect(policy.Spec.Deployments).To(HaveKey("default"))
			Expect(policy.Spec.Deployments["default"].Enable).To(BeTrue())
			Expect(policy.Spec.Deployments).To(HaveKey("app-ghost"))
		})
	})

	Context("Logic: buildTheDesiredState", func() {
		var reconciler *SleepPolicyReconciler

		BeforeEach(func() {
			reconciler = &SleepPolicyReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				Config: &config.Config{
					DefaultWakeAt:   "08:00",
					DefaultSleepAt:  "20:00",
					DefaultTimezone: "UTC",
				},
			}
		})

		It("should apply global defaults when no specific policy exists", func() {
			// Setup: Cluster has Deployment 'app-default'
			// Policy: 'default' enabled, no specific config for 'app-default'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-default", Namespace: "default"}}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep).Build()

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
				},
			}

			// Act
			desiredMap, err := reconciler.buildTheDesiredState(context.Background(), policy)

			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(desiredMap).To(HaveKey("policy-dep-app-default"))
			params := desiredMap["policy-dep-app-default"]
			Expect(params.WakeAt).To(Equal("08:00"))  // Global Default
			Expect(params.SleepAt).To(Equal("20:00")) // Global Default
			Expect(params.Timezone).To(Equal("UTC"))  // Global Default
		})

		It("should respect specific overrides", func() {
			// Setup: Cluster has Deployment 'app-special'
			// Policy: 'app-special' has specific Config
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-special", Namespace: "default"}}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep).Build()

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-special": {
							Enable:   true,
							WakeAt:   "10:00",
							Timezone: "CST", // UTC − 06:00.
						},
						"default": {Enable: true},
					},
				},
			}

			// Act
			desiredMap, err := reconciler.buildTheDesiredState(context.Background(), policy)

			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(desiredMap).To(HaveKey("policy-dep-app-special"))
			params := desiredMap["policy-dep-app-special"]
			Expect(params.WakeAt).To(Equal("10:00"))  // Specific
			Expect(params.SleepAt).To(Equal("20:00")) // Default (inherited because missing in specific)
			Expect(params.Timezone).To(Equal("CST"))  // Specific
		})

		It("should exclude resources that are disabled in policy", func() {
			// Setup: Cluster has Deployment 'app-disabled'
			// Policy: 'app-disabled' set to Enable: false
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-disabled", Namespace: "default"}}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep).Build()

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-disabled": {Enable: false},
						"default":      {Enable: true},
					},
				},
			}

			// Act
			desiredMap, err := reconciler.buildTheDesiredState(context.Background(), policy)

			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(desiredMap).ToNot(HaveKey("policy-dep-app-disabled"))
		})
	})

	Context("Logic: DeploySleepOrderResource", func() {
		var reconciler *SleepPolicyReconciler
		var policy *sleepodv1alpha1.SleepPolicy
		var resourceDesiredState sleepodv1alpha1.ResourceSleepParams

		BeforeEach(func() {
			reconciler = &SleepPolicyReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				Scheme: scheme.Scheme,
				Config: &config.Config{
					DefaultWakeAt:   "08:00",
					DefaultSleepAt:  "20:00",
					DefaultTimezone: "UTC",
				},
			}
		})

		It("should create SleepOrder resource", func() {
			// Setup: Cluster has Deployment 'app-create'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-create", Namespace: "default"}}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep).Build()

			// Setup: Policy has Deployment 'app-create'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-create": {Enable: true},
					},
				},
			}
			resourceDesiredState = sleepodv1alpha1.ResourceSleepParams{
				Name:      "app-create",
				Namespace: "default",
				Kind:      "Deployment",
				WakeAt:    "08:00",
				SleepAt:   "20:00",
				Timezone:  "UTC",
			}

			err := reconciler.DeploySleepOrderResource(context.Background(), policy, resourceDesiredState, actionCreate)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should update SleepOrder resource", func() {
			// Setup: Cluster has Deployment 'app-update'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-update", Namespace: "default"}}
			sleepOrder := &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-dep-app-update", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "Deployment",
						Name: "app-update",
					},
					WakeAt:   "08:00",
					SleepAt:  "20:00",
					Timezone: "UTC",
				},
			}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep, sleepOrder).Build()

			// Setup: Policy has Deployment 'app-update'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-update": {Enable: true, WakeAt: "10:00", SleepAt: "20:00", Timezone: "CST"},
						"default":    {Enable: true},
					},
				},
			}
			resourceDesiredState = sleepodv1alpha1.ResourceSleepParams{
				Name:      "app-update",
				Namespace: "default",
				Kind:      "Deployment",
				WakeAt:    "10:00",
				SleepAt:   "20:00",
				Timezone:  "CST",
			}

			err := reconciler.DeploySleepOrderResource(context.Background(), policy, resourceDesiredState, actionUpdate)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should not do anything, nothing to change", func() {
			// Setup: Cluster has Deployment 'app-nochange'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-nochange", Namespace: "default"}}
			sleepOrder := &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-dep-app-nochange", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "Deployment",
						Name: "app-nochange",
					},
					WakeAt:   "08:00",
					SleepAt:  "20:00",
					Timezone: "UTC",
				},
			}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep, sleepOrder).Build()

			// Setup: Policy has Deployment 'app-nochange'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true, WakeAt: "08:00", SleepAt: "20:00", Timezone: "UTC"},
					},
				},
			}
			resourceDesiredState = sleepodv1alpha1.ResourceSleepParams{
				Name:      "app-nochange",
				Namespace: "default",
				Kind:      "Deployment",
				WakeAt:    "08:00",
				SleepAt:   "20:00",
				Timezone:  "UTC",
			}

			err := reconciler.DeploySleepOrderResource(context.Background(), policy, resourceDesiredState, actionUpdate)

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Logic: needToDeploySleepOrder", func() {
		var reconciler *SleepPolicyReconciler
		var policy *sleepodv1alpha1.SleepPolicy
		var resourceDesiredState sleepodv1alpha1.ResourceSleepParams

		BeforeEach(func() {
			reconciler = &SleepPolicyReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				Scheme: scheme.Scheme,
				Config: &config.Config{
					DefaultWakeAt:   "08:00",
					DefaultSleepAt:  "20:00",
					DefaultTimezone: "UTC",
				},
			}
		})

		It("should create", func() {
			// Setup: Cluster has Deployment 'app-create'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-create", Namespace: "default"}}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep).Build()

			// Setup: Policy has Deployment 'app-create'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-create": {Enable: true},
					},
				},
			}
			resourceDesiredState = sleepodv1alpha1.ResourceSleepParams{
				Name:      "app-create",
				Namespace: "default",
				Kind:      "Deployment",
				WakeAt:    "08:00",
				SleepAt:   "20:00",
				Timezone:  "UTC",
			}

			needToDeploy, action, err := reconciler.needToDeploySleepOrder(context.Background(), "main-policy", resourceDesiredState)
			Expect(err).ToNot(HaveOccurred())

			Expect(needToDeploy).To(BeTrue())
			Expect(action).To(Equal(actionCreate))
		})

		It("should update", func() {
			// Setup: Cluster has StatefulSet 'app-update'
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "app-update", Namespace: "default"}}
			sleepOrder := &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-sts-app-update", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "StatefulSet",
						Name: "app-update",
					},
					WakeAt:   "08:00",
					SleepAt:  "20:00",
					Timezone: "UTC",
				},
			}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(sts, sleepOrder).Build()

			// Setup: Policy has StatefulSet 'app-update'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
						"app-update": {Enable: true, WakeAt: "10:00", SleepAt: "20:00", Timezone: "CST"},
						"default":    {Enable: true},
					},
				},
			}
			resourceDesiredState = sleepodv1alpha1.ResourceSleepParams{
				Name:      "app-update",
				Namespace: "default",
				Kind:      "StatefulSet",
				WakeAt:    "10:00",
				SleepAt:   "20:00",
				Timezone:  "CST",
			}

			needToDeploy, action, err := reconciler.needToDeploySleepOrder(context.Background(), "policy", resourceDesiredState)
			Expect(err).ToNot(HaveOccurred())

			Expect(needToDeploy).To(BeTrue())
			Expect(action).To(Equal(actionUpdate))
		})

		It("should do nothing, nothing to change", func() {
			// Setup: Cluster has Deployment 'app-nochange'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-nochange", Namespace: "default"}}
			sleepOrder := &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-dep-app-nochange", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "Deployment",
						Name: "app-nochange",
					},
					WakeAt:   "08:00",
					SleepAt:  "20:00",
					Timezone: "UTC",
				},
			}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep, sleepOrder).Build()

			// Setup: Policy has Deployment 'app-nochange'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true, WakeAt: "08:00", SleepAt: "20:00", Timezone: "UTC"},
					},
				},
			}
			resourceDesiredState = sleepodv1alpha1.ResourceSleepParams{
				Name:      "app-nochange",
				Namespace: "default",
				Kind:      "Deployment",
				WakeAt:    "08:00",
				SleepAt:   "20:00",
				Timezone:  "UTC",
			}

			err := reconciler.DeploySleepOrderResource(context.Background(), policy, resourceDesiredState, actionUpdate)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Logic: deleteUndesiredResources", func() {
		var reconciler *SleepPolicyReconciler
		var policy *sleepodv1alpha1.SleepPolicy
		var resourceDesiredState map[string]sleepodv1alpha1.ResourceSleepParams

		BeforeEach(func() {
			reconciler = &SleepPolicyReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				Scheme: scheme.Scheme,
				Config: &config.Config{
					DefaultWakeAt:   "08:00",
					DefaultSleepAt:  "20:00",
					DefaultTimezone: "UTC",
				},
			}
		})

		It("should delete undesired resources - default enable is false specified", func() {
			// Setup: Cluster has Deployment 'app-delete'
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-delete", Namespace: "default"}}
			sleepOrder := &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{Name: "default-app-delete", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "Deployment",
						Name: "app-delete",
					},
					WakeAt:   "08:00",
					SleepAt:  "20:00",
					Timezone: "UTC",
				},
			}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(dep, sleepOrder).Build()

			// Setup: Policy has Deployment 'app-delete'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: false},
					},
				},
			}

			err := reconciler.deleteUndesiredResources(context.Background(), policy.Namespace, resourceDesiredState)
			Expect(err).ToNot(HaveOccurred())
			// verify sleepOrder deleted
			Expect(reconciler.Client.Get(context.Background(), types.NamespacedName{Name: "default-app-delete", Namespace: "default"}, &sleepodv1alpha1.SleepOrder{})).ToNot(Succeed())
		})

		It("should do nothing, no sleepOrder to delete", func() {
			// Setup: Cluster has StatefulSet 'app-delete'
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "desired-app", Namespace: "default"}}
			sleepOrder := &sleepodv1alpha1.SleepOrder{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-sts-desired-app", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepOrderSpec{
					TargetRef: sleepodv1alpha1.TargetRef{
						Kind: "StatefulSet",
						Name: "desired-app",
					},
					WakeAt:   "08:00",
					SleepAt:  "20:00",
					Timezone: "UTC",
				},
			}
			reconciler.Client = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(sts, sleepOrder).Build()

			// Setup: Policy has StatefulSet 'desired-app'
			policy = &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true, WakeAt: "08:00", SleepAt: "20:00", Timezone: "UTC"},
					},
				},
			}
			resourceDesiredState = map[string]sleepodv1alpha1.ResourceSleepParams{
				"policy-sts-desired-app": {
					Name:      "desired-app",
					Namespace: "default",
					WakeAt:    "08:00",
					SleepAt:   "20:00",
					Timezone:  "UTC",
				},
			}

			err := reconciler.deleteUndesiredResources(context.Background(), policy.Namespace, resourceDesiredState)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconciler.Client.Get(context.Background(), types.NamespacedName{Name: "policy-sts-desired-app", Namespace: "default"}, &sleepodv1alpha1.SleepOrder{})).To(Succeed())
		})
	})

	Context("Logic: FetchSleepPolicyOrContinue", func() {
		var fakeClient client.Client
		var ctx = context.Background()

		BeforeEach(func() {
			s := scheme.Scheme
			_ = sleepodv1alpha1.AddToScheme(s)
			fakeClient = fake.NewClientBuilder().WithScheme(s).Build()
		})

		It("should return nil if no policies exist", func() {
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "any", Namespace: "default"}}
			policy, err := FetchSleepPolicyOrContinue(ctx, fakeClient, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(policy).To(BeNil())
		})

		It("should return the single policy if one exists", func() {
			existing := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "my-policy", Namespace: "default"}}
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(existing).Build()

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "my-policy", Namespace: "default"}}
			policy, err := FetchSleepPolicyOrContinue(ctx, fakeClient, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(policy).ToNot(BeNil())
			Expect(policy.Name).To(Equal("my-policy"))
		})

		It("should return non-default policy if two exist (one default, one custom)", func() {
			def := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: DefaultSleepPolicyName, Namespace: "default"}}
			custom := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom-policy", Namespace: "default"}}
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(def, custom).Build()

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "any", Namespace: "default"}}
			policy, err := FetchSleepPolicyOrContinue(ctx, fakeClient, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(policy).ToNot(BeNil())
			Expect(policy.Name).To(Equal("custom-policy"))
		})

		It("should error if two non-default policies exist", func() {
			p1 := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom-1", Namespace: "default"}}
			p2 := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom-2", Namespace: "default"}}
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(p1, p2).Build()

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "any", Namespace: "default"}}
			policy, err := FetchSleepPolicyOrContinue(ctx, fakeClient, req)
			Expect(err).To(HaveOccurred())
			Expect(policy).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 2 sleep policies"))
		})

		It("should error if more than 2 policies exist", func() {
			p1 := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom-1", Namespace: "default"}}
			p2 := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom-2", Namespace: "default"}}
			p3 := &sleepodv1alpha1.SleepPolicy{ObjectMeta: metav1.ObjectMeta{Name: "custom-3", Namespace: "default"}}
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(p1, p2, p3).Build()

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "any", Namespace: "default"}}
			policy, err := FetchSleepPolicyOrContinue(ctx, fakeClient, req)
			Expect(err).To(HaveOccurred())
			Expect(policy).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("found 3 sleep policies"))
		})

		It("should return shadowed default policy if it is being deleted", func() {
			s := runtime.NewScheme()
			_ = sleepodv1alpha1.AddToScheme(s)

			defKey := types.NamespacedName{Name: DefaultSleepPolicyName, Namespace: "default"}
			def := &sleepodv1alpha1.SleepPolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SleepPolicy",
					APIVersion: "sleepod.sleepod.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       DefaultSleepPolicyName,
					Namespace:  "default",
					Finalizers: []string{sleepPolicyFinalizer},
				},
			}
			custom := &sleepodv1alpha1.SleepPolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SleepPolicy",
					APIVersion: "sleepod.sleepod.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{Name: "custom-policy", Namespace: "default"},
			}
			fakeClient = fake.NewClientBuilder().WithScheme(s).Build()
			Expect(fakeClient.Create(ctx, def)).To(Succeed())
			Expect(fakeClient.Create(ctx, custom)).To(Succeed())

			// Mark def as deleted
			Expect(fakeClient.Delete(ctx, def)).To(Succeed())
			// Re-fetch to ensure it has deletion timestamp in the client (Create Update Delete loop)
			// Actually fake client Delete sets the timestamp.

			req := ctrl.Request{NamespacedName: defKey}
			policy, err := FetchSleepPolicyOrContinue(ctx, fakeClient, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(policy).ToNot(BeNil())
			Expect(policy.Name).To(Equal(DefaultSleepPolicyName))
		})
	})

	// Integration Tests
	Context("Integration: Reconciliation Logic", func() {
		ctx := context.Background()
		var reconciler *SleepPolicyReconciler
		var fakeClient client.Client

		BeforeEach(func() {
			s := scheme.Scheme
			_ = sleepodv1alpha1.AddToScheme(s)
			fakeClient = fake.NewClientBuilder().
				WithScheme(s).
				WithStatusSubresource(&sleepodv1alpha1.SleepPolicy{}).
				Build()
			reconciler = &SleepPolicyReconciler{
				Client: fakeClient,
				Scheme: s,
				Config: &config.Config{
					DefaultWakeAt:   "08:00",
					DefaultSleepAt:  "20:00",
					DefaultTimezone: "UTC",
				},
			}
		})

		It("Happy Path: Should create SleepOrder for managed Deployment", func() {
			// 1. Setup Cluster State
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "app-happy", Namespace: "default"},
			}
			Expect(fakeClient.Create(ctx, dep)).To(Succeed())

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "main-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"app-happy": {Enable: true, WakeAt: "09:00", SleepAt: "18:00"},
					},
				},
			}
			Expect(fakeClient.Create(ctx, policy)).To(Succeed())

			// 2. Run Reconcile
			// 2. Run Reconcile (Loop to handle Requeue)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "main-policy", Namespace: "default"}}
			var result ctrl.Result
			var err error
			for i := 0; i < 5; i++ {
				result, err = reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				if result == (ctrl.Result{}) {
					break
				}
			}
			Expect(result).To(Equal(ctrl.Result{}))

			// 3. Assert SleepOrder Created
			expectedName := "main-policy-dep-app-happy"
			sleepOrder := &sleepodv1alpha1.SleepOrder{}
			err = reconciler.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: "default"}, sleepOrder)
			Expect(err).ToNot(HaveOccurred())

			// Verify Specs
			Expect(sleepOrder.Spec.WakeAt).To(Equal("09:00"))
			Expect(sleepOrder.Spec.SleepAt).To(Equal("18:00"))
			Expect(sleepOrder.Spec.TargetRef.Kind).To(Equal("Deployment"))
			Expect(sleepOrder.Spec.TargetRef.Name).To(Equal("app-happy"))

			// Verify OwnerReference
			Expect(sleepOrder.OwnerReferences).To(HaveLen(1))
			Expect(sleepOrder.OwnerReferences[0].Name).To(Equal("main-policy"))
		})

		It("Resource Name Collision: Should handle Deployment and StatefulSet with same name", func() {
			// 1. Setup Cluster State: 'web' Deployment AND 'web' StatefulSet
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
			Expect(reconciler.Client.Create(ctx, dep)).To(Succeed())
			Expect(reconciler.Client.Create(ctx, sts)).To(Succeed())

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "main-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true}, // Use default config for both
					},
					StatefulSets: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
				},
			}
			Expect(fakeClient.Create(ctx, policy)).To(Succeed())

			// 2. Run Reconcile
			// 2. Run Reconcile (Loop)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "main-policy", Namespace: "default"}}
			var result ctrl.Result
			var err error
			for i := 0; i < 5; i++ {
				result, err = reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				if result == (ctrl.Result{}) {
					break
				}
			}
			Expect(result).To(Equal(ctrl.Result{}))

			// 3. Assert BOTH SleepOrders Created
			// Deployment SleepOrder
			depSO := &sleepodv1alpha1.SleepOrder{}
			err = reconciler.Get(ctx, types.NamespacedName{Name: "main-policy-dep-web", Namespace: "default"}, depSO)
			Expect(err).ToNot(HaveOccurred())
			Expect(depSO.Spec.TargetRef.Kind).To(Equal("Deployment"))

			// StatefulSet SleepOrder
			stsSO := &sleepodv1alpha1.SleepOrder{}
			err = reconciler.Get(ctx, types.NamespacedName{Name: "main-policy-sts-web", Namespace: "default"}, stsSO)
			Expect(err).ToNot(HaveOccurred())
			Expect(stsSO.Spec.TargetRef.Kind).To(Equal("StatefulSet"))
		})

		It("Garbage Collection: Should delete SleepOrder when resource is removed", func() {
			// 1. Setup: Orphan Deployment exists, gets SleepOrder first
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app-orphan", Namespace: "default"}}
			Expect(fakeClient.Create(ctx, dep)).To(Succeed())

			policy := &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "main-policy", Namespace: "default"},
				Spec: sleepodv1alpha1.SleepPolicySpec{
					Deployments: map[string]sleepodv1alpha1.PolicyConfig{
						"default": {Enable: true},
					},
				},
			}
			Expect(fakeClient.Create(ctx, policy)).To(Succeed())

			// Initial Reconcile -> Create SleepOrder
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "main-policy", Namespace: "default"}}
			var result ctrl.Result
			var err error
			for i := 0; i < 5; i++ {
				result, err = reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				if result == (ctrl.Result{}) {
					break
				}
			}

			// Verify SleepOrder Exists
			soName := "main-policy-dep-app-orphan"
			Expect(reconciler.Client.Get(ctx, types.NamespacedName{Name: soName, Namespace: "default"}, &sleepodv1alpha1.SleepOrder{})).To(Succeed())

			// 2. Action: Delete Deployment
			Expect(reconciler.Client.Delete(ctx, dep)).To(Succeed())

			// 3. Reconcile Again
			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			// 4. Assert SleepOrder Deleted
			Expect(reconciler.Client.Get(ctx, types.NamespacedName{Name: soName, Namespace: "default"}, &sleepodv1alpha1.SleepOrder{})).ToNot(Succeed())
		})
	})
})
