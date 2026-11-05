package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	"github.com/alvarorg14/kubewattch/internal/reporter"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Set only in the isolated e2e reporter build. Release builds use Telegram HTTPS.
var telegramTestEndpoint string

type worker struct {
	client                              client.Client
	namespace, name, uid, resource, job string
	generation                          int64
	credentialRoot                      string
	telegramBase                        string
}
type snapshot struct {
	Scheduled  time.Time `json:"scheduled"`
	Generation int64     `json:"generation"`
	Chunks     []string  `json:"chunks"`
	Complete   bool      `json:"complete"`
}

func (w worker) report(ctx context.Context) (*energy.EnergyReport, error) {
	r := &energy.EnergyReport{}
	if err := w.client.Get(ctx, types.NamespacedName{Namespace: w.namespace, Name: w.name}, r); err != nil {
		return nil, err
	}
	if string(r.UID) != w.uid || r.Generation != w.generation || !r.DeletionTimestamp.IsZero() || r.Spec.Suspend {
		return nil, errors.New("report is stale or suspended")
	}
	return r, nil
}
func (w worker) patchStatus(ctx context.Context, fn func(*energy.EnergyReport) error) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r, err := w.report(ctx)
		if err != nil {
			return err
		}
		if err = fn(r); err != nil {
			return err
		}
		return w.client.Status().Update(ctx, r)
	})
}
func (w worker) scheduledTime(ctx context.Context) (time.Time, error) {
	j := &batchv1.Job{}
	if err := w.client.Get(ctx, types.NamespacedName{Namespace: w.namespace, Name: w.job}, j); err != nil {
		return time.Time{}, err
	}
	valid := false
	for _, o := range j.OwnerReferences {
		if o.Kind == "CronJob" && o.Name == w.resource && o.Controller != nil && *o.Controller {
			valid = true
		}
	}
	if !valid {
		return time.Time{}, errors.New("job is not owned by the report CronJob")
	}
	at, err := time.Parse(time.RFC3339, j.Annotations["batch.kubernetes.io/cronjob-scheduled-timestamp"])
	if err != nil {
		return time.Time{}, errors.New("scheduled timestamp missing or invalid")
	}
	return at, nil
}
func readCredential(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(b))) == 0 {
		return "", errors.New("credential unavailable")
	}
	return strings.TrimSpace(string(b)), nil
}
func (w worker) credentialPath(name string) string {
	root := w.credentialRoot
	if root == "" {
		root = "/credentials"
	}
	return filepath.Join(root, name, "value")
}

