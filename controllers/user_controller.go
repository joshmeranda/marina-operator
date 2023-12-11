/*
MIT License

Copyright (c) 2023 Josh Meranda

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	terminalv1 "github.com/joshmeranda/marina-operator/api/v1"
)

const (
	UserServiceAccountFinalizer = "marina.io.serviceaccount/finalizer"
	UserRoleBindingFinalizer    = "marina.io.rolebinding/finalizer"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
}

//+kubebuilder:rbac:groups=terminal.marina.io,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=terminal.marina.io,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=terminal.marina.io,resources=users/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *UserReconciler) reconcileServiceAccount(ctx context.Context, user *terminalv1.User) error {
	logger := log.FromContext(ctx)

	desiredServiceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-" + user.Name,
			Namespace: user.Namespace,
		},
	}

	if user.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(user, UserServiceAccountFinalizer) {
			if err := r.Delete(ctx, &desiredServiceAccount); err != nil {
				logger.Error(err, "unable to delete service account", "serviceaccount", client.ObjectKeyFromObject(&desiredServiceAccount))
				return err
			}

			controllerutil.RemoveFinalizer(user, UserServiceAccountFinalizer)
			logger.Info("deleted service account", "serviceaccount", client.ObjectKeyFromObject(&desiredServiceAccount))
		}

		return nil
	}

	var foundServiceAccount corev1.ServiceAccount
	if err := r.Get(ctx, client.ObjectKeyFromObject(&desiredServiceAccount), &foundServiceAccount); err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, &desiredServiceAccount); err != nil {
			logger.Error(err, "unable to create service account", "serviceaccount", desiredServiceAccount)
			return err
		}

		controllerutil.AddFinalizer(user, UserServiceAccountFinalizer)
		logger.Info("created service account", "serviceaccount", client.ObjectKeyFromObject(&desiredServiceAccount))
	} else {
		logger.Error(err, "could not fetch service account")
		return err
	}

	return nil
}

func (r *UserReconciler) reconcileRoleBindingss(ctx context.Context, user *terminalv1.User) error {
	logger := log.FromContext(ctx)

	isDeleting := user.GetDeletionTimestamp() != nil

	for _, role := range user.Spec.Roles {
		logger.Info("creating rolebinding for", "role", role)

		desiredRoleBinding := rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-" + user.Name + "-" + role,
				Namespace: user.Namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "user-" + user.Name,
					Namespace: user.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     role,
			},
		}

		if isDeleting {
			if controllerutil.ContainsFinalizer(user, UserRoleBindingFinalizer) {
				if err := r.Delete(ctx, &desiredRoleBinding); err != nil && errors.IsNotFound(err) {
					logger.Error(err, "unable to delete role", "role", client.ObjectKeyFromObject(&desiredRoleBinding))
					return err
				}

				logger.Info("deleted role binding", "rolebinding", client.ObjectKeyFromObject(&desiredRoleBinding))
			}
		} else {
			foundRole := rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      role,
					Namespace: user.Namespace,
				},
			}

			if err := r.Get(ctx, client.ObjectKeyFromObject(&foundRole), &foundRole); err != nil {
				if errors.IsNotFound(err) {
					return fmt.Errorf("role does not exist: %w", err)
				}

				return fmt.Errorf("could not fetch role: %w", err)
			}

			logger.Info("found role", "role", client.ObjectKeyFromObject(&foundRole))

			var foundRoleBinding rbacv1.RoleBinding
			if err := r.Get(ctx, client.ObjectKeyFromObject(&desiredRoleBinding), &foundRoleBinding); err != nil {
				if errors.IsNotFound(err) {
					if err := r.Create(ctx, &desiredRoleBinding); err != nil {
						logger.Error(err, "unable to create role binding", "rolebinding", client.ObjectKeyFromObject(&desiredRoleBinding))
						return err
					}

					logger.Info("created role binding", "rolebinding", client.ObjectKeyFromObject(&desiredRoleBinding))
				} else {
					logger.Error(err, "unable to fetch role binding", "rolebinding", client.ObjectKeyFromObject(&desiredRoleBinding))
					return err
				}
			}

			controllerutil.AddFinalizer(user, UserRoleBindingFinalizer)
		}
	}

	if isDeleting {
		controllerutil.RemoveFinalizer(user, UserRoleBindingFinalizer)
	}

	return nil
}

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	user := &terminalv1.User{}

	logger.Info("[UserReconciler Reconcile] 000")
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("[UserReconciler Reconcile] 001")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		logger.Info("[UserReconciler Reconcile] 002")
		logger.Error(err, "unable to fetch user", "user", req)
		return ctrl.Result{}, err
	}

	logger.Info("[UserReconciler Reconcile] 003")

	if err := r.reconcileServiceAccount(ctx, user); err != nil {
		logger.Error(err, "unable to reconcile service account", "user", user)
		return ctrl.Result{}, err
	}

	if err := r.reconcileRoleBindingss(ctx, user); err != nil {
		logger.Error(err, "unable to reconcile role bindings", "user", user)
		return ctrl.Result{}, err
	}

	if err := r.Update(ctx, user); err != nil {
		logger.Error(err, "unable to update terminal")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terminalv1.User{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
