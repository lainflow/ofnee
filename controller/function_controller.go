/*
Copyright 2023.

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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openfunction/pkg/functions-framework/python"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	function "github.com/openfunction/api/v2beta1"
)

// FunctionReconciler reconciles a Function object
type FunctionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.openfunction.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openfunction.io,resources=functions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.openfunction.io,resources=functions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Function object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	logger := log.Log

	// Fetch the Function instance
	function := &function.Function{}
	err := r.Get(ctx, req.NamespacedName, function)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Function instance not found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Function instance")
		return ctrl.Result{}, err
	}

	// Get the ConfigMap data.
	configMap := &corev1.ConfigMap{}
	configMapNamespacedName := types.NamespacedName{Name: function.Spec.Code.ConfigMapRef, Namespace: function.Namespace}
	if err := r.Get(context.Background(), configMapNamespacedName, configMap); err != nil {
		logger.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	userCode, ok := configMap.Data[function.Name]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("ConfigMap %s does not contain key %s", function.Spec.Code.ConfigMapRef, function.Name)
	}

	ff := python.CreatePythonFunctionFramework(function.Spec.Name, function.Spec.Port, userCode)
	name := fmt.Sprintf("%s-python-usercode", function.Name)
	userCodeConfigMap := ff.GenerateConfigMap(name, function.Namespace)

	// Create the ConfigMap
	err = r.Create(ctx, userCodeConfigMap)
	if err != nil {
		logger.Error(err, "Failed to create ConfigMap")
		return ctrl.Result{}, err
	}

	// Create a Volume containing the rendered template and any other necessary files
	volume := r.createVolume(ctx, function, userCodeConfigMap)

	// Get the Deployment associated with the Function instance
	deploymentName := function.Name
	deploymentNamespace := req.Namespace
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: deploymentNamespace}, deployment)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Add the Volume to the Deployment
	deployment.Name = deploymentName
	deployment.Namespace = deploymentNamespace
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, *volume)

	// Add the function container to the Deployment
	functionContainer := &corev1.Container{}
	functionContainer.Name = "function"
	functionContainer.Image = "ofn-python-runner:latest"
	functionContainer.ImagePullPolicy = "IfNotPresent"
	functionContainer.Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(function.Spec.Port),
		},
	}

	volumeMount := &corev1.VolumeMount{}
	volumeMount.Name = volume.Name
	volumeMount.MountPath = "/app/"
	functionContainer.VolumeMounts = []corev1.VolumeMount{
		*volumeMount,
	}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{
		*functionContainer,
	}
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": function.Name,
		},
	}

	// Add dapr annotations
	deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{
		"dapr.io/enabled":      "true",
		"dapr.io/app-id":       function.Name,
		"dapr.io/app-port":     fmt.Sprint(function.Spec.Port),
		"dapr.io/app-protocol": "grpc",
	}
	deployment.Spec.Template.Labels = map[string]string{
		"app": function.Name,
	}

	// Create the Deployment
	err = r.Create(ctx, deployment)
	if err != nil {
		logger.Error(err, "Failed to create Deployment")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FunctionReconciler) createVolume(ctx context.Context, function *function.Function, configmap *corev1.ConfigMap) *corev1.Volume {

	// 创建 Volume
	return &corev1.Volume{
		Name: fmt.Sprintf("%s-usercode", function.Name),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configmap.Name,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "main.py",
						Path: "main.py",
					},
					{
						Key:  "function_context.py",
						Path: "function_context.py",
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&function.Function{}).
		Complete(r)
}