func (w worker) makePrometheus(r *energy.EnergyReport) (reporter.Prometheus, error) {
	if r.Spec.Prometheus == nil {
		return reporter.Prometheus{}, errors.New("prometheus configuration missing")
	}
	p := reporter.Prometheus{URL: r.Spec.Prometheus.URL}
	var err error
	if r.Spec.Prometheus.BearerTokenSecretRef != nil {
		p.BearerToken, err = readCredential(w.credentialPath("prom-token"))
		if err != nil {
			return p, err
		}
	}
	if r.Spec.Prometheus.CASecretRef != nil {
		b, err := os.ReadFile(w.credentialPath("prom-ca"))
		if err != nil {
			return p, errors.New("prometheus CA unavailable")
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(b) {
			return p, errors.New("invalid Prometheus CA")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
		p.Client = &http.Client{Timeout: 30 * time.Second, Transport: transport}
	}
	return p, nil
}
func (w worker) build(ctx context.Context, r *energy.EnergyReport, scheduled time.Time) (snapshot, error) {
	selector, err := metav1.LabelSelectorAsSelector(&r.Spec.DeviceSelector)
	if err != nil {
		return snapshot{}, err
	}
	devices := &energy.EnergyDeviceList{}
	if err = w.client.List(ctx, devices, client.InNamespace(w.namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return snapshot{}, err
	}
	selected := make([]reporter.Device, 0, len(devices.Items))
	for _, d := range devices.Items {
		interval := d.Spec.CollectionIntervalSeconds
		if interval == 0 {
			interval = 30
		}
		selected = append(selected, reporter.Device{Name: d.Name, Created: d.CreationTimestamp.Time, Interval: time.Duration(interval) * time.Second})
	}
	p, err := w.makePrometheus(r)
	if err != nil {
		return snapshot{}, err
	}
	zone := r.Spec.TimeZone
	if zone == "" {
		zone = "UTC"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return snapshot{}, err
	}
	result, err := reporter.Build(ctx, p, reporter.ReportInput{Name: r.Name, Namespace: r.Namespace, Period: r.Spec.Period, Currency: r.Spec.Pricing.Currency, Price: r.Spec.Pricing.PricePerKWh, Scheduled: scheduled, Location: loc, Devices: selected})
	if err != nil {
		return snapshot{}, err
	}
	return snapshot{Scheduled: scheduled, Generation: w.generation, Chunks: result.Chunks, Complete: result.Complete}, nil
}
func (w worker) saveSnapshot(ctx context.Context, s snapshot) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if len(raw) > 900000 {
		return errors.New("report snapshot exceeds size limit")
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm := &corev1.ConfigMap{}
		if err := w.client.Get(ctx, types.NamespacedName{Namespace: w.namespace, Name: w.resource}, cm); err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data["snapshot.json"] = string(raw)
		return w.client.Update(ctx, cm)
	})
}
func (w worker) loadSnapshot(ctx context.Context, scheduled time.Time) (snapshot, bool, error) {
	cm := &corev1.ConfigMap{}
	if err := w.client.Get(ctx, types.NamespacedName{Namespace: w.namespace, Name: w.resource}, cm); err != nil {
		return snapshot{}, false, err
	}
	var s snapshot
	if json.Unmarshal([]byte(cm.Data["snapshot.json"]), &s) != nil || !s.Scheduled.Equal(scheduled) || s.Generation != w.generation {
		return snapshot{}, false, nil
	}
	return s, true, nil
}
func (w worker) run(ctx context.Context) (runErr error) {
	var executionTime time.Time
	defer func() {
		if runErr == nil {
			return
		}
		_ = w.patchStatus(ctx, func(r *energy.EnergyReport) error {
			if r.Status.LastRun != nil && (executionTime.IsZero() || r.Status.LastRun.ScheduledTime.After(executionTime)) {
				return nil
			}
			meta.SetStatusCondition(&r.Status.Conditions, metav1.Condition{Type: "Delivered", Status: metav1.ConditionFalse, Reason: "ExecutionFailed", Message: "Report execution failed; inspect Job status", ObservedGeneration: w.generation})
			return nil
		})
	}()
	r, err := w.report(ctx)
	if err != nil {
		return err
	}
	scheduled, err := w.scheduledTime(ctx)
	executionTime = scheduled
	if err != nil {
		return err
	}
	if r.Status.LastRun != nil && r.Status.LastRun.ScheduledTime.After(scheduled) {
		return nil
	}
	if r.Status.LastRun != nil && r.Status.LastRun.ScheduledTime.Time.Equal(scheduled) && r.Status.LastRun.Generation == w.generation && r.Status.LastRun.State == "Succeeded" {
		return nil
	}
	err = w.patchStatus(ctx, func(current *energy.EnergyReport) error {
		if current.Status.LastRun != nil && current.Status.LastRun.ScheduledTime.After(scheduled) {
			return errors.New("newer report execution exists")
		}
		if current.Status.LastRun == nil || !current.Status.LastRun.ScheduledTime.Time.Equal(scheduled) || current.Status.LastRun.Generation != w.generation {
			current.Status.LastRun = &energy.ReportRunStatus{ScheduledTime: metav1.NewTime(scheduled), Generation: w.generation, State: "Running"}
		}
		current.Status.LastScheduleTime = &metav1.Time{Time: scheduled}
		return nil
	})
	if err != nil {
		return err
	}
	s, found, err := w.loadSnapshot(ctx, scheduled)
	if err != nil {
		return err
	}
	if !found {
		s, err = w.build(ctx, r, scheduled)
		if err != nil {
			return err
		}
		if err = w.saveSnapshot(ctx, s); err != nil {
			return err
		}
	}
	err = w.patchStatus(ctx, func(current *energy.EnergyReport) error {
		if current.Status.LastRun != nil && current.Status.LastRun.ScheduledTime.After(scheduled) {
			return errors.New("newer report execution exists")
		}
		if current.Status.LastRun == nil || !current.Status.LastRun.ScheduledTime.Time.Equal(scheduled) || current.Status.LastRun.Generation != w.generation {
			current.Status.LastRun = &energy.ReportRunStatus{ScheduledTime: metav1.NewTime(scheduled), Generation: w.generation, State: "Running"}
		}
		status := metav1.ConditionFalse
		reason := "IncompleteMeasurements"
		message := "Report has missing measurements"
		if s.Complete {
			status = metav1.ConditionTrue
			reason = "MeasurementsComplete"
			message = "Report measurements are complete"
		}
		meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{Type: "DataComplete", Status: status, Reason: reason, Message: message, ObservedGeneration: w.generation})
		return nil
	})
	if err != nil {
		return err
	}
	failures := false
	for i, d := range r.Spec.Destinations {
		if d.Telegram == nil {
			failures = true
			continue
		}
		token, e := readCredential(w.credentialPath(fmt.Sprintf("destination-%d", i)))
		if e != nil {
			failures = true
			continue
		}
		sender := reporter.Telegram{Token: token, ChatID: d.Telegram.ChatID, BaseURL: w.telegramBase}
		for index, chunk := range s.Chunks {
			current, e := w.report(ctx)
			if e != nil {
				return e
			}
			if current.Status.LastRun == nil || !current.Status.LastRun.ScheduledTime.Time.Equal(scheduled) || current.Status.LastRun.Generation != w.generation {
				return errors.New("execution changed")
			}
			progress := int32(0)
			for _, p := range current.Status.LastRun.Destinations {
				if p.Name == d.Name {
					progress = p.ConfirmedChunks
				}
			}
			if int32(index) < progress {
				continue
			}
			if e = sender.Send(ctx, chunk); e != nil {
				failures = true
				break
			}
			e = w.patchStatus(ctx, func(current *energy.EnergyReport) error {
				run := current.Status.LastRun
				if run == nil || !run.ScheduledTime.Time.Equal(scheduled) || run.Generation != w.generation {
					return errors.New("execution changed")
				}
				for k := range run.Destinations {
					if run.Destinations[k].Name == d.Name {
						run.Destinations[k].ConfirmedChunks = int32(index + 1)
						return nil
					}
				}
				run.Destinations = append(run.Destinations, energy.DestinationProgress{Name: d.Name, ConfirmedChunks: int32(index + 1)})
				return nil
			})
			if e != nil {
				return e
			}
		}
	}
	err = w.patchStatus(ctx, func(current *energy.EnergyReport) error {
		if current.Status.LastRun == nil || !current.Status.LastRun.ScheduledTime.Time.Equal(scheduled) {
			return errors.New("execution changed")
		}
		status := metav1.ConditionTrue
		reason := "DeliverySucceeded"
		message := "All destinations received the report"
		current.Status.LastRun.State = "Succeeded"
		if failures {
			status = metav1.ConditionFalse
			reason = "DeliveryFailed"
			message = "One or more destinations failed"
			current.Status.LastRun.State = "Failed"
		} else {
			now := metav1.Now()
			current.Status.LastSuccessfulTime = &now
		}
		meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{Type: "Delivered", Status: status, Reason: reason, Message: message, ObservedGeneration: w.generation})
		return nil
	})
	if err != nil {
		return err
	}
	if failures {
		return errors.New("report delivery failed")
	}
	return nil
}
func main() {
	var w worker
	flag.StringVar(&w.namespace, "namespace", "", "Report namespace")
	flag.StringVar(&w.name, "report", "", "Report name")
	flag.StringVar(&w.uid, "uid", "", "Report UID")
	flag.StringVar(&w.resource, "resource-name", "", "Owned resource name")
	flag.Int64Var(&w.generation, "generation", 0, "Report generation")
	flag.Parse()
	w.job = os.Getenv("JOB_NAME")
	w.telegramBase = telegramTestEndpoint
	pod := os.Getenv("POD_NAME")
	if w.namespace == "" || w.name == "" || w.uid == "" || w.resource == "" || w.generation < 1 || w.job == "" || pod == "" {
		fmt.Fprintln(os.Stderr, "missing reporter configuration")
		os.Exit(1)
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Kubernetes configuration unavailable")
		os.Exit(1)
	}
	scheme := runtime.NewScheme()
	_ = energy.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	w.client, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Kubernetes client unavailable")
		os.Exit(1)
	}
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Kubernetes client unavailable")
		os.Exit(1)
	}
	lock := &resourcelock.LeaseLock{LeaseMeta: metav1.ObjectMeta{Name: w.resource, Namespace: w.namespace}, Client: kube.CoordinationV1(), LockConfig: resourcelock.ResourceLockConfig{Identity: pod}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{Lock: lock, LeaseDuration: 60 * time.Second, RenewDeadline: 40 * time.Second, RetryPeriod: 10 * time.Second, ReleaseOnCancel: true, Callbacks: leaderelection.LeaderCallbacks{OnStartedLeading: func(ctx context.Context) { result <- w.run(ctx); cancel() }, OnStoppedLeading: func() {}}})
	select {
	case err = <-result:
		if err != nil {
			fmt.Fprintln(os.Stderr, "Report execution failed:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "Report execution lost its lock")
		os.Exit(1)
	}
}
