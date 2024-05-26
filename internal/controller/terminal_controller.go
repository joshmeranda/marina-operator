package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	marinacorev1 "github.com/joshmeranda/marina-operator.git/api/v1"
)

const (
	TerminalDeploymentFinalizer = "marina.io.deployment/finalizer"
	TerminalServiceFinalizer    = "marina.io.service/finalizer"
)

var (
	CommonLabels = map[string]string{
		"app": "marina-terminal",
	}
)

func ToPtr[T any](t T) *T {
	return &t
}

func deploymentForTerminal(terminal *marinacorev1.Terminal) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "marina-terminal-" + terminal.Name,
			Namespace: terminal.Namespace,
			Labels:    CommonLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ToPtr[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: CommonLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: CommonLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "exec-shell",
							Image:   terminal.Spec.Image,
							Command: []string{"/bin/sh", "-ec", "trap : TERM INT; sleep infinity & wait"},
						},
					},
				},
			},
		},
	}
}

func serviceForTerminal(terminal *marinacorev1.Terminal) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "marina-terminal-" + terminal.Name,
			Namespace: terminal.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "ssh",
					Protocol: corev1.ProtocolTCP,
					Port:     22,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "ssh",
					},
				},
			},
			Selector: CommonLabels,
		},
	}
}

// TerminalReconciler reconciles a Terminal object
type TerminalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.marina.io,resources=terminals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.marina.io,resources=terminals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.marina.io,resources=terminals/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *TerminalReconciler) reconcileDeployment(ctx context.Context, terminal *marinacorev1.Terminal) error {
	logger := log.FromContext(ctx)
	deployment := deploymentForTerminal(terminal)

	if terminal.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(terminal, TerminalDeploymentFinalizer) {
			if err := r.Client.Delete(ctx, deployment); err != nil {
				return fmt.Errorf("could not delete deployment: %w", err)
			}

			controllerutil.RemoveFinalizer(terminal, TerminalDeploymentFinalizer)

			logger.Info("deleted terminal deployment", "terminal", client.ObjectKeyFromObject(terminal))
		}

		return nil
	}

	_ = controllerutil.AddFinalizer(terminal, TerminalDeploymentFinalizer)

	if err := r.Create(ctx, deployment); err != nil {
		return client.IgnoreAlreadyExists(err)
	}

	logger.Info("created terminal deployment", "terminal", client.ObjectKeyFromObject(terminal))

	return nil
}

func (r *TerminalReconciler) reconcileService(ctx context.Context, terminal *marinacorev1.Terminal) error {
	logger := log.FromContext(ctx)
	service := serviceForTerminal(terminal)

	if terminal.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(terminal, TerminalServiceFinalizer) {
			if err := r.Client.Delete(ctx, service); err != nil {
				return fmt.Errorf("could not delete service: %w", err)
			}

			controllerutil.RemoveFinalizer(terminal, TerminalServiceFinalizer)

			logger.Info("deleted terminal service", "terminal", client.ObjectKeyFromObject(terminal))
		}

		return nil
	}

	_ = controllerutil.AddFinalizer(terminal, TerminalServiceFinalizer)

	if err := r.Create(ctx, service); err != nil {
		return client.IgnoreAlreadyExists(err)
	}

	logger.Info("created terminal service", "terminal", client.ObjectKeyFromObject(terminal))

	return nil
}

func (r *TerminalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling terminal", "temrinal", req.NamespacedName)

	terminal := &marinacorev1.Terminal{}
	if err := r.Get(ctx, req.NamespacedName, terminal); err != nil {
		logger.Error(err, "error fetching terminal", "terminal", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.reconcileDeployment(ctx, terminal); err != nil {
		logger.Error(err, "error reconciling terminal deployment", "terminal", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, terminal); err != nil {
		logger.Error(err, "error reconciling terminal service", "terminal", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if err := r.Update(ctx, terminal); err != nil {
		logger.Error(err, "error updating terminal", req.NamespacedName)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TerminalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&marinacorev1.Terminal{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
