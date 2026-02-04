package controller

import (
	"context"
	"testing"
	"time"

	sleepodv1alpha1 "github.com/sleepod-io/sleepod-controller/api/v1alpha1"
	"github.com/sleepod-io/sleepod-controller/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceTTL(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = sleepodv1alpha1.AddToScheme(scheme)

	now := time.Now().UTC()
	futureDate := now.AddDate(0, 0, 10).Format("02/01/2006")
	pastDate := now.AddDate(0, 0, -10).Format("02/01/2006")

	tests := []struct {
		name             string
		namespace        *corev1.Namespace
		config           *config.Config
		expectDelete     bool
		expectAnnotation bool
		expectRequeue    bool
	}{
		{
			name: "TTL Disabled - No Action",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "ttl-disabled"},
			},
			config: &config.Config{
				NamespaceConfig: config.NamespaceConfig{TTLEnabled: false},
			},
			expectDelete:     false,
			expectAnnotation: false,
		},
		{
			name: "TTL Enabled - Add Annotation",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "ttl-add-annotation"},
			},
			config: &config.Config{
				NamespaceConfig: config.NamespaceConfig{
					TTLEnabled:          true,
					ExpirationTTLInDays: 7,
				},
			},
			expectDelete:     false,
			expectAnnotation: true,
		},
		{
			name: "TTL Enabled - Future Expiration - Requeue",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ttl-future",
					Annotations: map[string]string{
						namespaceExpirationDate: futureDate,
					},
				},
			},
			config: &config.Config{
				NamespaceConfig: config.NamespaceConfig{TTLEnabled: true},
			},
			expectDelete:  false,
			expectRequeue: true,
		},
		{
			name: "TTL Enabled - Past Expiration - Delete",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ttl-past",
					Annotations: map[string]string{
						namespaceExpirationDate: pastDate,
					},
				},
			},
			config: &config.Config{
				NamespaceConfig: config.NamespaceConfig{TTLEnabled: true},
			},
			expectDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.namespace).Build()

			// Ensure config defaults if likely nil
			if tt.config.DefaultTimezone == "" {
				tt.config.DefaultTimezone = "UTC"
			}
			// Set delay to avoid nil pointer if needed
			if tt.config.NamespaceConfig.DelaySeconds == 0 {
				tt.config.NamespaceConfig.DelaySeconds = 1
			}

			r := &NamespaceReconciler{
				Client: cl,
				Scheme: scheme,
				Config: tt.config,
			}

			result, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKey{Name: tt.namespace.Name},
			})
			if err != nil {
				t.Errorf("Reconcile() error = %v", err)
			}

			// Check Requeue
			if tt.expectRequeue {
				if result.RequeueAfter == 0 {
					t.Errorf("Expected RequeueAfter > 0, got 0")
				}
			}

			// Check existence
			ns := &corev1.Namespace{}
			err = cl.Get(context.Background(), client.ObjectKey{Name: tt.namespace.Name}, ns)
			if tt.expectDelete {
				if !errors.IsNotFound(err) {
					t.Errorf("Expected namespace to be deleted, but found error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected namespace to exist, but got error: %v", err)
				}
				if tt.expectAnnotation {
					if _, ok := ns.Annotations[namespaceExpirationDate]; !ok {
						t.Errorf("Expected expiration annotation to be present")
					}
				}
			}
		})
	}
}
