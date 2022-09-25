/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	_ "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ssmv1alpha1 "github.com/fr123k/aws-ssm-operator/api/v1alpha1"

	"github.com/fr123k/aws-ssm-operator/pkg/aws"
	awsCli "github.com/fr123k/aws-ssm-operator/pkg/aws"
)

// var log = logf.Log.WithName("parameterstore-controller")

// ParameterStoreReconciler reconciles a ParameterStore object
type ParameterStoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	SSMc *awsCli.SSMClient
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ssm.aws,resources=parameterstores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ssm.aws,resources=parameterstores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ssm.aws,resources=parameterstores/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ParameterStore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ParameterStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	reqLogger.Info("Reconciling ParameterStore")

	// Fetch the ParameterStore instance
	instance := &ssmv1alpha1.ParameterStore{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile req.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the req.
		return reconcile.Result{}, err
	}

	// Define a new Secret object
	desired, err := r.newSecretForCR(instance)
	if err != nil {
		var ssmStatus ssmv1alpha1.SSMStatus
		var conditionType string
		// Update status.Nodes if needed
		if ssmErr, ok := err.(*awsCli.SSMError); ok {
			ks := make([]ssmv1alpha1.KeyStatus, len(ssmErr.ParameterErrors))
			for i, e := range ssmErr.ParameterErrors {
				ks[i] = ssmv1alpha1.KeyStatus{Name: e.Name, Error: e.Error()}
			}
			ssmStatus = ssmv1alpha1.SSMStatus{
				Key: ks,
			}
			conditionType = ssmv1alpha1.ConditionTypeSSMParamMissing
		} else {
			ssmStatus = ssmv1alpha1.SSMStatus{
				Error: err.Error(),
			}
			conditionType = ssmv1alpha1.ConditionTypeSSMError
		}
		if !reflect.DeepEqual(ssmStatus, instance.Status.SSMStatus) {
			instance.Status.SSMStatus = &ssmStatus
			err := r.Status().Update(ctx, instance)
			if err != nil {
				log.Error(err, "Failed to update ParameterStore status")
				return reconcile.Result{}, err
			}
		}
		{
			readyCondition := metav1.Condition{
				Status:             metav1.ConditionFalse,
				Reason:             ssmv1alpha1.ReconciliationFailedReason,
				Message:            err.Error(),
				Type:               conditionType,
				ObservedGeneration: instance.GetGeneration(),
			}
			apimeta.SetStatusCondition(&instance.Status.Conditions, readyCondition)
			err := r.Status().Update(ctx, instance)
			if err != nil {
				log.Error(err, "Failed to update ParameterStore status")
				return reconcile.Result{}, err
			}
		}

		log.Error(err, "Failed to fetch SSM parameters status")
		return reconcile.Result{}, err
	} else {
		instance.Status.SSMStatus = nil
	}

	// Set ParameterStore instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Secret already exists
	current := &corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, current)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Secret", "desired.Namespace", desired.Namespace, "desired.Name", desired.Name)
			err = r.Client.Create(context.TODO(), desired)
		}
	} else {
		reqLogger.Info("Updating an existing Secret", "desired.Namespace", desired.Namespace, "desired.Name", desired.Name)
		err = r.Client.Update(context.TODO(), desired)
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	// Update status.Nodes if needed
	secretStatus := ssmv1alpha1.SecretStatus{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}
	if !reflect.DeepEqual(secretStatus, instance.Status.SecretStatus) {
		instance.Status.SecretStatus = &secretStatus
		err := r.Status().Update(ctx, instance)
		if err != nil {
			log.Error(err, "Failed to update ParameterStore status")
			return reconcile.Result{}, err
		}
	}

	readyCondition := metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ssmv1alpha1.ReconciliationSucceededReason,
		Message:            fmt.Sprintf("Secret %s in ready state", desired.Name),
		Type:               ssmv1alpha1.ConditionTypeReady,
		ObservedGeneration: instance.GetGeneration(),
	}
	apimeta.SetStatusCondition(&instance.Status.Conditions, readyCondition)
	err = r.Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "Failed to update ParameterStore status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// newSecretForCR returns a Secret with the same name/namespace as the cr
func (r *ParameterStoreReconciler) newSecretForCR(cr *ssmv1alpha1.ParameterStore) (*corev1.Secret, error) {
	labels := map[string]string{
		"app": cr.Name,
	}
	ref := cr.Spec.ValueFrom.ParameterStoreRef
	var data1 = make(map[string]string)
	if ref != nil {
		var err *aws.SSMError
		data1, err = r.SSMc.SSMParameterValueToSecret(*ref)

		if err != nil {
			return nil, err
		}
	}
	var data2 = make(map[string]string)
	var anno = make(map[string]string)

	if cr.Spec.ValueFrom.ParametersStoreRef != nil {
		var err *aws.SSMError
		data2, anno, err = r.SSMc.SSMParametersValueToSecret(cr.Spec.ValueFrom.ParametersStoreRef)

		if err != nil {
			return nil, err
		}
	}

	for k, v := range data1 {
		data2[k] = v
	}

	anno["aws-ssm-operator/updated"] = time.Now().Format(time.RFC3339)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: anno,
		},
		StringData: data2,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParameterStoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ssmv1alpha1.ParameterStore{}).
		// WithOptions(controller.Options{RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 10*time.Second)}).
		//This ignores changes on the Custome Resource that were made outside of the Spec like Metadata or Status.
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(r)
}
