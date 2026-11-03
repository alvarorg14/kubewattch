package controller

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type EnergyDeviceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	CollectorImage string
}

// +kubebuilder:rbac:groups=energy.kubewattch.alvarorg14.github.io,resources=energydevices,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=energy.kubewattch.alvarorg14.github.io,resources=energydevices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=energy.kubewattch.alvarorg14.github.io,resources=energydevices/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

const collectorFinalizer = "energy.kubewattch.alvarorg14.github.io/collector-auth"

func collectorName(d *energy.EnergyDevice) string {
	return "kw-" + strings.ReplaceAll(string(d.UID), "-", "")
}
func ptr[T any](v T) *T { return &v }
func (r *EnergyDeviceReconciler) status(ctx context.Context, d *energy.EnergyDevice, status metav1.ConditionStatus, reason, message string) error {
	before := d.DeepCopy()
	d.Status.ObservedGeneration = d.Generation
	meta.SetStatusCondition(&d.Status.Conditions, metav1.Condition{Type: "Provisioned", Status: status, Reason: reason, Message: message, ObservedGeneration: d.Generation})
	return r.Status().Patch(ctx, d, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}
func (r *EnergyDeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	d := &energy.EnergyDevice{}
	if err := r.Get(ctx, req.NamespacedName, d); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	name := collectorName(d)
	binding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if !d.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(d, collectorFinalizer) {
			if err := r.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			before := d.DeepCopy()
			controllerutil.RemoveFinalizer(d, collectorFinalizer)
			return ctrl.Result{}, r.Patch(ctx, d, client.MergeFrom(before))
		}
		return ctrl.Result{}, nil
	}
	if d.Spec.Metrics == nil || d.Spec.Metrics.ServingCertSecretName == "" {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
		if err := r.Delete(ctx, deployment); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.status(ctx, d, metav1.ConditionFalse, "MissingMetricsCertificate", "Configure a serving certificate Secret")
	}
	if d.Spec.Tapo == nil {
		return ctrl.Result{}, r.status(ctx, d, metav1.ConditionFalse, "InvalidConfiguration", "Tapo configuration is required")
	}
	image := r.CollectorImage
	if image == "" {
		image = os.Getenv("COLLECTOR_IMAGE")
	}
	if image == "" {
		return ctrl.Result{}, r.status(ctx, d, metav1.ConditionFalse, "MissingCollectorImage", "Configure the collector image")
	}
	if !controllerutil.ContainsFinalizer(d, collectorFinalizer) {
		before := d.DeepCopy()
		controllerutil.AddFinalizer(d, collectorFinalizer)
		if err := r.Patch(ctx, d, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, err
		}
	}
	labels := map[string]string{"app.kubernetes.io/name": "kubewattch-collector", "energy.kubewattch.alvarorg14.github.io/device-uid": string(d.UID)}
	apply := func(obj client.Object, mutate func() error) error {
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
			if err := controllerutil.SetControllerReference(d, obj, r.Scheme); err != nil {
				return err
			}
			return mutate()
		})
		return err
	}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
	if err := apply(sa, func() error { return nil }); err != nil {
		return ctrl.Result{}, err
	}
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
	if err := apply(role, func() error {
		role.Rules = []rbacv1.PolicyRule{{APIGroups: []string{energy.GroupVersion.Group}, Resources: []string{"energydevices"}, ResourceNames: []string{d.Name}, Verbs: []string{"get"}}, {APIGroups: []string{energy.GroupVersion.Group}, Resources: []string{"energydevices/status"}, ResourceNames: []string{d.Name}, Verbs: []string{"get", "update", "patch"}}}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
	if err := apply(rb, func() error {
		rb.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}
		rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: d.Namespace}}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, binding, func() error {
		binding.Labels = map[string]string{"energy.kubewattch.alvarorg14.github.io/device-uid": string(d.UID)}
		binding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "system:auth-delegator"}
		binding.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: d.Namespace}}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
	if err := apply(svc, func() error {
		svc.Labels = labels
		svc.Spec.Selector = labels
		svc.Spec.Ports = []corev1.ServicePort{{Name: "https", Port: 8443, TargetPort: intstr.FromString("metrics")}}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
	if err := apply(deploy, func() error {
		deploy.Spec.Replicas = ptr(int32(1))
		deploy.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deploy.Spec.Template.Labels = labels
		optional := true
		volume := func(n string, ref energy.SecretKeyReference) corev1.Volume {
			return corev1.Volume{Name: n, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: ref.Name, Optional: &optional, Items: []corev1.KeyToPath{{Key: ref.Key, Path: "value"}}}}}
		}
		pod := corev1.PodSpec{ServiceAccountName: name, SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr(true), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}, Volumes: []corev1.Volume{volume("email", d.Spec.Tapo.EmailSecretRef), volume("password", d.Spec.Tapo.PasswordSecretRef), {Name: "metrics-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: d.Spec.Metrics.ServingCertSecretName}}}}}
		interval := d.Spec.CollectionIntervalSeconds
		if interval == 0 {
			interval = 30
		}
		pod.Containers = []corev1.Container{{Name: "collector", Image: image, Args: []string{"--namespace=" + d.Namespace, "--device=" + d.Name, "--uid=" + string(d.UID), "--generation=" + strconv.FormatInt(d.Generation, 10), "--address=" + d.Spec.Tapo.Address, "--interval=" + strconv.Itoa(int(interval))}, Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 8443}, {Name: "health", ContainerPort: 8081}}, VolumeMounts: []corev1.VolumeMount{{Name: "email", MountPath: "/credentials/email", ReadOnly: true}, {Name: "password", MountPath: "/credentials/password", ReadOnly: true}, {Name: "metrics-cert", MountPath: "/certs/metrics", ReadOnly: true}}, SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr(false), ReadOnlyRootFilesystem: ptr(true), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}, LivenessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString("health")}}}, ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromString("health")}}}}}
		deploy.Spec.Template.Spec = pod
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	if os.Getenv("COLLECTOR_SERVICE_MONITOR_ENABLED") == "true" {
		if d.Spec.Metrics.CASecretRef == nil {
			return ctrl.Result{}, r.status(ctx, d, metav1.ConditionFalse, "MissingMetricsCA", "Configure a CA Secret reference")
		}
		mon := &unstructured.Unstructured{}
		mon.SetGroupVersionKind(schemaMonitor())
		mon.SetName(name)
		mon.SetNamespace(d.Namespace)
		if err := apply(mon, func() error {
			labelsMonitor := map[string]string{}
			for _, p := range strings.Split(os.Getenv("COLLECTOR_SERVICE_MONITOR_LABELS"), ",") {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) == 2 {
					labelsMonitor[parts[0]] = parts[1]
				}
			}
			mon.SetLabels(labelsMonitor)
			mon.Object["spec"] = map[string]interface{}{"selector": map[string]interface{}{"matchLabels": stringMap(labels)}, "endpoints": []interface{}{map[string]interface{}{"port": "https", "scheme": "https", "bearerTokenFile": "/var/run/secrets/kubernetes.io/serviceaccount/token", "tlsConfig": map[string]interface{}{"serverName": fmt.Sprintf("%s.%s.svc", name, d.Namespace), "ca": map[string]interface{}{"secret": map[string]interface{}{"name": d.Spec.Metrics.CASecretRef.Name, "key": d.Spec.Metrics.CASecretRef.Key}}}}}}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
	}
	if deploy.Status.ReadyReplicas < 1 {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, r.status(ctx, d, metav1.ConditionFalse, "CollectorNotReady", "Waiting for the collector Deployment")
	}
	return ctrl.Result{}, r.status(ctx, d, metav1.ConditionTrue, "CollectorReady", "Collector Deployment is ready")
}
func (r *EnergyDeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&energy.EnergyDevice{}).Owns(&appsv1.Deployment{}).Owns(&corev1.Service{}).Named("energydevice").Complete(r)
}

func schemaMonitor() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}
}

func stringMap(input map[string]string) map[string]interface{} {
	output := make(map[string]interface{}, len(input))
	for k, v := range input {
		output[k] = v
	}
	return output
}
