package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	marinacorev1 "github.com/joshmeranda/marina-operator/api/v1"
)

const (
	UserServiceAccountFinalizer = "marina.io.serviceaccount/finalizer"
	UserRoleBindingFinalizer    = "marina.io.rolebinding/finalizer"
)

func serviceAccountForUser(user *marinacorev1.User) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: user.Namespace,
		},
	}
}

func userRoleBindingForRole(user *marinacorev1.User, role string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name + "-" + role,
			Namespace: user.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      user.Name,
				Namespace: user.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     role,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.marina.io,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.marina.io,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.marina.io,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *UserReconciler) reconcileServiceAccount(ctx context.Context, user *marinacorev1.User) error {
	logger := log.FromContext(ctx)
	serviceAccount := serviceAccountForUser(user)

	if user.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(user, UserServiceAccountFinalizer) {
			if err := r.Delete(ctx, serviceAccount); err != nil {
				logger.Error(err, "could not delete service account", "serviceaccount", client.ObjectKeyFromObject(serviceAccount))
				return err
			}

			controllerutil.RemoveFinalizer(user, UserServiceAccountFinalizer)
		}

		return nil
	}

	_ = controllerutil.AddFinalizer(user, UserServiceAccountFinalizer)

	if err := r.Create(ctx, serviceAccount); err != nil {
		return client.IgnoreAlreadyExists(err)
	}

	logger.Info("created service account", "serviceaccount", client.ObjectKeyFromObject(serviceAccount))

	return nil
}

func (r *UserReconciler) reconcileRoleBindings(ctx context.Context, user *marinacorev1.User) error {
	logger := log.FromContext(ctx)
	isDeleting := user.GetDeletionTimestamp() != nil

	if !isDeleting {
		_ = controllerutil.AddFinalizer(user, UserRoleBindingFinalizer)
	}

	for _, role := range user.Spec.Roles {
		binding := userRoleBindingForRole(user, role)

		if isDeleting {
			if controllerutil.ContainsFinalizer(user, UserRoleBindingFinalizer) {
				if err := r.Delete(ctx, binding); err != nil {
					logger.Error(err, "error deleting role binding", "rolebinding", client.ObjectKeyFromObject(binding))
					return err
				}

				logger.Info("deleted role binding", "rolebinding", client.ObjectKeyFromObject(binding))
			}
		} else {
			// assumed roles are validated before we reach this point
			if err := r.Create(ctx, binding); err != nil {
				logger.Error(err, "error creating role binding", "rolebinding", client.ObjectKeyFromObject(binding))
				return client.IgnoreAlreadyExists(err)
			}
		}
	}

	if isDeleting {
		_ = controllerutil.RemoveFinalizer(user, UserRoleBindingFinalizer)
	}

	return nil
}

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	user := &marinacorev1.User{}

	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		logger.Error(err, "error fethcing user", "user", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.reconcileServiceAccount(ctx, user); err != nil {
		logger.Error(err, "error reconciling service account", "user", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if err := r.reconcileRoleBindings(ctx, user); err != nil {
		logger.Error(err, "error reconciling role bindings", "user", req.NamespacedName)
		return ctrl.Result{}, err

	}

	if err := r.Update(ctx, user); err != nil {
		logger.Error(err, "error updating user", "user", req.NamespacedName)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&marinacorev1.User{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
