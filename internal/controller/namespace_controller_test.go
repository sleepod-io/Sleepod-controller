package controller

import (
	"context"
	"reflect"
	"testing"
	"time"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceCreation(t *testing.T) {
	// Setup Scheme
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = sleepodv1alpha1.AddToScheme(scheme)

	defaultConfig := &config.Config{
		NamespaceConfig: config.NamespaceConfig{
			DelaySeconds: 1,
		},
		DefaultPolicyEnabled: true,
	}

	tests := []struct {
		name                   string
		namespace              *corev1.Namespace
		config                 *config.Config
		expectRequeueAfter     time.Duration
		expecSleepPolicyObject *sleepodv1alpha1.SleepPolicy
		existingObjects        []client.Object
		shouldCreate           bool
	}{
		{
			name: "Test autocreation of sleepPolicy.",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			},
			expectRequeueAfter: 1 * time.Second,
			shouldCreate:       true,
			expecSleepPolicyObject: &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultSleepPolicyName,
					Namespace: "test-namespace",
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
			},
		},
		{
			name: "Test autocreation of sleepPolicy with exclude",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "excluded-namespace",
				},
			},
			config: &config.Config{
				NamespaceConfig: config.NamespaceConfig{
					DelaySeconds:       1,
					ExcludedNamespaces: []string{"excluded-namespace"},
				},
				DefaultPolicyEnabled: true,
			},
			expectRequeueAfter:     1 * time.Second,
			shouldCreate:           false,
			expecSleepPolicyObject: nil,
		},
		{
			name: "Test autocreation of sleepPolicy with sleepPolicy already exists",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			},
			existingObjects: []client.Object{
				&sleepodv1alpha1.SleepPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom-sleep-policy",
						Namespace: "test-namespace",
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
				},
			},
			expectRequeueAfter: 1 * time.Second,
			shouldCreate:       false,
			expecSleepPolicyObject: &sleepodv1alpha1.SleepPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-sleep-policy",
					Namespace: "test-namespace",
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create fake client
			builder := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.namespace)

			if len(tt.existingObjects) > 0 {
				builder.WithObjects(tt.existingObjects...)
			}

			cl := builder.Build()

			testConfig := defaultConfig
			if tt.config != nil {
				testConfig = tt.config
			}

			// create the Reconciler with the fake client
			r := &NamespaceReconciler{
				Client: cl,
				Scheme: scheme,
				Config: testConfig,
			}

			// call the Reconcile method
			result, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name: tt.namespace.Name,
				},
			})
			if err != nil {
				t.Errorf("Reconcile() error = %v", err)
			}
			// validate the requeueAfter mechanism
			diff := tt.expectRequeueAfter - result.RequeueAfter
			if diff > 0 && diff < 1*time.Second {
				t.Errorf("Reconcile() RequeueAfter = %v, expected %v", result.RequeueAfter, tt.expectRequeueAfter)
			}

			if tt.shouldCreate {
				// validate the sleepPolicy was created
				sleepPolicy := &sleepodv1alpha1.SleepPolicy{}
				err = r.Get(context.Background(), client.ObjectKey{
					Name:      tt.expecSleepPolicyObject.Name,
					Namespace: tt.expecSleepPolicyObject.Namespace,
				}, sleepPolicy)
				if err != nil {
					if errors.IsNotFound(err) {
						t.Errorf("Get() error = %v, expected %v", err, tt.expecSleepPolicyObject.Name)
					}
					t.Errorf("Get() error = %v", err)
				}
				if !reflect.DeepEqual(sleepPolicy.Spec, tt.expecSleepPolicyObject.Spec) {
					t.Errorf("Get() SleepPolicy spec = %v, expected %v", sleepPolicy.Spec, tt.expecSleepPolicyObject.Spec)
				}
			} else {
				// validate the sleepPolicy was NOT created
				sleepPolicy := &sleepodv1alpha1.SleepPolicy{}
				err = r.Get(context.Background(), client.ObjectKey{
					Name:      DefaultSleepPolicyName,
					Namespace: tt.namespace.Name,
				}, sleepPolicy)
				if err == nil {
					t.Errorf("SleepPolicy should not be created for excluded namespace")
				}
				if !errors.IsNotFound(err) {
					t.Errorf("Expected IsNotFound error, got %v", err)
				}
				if tt.expecSleepPolicyObject != nil {
					if tt.expecSleepPolicyObject.Name != DefaultSleepPolicyName {
						err = r.Get(context.Background(), client.ObjectKey{
							Name:      tt.expecSleepPolicyObject.Name,
							Namespace: tt.expecSleepPolicyObject.Namespace,
						}, sleepPolicy)
						if err != nil {
							if errors.IsNotFound(err) {
								t.Errorf("Get() error = %v, expected %v", err, tt.expecSleepPolicyObject.Name)
							}
							t.Errorf("Get() error = %v", err)
						}
					}
				}
			}

		})
	}

}

func TestNamespaceDeletion(t *testing.T) {
	// Setup Scheme
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// create fake client without the namespace
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	defaultConfig := &config.Config{
		NamespaceConfig: config.NamespaceConfig{
			DelaySeconds: 1,
		},
		DefaultPolicyEnabled: true,
	}

	// create the Reconciler
	r := &NamespaceReconciler{
		Client: cl,
		Scheme: scheme,
		Config: defaultConfig,
	}

	// call the Reconcile method for a non-existent namespace
	req := ctrl.Request{
		NamespacedName: client.ObjectKey{
			Name: "deleted-namespace",
		},
	}
	result, err := r.Reconcile(context.Background(), req)

	// Assertions
	if err != nil {
		t.Errorf("Reconcile() error = %v, expected nil", err)
	}
	if (result != ctrl.Result{}) {
		t.Errorf("Reconcile() result = %v, expected empty Result", result)
	}
}
