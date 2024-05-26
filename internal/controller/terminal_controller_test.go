package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	marinacorev1 "github.com/joshmeranda/marina-operator.git/api/v1"
)

var _ = Describe("Terminal Controller", Ordered, func() {
	var reconciler *TerminalReconciler
	var namespace *corev1.Namespace
	var terminal *marinacorev1.Terminal
	var ctx context.Context

	BeforeAll(func() {
		ctx = context.Background()

		reconciler = &TerminalReconciler{
			Client: k8sClient,
		}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "marina-system",
			},
		}

		terminal = &marinacorev1.Terminal{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-terminal",
				Namespace: namespace.Name,
			},
			Spec: marinacorev1.TerminalSpec{
				Image: "busybox: 1.36.0",
			},
		}

		err := k8sClient.Create(ctx, namespace)
		if !errors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred())
		}
	})

	When("a terminal is created", func() {
		It("should create temrinal resources", func() {
			err := k8sClient.Create(ctx, terminal)
			Expect(err).ToNot(HaveOccurred())

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      terminal.Name,
					Namespace: terminal.Namespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			deployment := appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "marina-terminal-" + terminal.Name,
				Namespace: terminal.Namespace,
			}, &deployment)
			Expect(err).ToNot(HaveOccurred())

			service := corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "marina-terminal-" + terminal.Name,
				Namespace: terminal.Namespace,
			}, &service)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("a terminal is deleted", func() {
		It("should delete terminal resources", func() {
			err := k8sClient.Delete(ctx, terminal)
			Expect(err).ToNot(HaveOccurred())

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      terminal.Name,
					Namespace: terminal.Namespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			deployment := appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "maina-terminal-" + terminal.Name,
				Namespace: terminal.Namespace,
			}, &deployment)
			Expect(err).To(HaveOccurred())

			service := corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "maina-terminal-" + terminal.Name,
				Namespace: terminal.Namespace,
			}, &service)
			Expect(err).To(HaveOccurred())
		})
	})
})
