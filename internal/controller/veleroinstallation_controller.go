/*
Copyright 2024.

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
	"cmp"
	"context"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	veleroaddonv1 "addons.cluster.x-k8s.io/cluster-api-addon-provider-velero/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	helmv1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

type VeleroHelmState struct {
	DeployNodeAgent bool `yaml:"deployNodeAgent"`
	CleanUpCRDs     bool `yaml:"cleanUpCRDs"`
}

var DefaultHelmState []byte

func init() {
	var parseOk error
	DefaultHelmState, parseOk = yaml.Marshal(VeleroHelmState{
		DeployNodeAgent: true,
		CleanUpCRDs:     true,
	})
	utilruntime.Must(parseOk)
}

// VeleroInstallationReconciler reconciles a VeleroInstallation object
type VeleroInstallationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=veleroinstallations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=veleroinstallations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=veleroinstallations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VeleroInstallation object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *VeleroInstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	installation := &veleroaddonv1.VeleroInstallation{}
	if err := r.Client.Get(ctx, req.NamespacedName, installation); err != nil {
		return ctrl.Result{}, err
	}

	helmProxy := &helmv1.HelmChartProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmProxy); apierrors.IsNotFound(err) {
		return ctrl.Result{}, r.Client.Create(ctx, templateHelmChartProxy(installation))
	} else if err != nil {
		return ctrl.Result{}, err
	}

	installation.Status.HelmChartProxyStatus = helmProxy.Status

	return ctrl.Result{}, r.Client.Status().Update(ctx, installation)
}

func templateHelmChartProxy(installation *veleroaddonv1.VeleroInstallation) *helmv1.HelmChartProxy {
	return &helmv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      installation.Name,
			Namespace: installation.Namespace,
		},
		Spec: *cmp.Or(installation.Spec.HelmChartProxySpec, &helmv1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{},
			RepoURL:         "https://vmware-tanzu.github.io/helm-charts",
			ChartName:       "velero",
			ValuesTemplate:  string(DefaultHelmState),
		}),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VeleroInstallationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&veleroaddonv1.VeleroInstallation{}).
		Owns(&helmv1.HelmChartProxy{}).
		Complete(r)
}