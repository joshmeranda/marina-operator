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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	marinav1 "github.com/joshmeranda/marina-operator/api/v1"
)

const (
	TerminalDeploymentFinalizer = "marina.io.deployment/finalizer"
	TerminalServiceFinalizer    = "marina.io.service/finalizer"
)

func ToPtr[T any](t T) *T {
	return &t
}

// TerminalReconciler reconciles a Terminal object
type TerminalReconciler struct {
	client.Client
}

//+kubebuilder:rbac:groups=terminal.marina.io,resources=terminals,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=terminal.marina.io,resources=terminals/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=terminal.marina.io,resources=terminals/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *TerminalReconciler) reconcileDeployment(ctx context.Context, terminal *marinav1.Terminal) error {
	logger := log.FromContext(ctx)

	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "marina-" + terminal.Name,
			Namespace: terminal.Namespace,
			Labels: map[string]string{
				"app": "marina.terminal",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ToPtr[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "marina",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "marina",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "shell",
							// todo: validate image against allowed / denied images
							Image:   terminal.Spec.Image,
							Command: []string{"bin/sh", "-ec", "trap : TERM INT; sleep infinity & wait"},
						},
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
				},
			},
		},
	}

	if terminal.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(terminal, TerminalDeploymentFinalizer) {
			if err := r.Client.Delete(ctx, desiredDeployment); err != nil {
				return fmt.Errorf("could not delete deployment: %w", err)
			}

			controllerutil.RemoveFinalizer(terminal, TerminalDeploymentFinalizer)

			logger.Info("deleted deployment for terminal",
				"terminal", terminal.Name,
			)
		}

		return nil
	}

	var foundDeployment appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKeyFromObject(desiredDeployment), &foundDeployment); err != nil && errors.IsNotFound(err) {
		if err := r.Client.Create(ctx, desiredDeployment); err != nil {
			return fmt.Errorf("could not create deployment: %w", err)
		}

		controllerutil.AddFinalizer(terminal, TerminalDeploymentFinalizer)

		logger.Info("created deployment for terminal",
			"terminal", client.ObjectKeyFromObject(desiredDeployment),
		)
	} else {
		return fmt.Errorf("could not fetch deployment: %w", err)
	}

	return nil
}

func (r *TerminalReconciler) reconcileService(ctx context.Context, terminal *marinav1.Terminal) error {
	logger := log.FromContext(ctx)

	desiredService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "marina-" + terminal.Name,
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
			Selector: map[string]string{
				"app": "marina.terminal",
			},
		},
	}

	if terminal.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(terminal, TerminalServiceFinalizer) {
			if err := r.Client.Delete(ctx, desiredService); err != nil {
				return fmt.Errorf("could not delete service: %w", err)
			}

			controllerutil.RemoveFinalizer(terminal, TerminalServiceFinalizer)

			logger.Info("deleted service for terminal",
				"terminal", terminal.Name,
			)
		}

		return nil
	}

	var foundService corev1.Service
	if err := r.Get(ctx, client.ObjectKeyFromObject(desiredService), &foundService); err != nil && errors.IsNotFound(err) {
		if err := r.Client.Create(ctx, desiredService); err != nil {
			return fmt.Errorf("could not create service: %w", err)
		}

		controllerutil.AddFinalizer(terminal, TerminalServiceFinalizer)

		logger.Info("created service for terminal",
			"terminal", client.ObjectKeyFromObject(desiredService),
		)
	} else {
		return fmt.Errorf("could not fetch service: %w", err)
	}

	return nil
}

func (r *TerminalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling terminal", "request", req)

	// var terminal *marinav1.Terminal
	terminal := &marinav1.Terminal{}
	if err := r.Get(ctx, req.NamespacedName, terminal); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		logger.Error(err, "unable to fetch Terminal", "terminal", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.reconcileDeployment(ctx, terminal); err != nil {
		logger.Error(err, "unable to reconcile deployment")
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, terminal); err != nil {
		logger.Error(err, "unable to reconcile service")
		return ctrl.Result{}, err
	}

	if err := r.Update(ctx, terminal); err != nil {
		logger.Error(err, "unable to update terminal")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TerminalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&marinav1.Terminal{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
