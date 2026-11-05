package controller

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type EnergyReportReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ReporterImage string
}

// +kubebuilder:rbac:groups=energy.kubewattch.alvarorg14.github.io,resources=energyreports,verbs=get;list;watch
// +kubebuilder:rbac:groups=energy.kubewattch.alvarorg14.github.io,resources=energyreports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

func validateReport(r *energy.EnergyReport) error {
	if r.Spec.Prometheus == nil {
		return errors.New("prometheus configuration is required")
	}
	u, err := url.Parse(r.Spec.Prometheus.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid Prometheus URL")
	}
	if len(strings.Fields(r.Spec.Schedule)) != 5 {
		return errors.New("schedule must have five cron fields")
	}
	if _, err = cron.ParseStandard(r.Spec.Schedule); err != nil {
		return errors.New("invalid cron schedule")
	}
	zone := r.Spec.TimeZone
	if zone == "" {
		zone = "UTC"
	}
	if _, err = time.LoadLocation(zone); err != nil {
		return errors.New("invalid timezone")
	}
	if _, err = metav1.LabelSelectorAsSelector(&r.Spec.DeviceSelector); err != nil {
		return errors.New("invalid device selector")
	}
	if !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(r.Spec.Pricing.Currency) || !regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`).MatchString(r.Spec.Pricing.PricePerKWh) {
		return errors.New("invalid electricity pricing")
	}
	if _, ok := new(big.Rat).SetString(r.Spec.Pricing.PricePerKWh); !ok {
		return errors.New("invalid electricity pricing")
	}
	if r.Spec.Period != "daily" && r.Spec.Period != "weekly" && r.Spec.Period != "monthly" {
		return errors.New("invalid report period")
	}
	if len(r.Spec.Destinations) == 0 {
		return errors.New("at least one destination is required")
	}
	for _, d := range r.Spec.Destinations {
		if d.Type != "telegram" || d.Telegram == nil || d.Telegram.ChatID == "" {
			return errors.New("invalid Telegram destination")
		}
	}
	return nil
}

func reportName(r *energy.EnergyReport) string {
	return "kwr-" + strings.ReplaceAll(string(r.UID), "-", "")
}
func (r *EnergyReportReconciler) setReportStatus(ctx context.Context, report *energy.EnergyReport, condition metav1.ConditionStatus, reason, message string) error {
	before := report.DeepCopy()
	report.Status.ObservedGeneration = report.Generation
	meta.SetStatusCondition(&report.Status.Conditions, metav1.Condition{Type: "Provisioned", Status: condition, Reason: reason, Message: message, ObservedGeneration: report.Generation})
	if reflect.DeepEqual(before.Status, report.Status) {
		return nil
	}
	return r.Status().Patch(ctx, report, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}
func (r *EnergyReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	report := &energy.EnergyReport{}
	if err := r.Get(ctx, req.NamespacedName, report); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !report.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	name := reportName(report)
	cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
	if err := validateReport(report); err != nil {
		existing := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
		if e := r.Get(ctx, client.ObjectKeyFromObject(existing), existing); e == nil {
			if !metav1.IsControlledBy(existing, report) {
				return ctrl.Result{}, errors.New("CronJob is not owned by report")
			}
			if existing.Spec.Suspend == nil || !*existing.Spec.Suspend {
				before := existing.DeepCopy()
				existing.Spec.Suspend = ptr(true)
				if e = r.Patch(ctx, existing, client.MergeFrom(before)); e != nil {
					return ctrl.Result{}, e
				}
			}
		} else if !apierrors.IsNotFound(e) {
			return ctrl.Result{}, e
		}
		return ctrl.Result{}, r.setReportStatus(ctx, report, metav1.ConditionFalse, "InvalidConfiguration", err.Error())
	}
	quantities := map[string]resource.Quantity{}
	for key, fallback := range map[string]string{"REPORTER_CPU_REQUEST": "10m", "REPORTER_MEMORY_REQUEST": "64Mi", "REPORTER_MEMORY_LIMIT": "256Mi"} {
		value := os.Getenv(key)
		if value == "" {
			value = fallback
		}
		quantity, err := resource.ParseQuantity(value)
		if err != nil || quantity.Sign() <= 0 {
			return ctrl.Result{}, r.setReportStatus(ctx, report, metav1.ConditionFalse, "InvalidReporterResources", "Invalid reporter resource configuration")
		}
		quantities[key] = quantity
	}
	image := r.ReporterImage
	if image == "" {
		image = os.Getenv("REPORTER_IMAGE")
	}
	if image == "" {
		image = "ghcr.io/alvarorg14/kubewattch/reporter:dev"
	}
	apply := func(obj client.Object, mutate func() error) error {
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
			if err := controllerutil.SetControllerReference(report, obj, r.Scheme); err != nil {
				return err
			}
			return mutate()
		})
		return err
	}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
	if err := apply(sa, func() error { return nil }); err != nil {
		return ctrl.Result{}, err
	}
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
	if err := apply(role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{APIGroups: []string{energy.GroupVersion.Group}, Resources: []string{"energyreports"}, ResourceNames: []string{report.Name}, Verbs: []string{"get"}},
			{APIGroups: []string{energy.GroupVersion.Group}, Resources: []string{"energyreports/status"}, ResourceNames: []string{report.Name}, Verbs: []string{"get", "update", "patch"}},
			{APIGroups: []string{energy.GroupVersion.Group}, Resources: []string{"energydevices"}, Verbs: []string{"list"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"get"}},
			{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, ResourceNames: []string{name}, Verbs: []string{"get", "update", "patch"}},
			{APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{name}, Verbs: []string{"get", "update", "patch"}},
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
	if err := apply(rb, func() error {
		rb.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}
		rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: report.Namespace}}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	snapshot := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
	if err := apply(snapshot, func() error { return nil }); err != nil {
		return ctrl.Result{}, err
	}
	lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: report.Namespace}}
	if err := apply(lease, func() error { return nil }); err != nil {
		return ctrl.Result{}, err
	}
	zone := report.Spec.TimeZone
	if zone == "" {
		zone = "UTC"
	}
	volumes := []corev1.Volume{}
	mounts := []corev1.VolumeMount{}
	addSecret := func(volume, path string, ref *energy.SecretKeyReference) {
		if ref == nil {
			return
		}
		optional := true
		volumes = append(volumes, corev1.Volume{Name: volume, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: ref.Name, Optional: &optional, Items: []corev1.KeyToPath{{Key: ref.Key, Path: "value"}}}}})
		mounts = append(mounts, corev1.VolumeMount{Name: volume, MountPath: path, ReadOnly: true})
	}
	addSecret("prom-token", "/credentials/prom-token", report.Spec.Prometheus.BearerTokenSecretRef)
	addSecret("prom-ca", "/credentials/prom-ca", report.Spec.Prometheus.CASecretRef)
	for i, d := range report.Spec.Destinations {
		addSecret(fmt.Sprintf("destination-%d", i), fmt.Sprintf("/credentials/destination-%d", i), &d.Telegram.BotTokenSecretRef)
	}
	if err := apply(cj, func() error {
		cj.Spec.Schedule = report.Spec.Schedule
		cj.Spec.TimeZone = &zone
		cj.Spec.Suspend = ptr(report.Spec.Suspend)
		cj.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
		cj.Spec.StartingDeadlineSeconds = ptr(int64(3600))
		cj.Spec.SuccessfulJobsHistoryLimit = ptr(int32(1))
		cj.Spec.FailedJobsHistoryLimit = ptr(int32(3))
		cj.Spec.JobTemplate.Labels = map[string]string{"energy.kubewattch.alvarorg14.github.io/report-uid": string(report.UID), "energy.kubewattch.alvarorg14.github.io/report-generation": strconv.FormatInt(report.Generation, 10)}
		cj.Spec.JobTemplate.Annotations = map[string]string{"energy.kubewattch.alvarorg14.github.io/report-name": report.Name}
		cj.Spec.JobTemplate.Spec.BackoffLimit = ptr(int32(3))
		cj.Spec.JobTemplate.Spec.ActiveDeadlineSeconds = ptr(int64(1800))
		cj.Spec.JobTemplate.Spec.Template.Spec = corev1.PodSpec{ServiceAccountName: name, RestartPolicy: corev1.RestartPolicyNever, SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr(true), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}, Volumes: volumes, Containers: []corev1.Container{{Name: "reporter", Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: quantities["REPORTER_CPU_REQUEST"], corev1.ResourceMemory: quantities["REPORTER_MEMORY_REQUEST"]}, Limits: corev1.ResourceList{corev1.ResourceMemory: quantities["REPORTER_MEMORY_LIMIT"]}}, Image: image, Args: []string{"--namespace=" + report.Namespace, "--report=" + report.Name, "--uid=" + string(report.UID), "--generation=" + strconv.FormatInt(report.Generation, 10), "--resource-name=" + name}, Env: []corev1.EnvVar{{Name: "JOB_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['batch.kubernetes.io/job-name']"}}}, {Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}}}, VolumeMounts: mounts, SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr(false), ReadOnlyRootFilesystem: ptr(true), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}}}}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.setReportStatus(ctx, report, metav1.ConditionTrue, "CronJobReady", "Report CronJob is configured"); err != nil {
		return ctrl.Result{}, err
	}
	before := report.DeepCopy()
	if cj.Status.LastScheduleTime != nil && (report.Status.LastScheduleTime == nil || cj.Status.LastScheduleTime.After(report.Status.LastScheduleTime.Time)) {
		report.Status.LastScheduleTime = cj.Status.LastScheduleTime.DeepCopy()
	}
	jobs := &batchv1.JobList{}
	if err := r.List(ctx, jobs, client.InNamespace(report.Namespace), client.MatchingLabels{"energy.kubewattch.alvarorg14.github.io/report-uid": string(report.UID), "energy.kubewattch.alvarorg14.github.io/report-generation": strconv.FormatInt(report.Generation, 10)}); err != nil {
		return ctrl.Result{}, err
	}
	for _, job := range jobs.Items {
		if !metav1.IsControlledBy(&job, cj) {
			continue
		}
		scheduled, err := time.Parse(time.RFC3339, job.Annotations["batch.kubernetes.io/cronjob-scheduled-timestamp"])
		if err != nil {
			continue
		}
		if report.Status.LastRun != nil && report.Status.LastRun.ScheduledTime.After(scheduled) {
			continue
		}
		failed := false
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				failed = true
			}
		}
		if failed && (report.Status.LastRun == nil || report.Status.LastRun.State != "Succeeded" || report.Status.LastRun.Generation != report.Generation || !report.Status.LastRun.ScheduledTime.Time.Equal(scheduled)) {
			if report.Status.LastRun == nil || report.Status.LastRun.Generation != report.Generation || !report.Status.LastRun.ScheduledTime.Time.Equal(scheduled) {
				report.Status.LastRun = &energy.ReportRunStatus{ScheduledTime: metav1.NewTime(scheduled), Generation: report.Generation}
			}
			report.Status.LastRun.State = "Failed"
			meta.SetStatusCondition(&report.Status.Conditions, metav1.Condition{Type: "Delivered", Status: metav1.ConditionFalse, Reason: "JobFailed", Message: "Report Job failed", ObservedGeneration: report.Generation})
		}
	}
	if reflect.DeepEqual(before.Status, report.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Patch(ctx, report, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}
func (r *EnergyReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&energy.EnergyReport{}).Owns(&batchv1.CronJob{}).Watches(&batchv1.Job{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		name := obj.GetAnnotations()["energy.kubewattch.alvarorg14.github.io/report-name"]
		if name == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: name}}}
	})).Named("energyreport").Complete(r)
}
