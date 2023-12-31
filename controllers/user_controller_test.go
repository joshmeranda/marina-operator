package controllers

import (
	"context"

	terminalv1 "github.com/joshmeranda/marina-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("User Controller", func() {
	var reconciler *UserReconciler
	var namespace *corev1.Namespace
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()

		reconciler = &UserReconciler{
			Client: k8sClient,
		}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "marina-system",
				Namespace: "marina-system",
			},
		}

		err := k8sClient.Create(context.Background(), namespace)
		if !errors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		roles := []rbacv1.Role{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "SomeRole",
					Namespace: "marina-system",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "AnotherRole",
					Namespace: "marina-system",
				},
			},
		}

		for _, role := range roles {
			err := k8sClient.Create(ctx, &role)
			if !errors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	When("User with roles is created", Ordered, func() {
		var user *terminalv1.User

		BeforeAll(func() {
			user = &terminalv1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-test", Namespace: "marina-system"},
				Spec: terminalv1.UserSpec{
					Roles: []string{"SomeRole", "AnotherRole"},
				},
			}

			err := k8sClient.Create(ctx, user)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create user resources", func() {
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: user.Namespace,
					Name:      user.Name,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			var serviceaccount corev1.ServiceAccount
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "user-" + user.Name,
				Namespace: user.Namespace,
			}, &serviceaccount)
			Expect(err).NotTo(HaveOccurred())

			var role rbacv1.Role
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "SomeRole",
				Namespace: user.Namespace,
			}, &role)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "AnotherRole",
				Namespace: user.Namespace,
			}, &role)
			Expect(err).NotTo(HaveOccurred())

			var roleBinding rbacv1.RoleBinding
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "user-" + user.Name + "-" + "SomeRole",
				Namespace: user.Namespace,
			}, &roleBinding)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "user-" + user.Name + "-" + "AnotherRole",
				Namespace: user.Namespace,
			}, &roleBinding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should clean up user resources", func() {
			err := k8sClient.Delete(ctx, user)
			Expect(err).NotTo(HaveOccurred())

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: user.Namespace,
					Name:      user.Name,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			var serviceaccount corev1.ServiceAccount
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "user-" + user.Name,
				Namespace: user.Namespace,
			}, &serviceaccount)
			Expect(err).To(HaveOccurred())
			Expect(serviceaccount).To(BeZero())

			var roleBinding rbacv1.RoleBinding
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "user-" + user.Name + "-" + "SomeRole",
				Namespace: user.Namespace,
			}, &roleBinding)
			Expect(err).To(HaveOccurred())
			Expect(roleBinding).To(BeZero())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "user-" + user.Name + "-" + "AnotherRole",
				Namespace: user.Namespace,
			}, &roleBinding)
			Expect(err).To(HaveOccurred())
			Expect(roleBinding).To(BeZero())
		})
	})

	When("User with non-existant roles is created", func() {
		It("shuold fail", func() {
			user := &terminalv1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "user-test", Namespace: "marina-system"},
				Spec: terminalv1.UserSpec{
					Roles: []string{"SomeRole", "NoExistRole", "AnotherRole"},
				},
			}

			err := k8sClient.Create(ctx, user)
			Expect(err).ToNot(HaveOccurred())

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: user.Namespace,
					Name:      user.Name,
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})
})
