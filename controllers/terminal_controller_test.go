package controllers

import (
	"context"

	marinav1 "github.com/joshmeranda/marina-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Terminal Controller", Ordered, func() {
	var reconciler *TerminalReconciler
	var namespace *corev1.Namespace
	var terminal *marinav1.Terminal
	var ctx context.Context

	BeforeAll(func() {
		ctx = context.Background()

		reconciler = &TerminalReconciler{
			Client: k8sClient,
		}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "marina-system",
				Namespace: "marina-system",
			},
		}

		terminal = &marinav1.Terminal{
			ObjectMeta: metav1.ObjectMeta{Name: "terminal-test", Namespace: "marina-system"},
			Spec: marinav1.TerminalSpec{
				Image: "busybox:1.36",
			},
		}

		err := k8sClient.Create(context.Background(), namespace)
		if !errors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	When("a terminal is created", func() {
		It("should create terminal resources", func() {
			err := k8sClient.Create(ctx, terminal)
			Expect(err).NotTo(HaveOccurred())

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: terminal.Namespace,
					Name:      terminal.Name,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			deployment := appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Namespace: terminal.Namespace,
				Name:      "marina-" + terminal.Name,
			}, &deployment)
			Expect(err).NotTo(HaveOccurred())

			service := corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Namespace: terminal.Namespace,
				Name:      "marina-" + terminal.Name,
			}, &service)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("a terminal is deleted", func() {
		It("should cleanup terminal resources", func() {
			err := k8sClient.Delete(ctx, terminal)
			Expect(err).NotTo(HaveOccurred())

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: terminal.Namespace,
					Name:      terminal.Name,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			deployment := appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Namespace: terminal.Namespace,
				Name:      "marina-" + terminal.Name,
			}, &deployment)
			Expect(err).To(HaveOccurred())
			Expect(deployment).To(BeZero())

			service := corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Namespace: terminal.Namespace,
				Name:      "marina-" + terminal.Name,
			}, &service)
			Expect(err).To(HaveOccurred())
			Expect(service).To(BeZero())
		})
	})
})
