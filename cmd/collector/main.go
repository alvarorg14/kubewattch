package main

import (
	"context"
	"flag"
	"os"
	"time"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	"github.com/alvarorg14/kubewattch/internal/collector"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type worker struct {
	client                                                 client.Client
	namespace, name, uid, address, emailPath, passwordPath string
	generation                                             int64
	interval                                               time.Duration
	metrics                                                *collector.Metrics
}

func (w *worker) NeedLeaderElection() bool { return false }
func (w *worker) Start(ctx context.Context) error {
	var reader *collector.TapoReader
	var oldEmail, oldPassword string
	delay := w.interval
	for {
		email, e1 := os.ReadFile(w.emailPath)
		password, e2 := os.ReadFile(w.passwordPath)
		var err error
		if e1 != nil || e2 != nil || len(email) == 0 || len(password) == 0 {
			err = w.metrics.PollOnce(ctx, failedReader{})
		} else {
			if reader == nil || oldEmail != string(email) || oldPassword != string(password) {
				oldEmail, oldPassword = string(email), string(password)
				reader = collector.NewTapoReader(w.address, oldEmail, oldPassword)
			}
			pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = w.metrics.PollOnce(pollCtx, reader)
			cancel()
		}
		if err != nil {
			if delay < w.interval {
				delay = w.interval
			}
			delay *= 2
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
		} else {
			delay = w.interval
		}
		w.updateStatus(ctx, err == nil)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

type failedReader struct{}

func (failedReader) Read(context.Context) (collector.Observation, error) {
	return collector.Observation{}, os.ErrNotExist
}
func (w *worker) updateStatus(ctx context.Context, success bool) {
	_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		d := &energy.EnergyDevice{}
		if err := w.client.Get(ctx, types.NamespacedName{Namespace: w.namespace, Name: w.name}, d); err != nil {
			return client.IgnoreNotFound(err)
		}
		if string(d.UID) != w.uid || d.Generation != w.generation || !d.DeletionTimestamp.IsZero() {
			return nil
		}
		condition := metav1.Condition{Type: "Collecting", ObservedGeneration: d.Generation, Status: metav1.ConditionFalse, Reason: "CollectionFailed", Message: "Device collection failed"}
		if success {
			now := metav1.Now()
			d.Status.LastObservationTime = &now
			condition.Status = metav1.ConditionTrue
			condition.Reason = "CollectionSucceeded"
			condition.Message = "Device collection succeeded"
		}
		meta.SetStatusCondition(&d.Status.Conditions, condition)
		return w.client.Status().Update(ctx, d)
	})
}
func main() {
	var namespace, name, uid, address, emailPath, passwordPath, certPath string
	var generation int64
	var interval int
	flag.StringVar(&namespace, "namespace", "", "Device namespace")
	flag.StringVar(&name, "device", "", "Device name")
	flag.StringVar(&uid, "uid", "", "Device UID")
	flag.Int64Var(&generation, "generation", 0, "Device generation")
	flag.StringVar(&address, "address", "", "Device address")
	flag.StringVar(&emailPath, "email-file", "/credentials/email/value", "Email file")
	flag.StringVar(&passwordPath, "password-file", "/credentials/password/value", "Password file")
	flag.StringVar(&certPath, "cert-path", "/certs/metrics", "Metrics certificate directory")
	flag.IntVar(&interval, "interval", 30, "Collection interval in seconds")
	flag.Parse()
	if namespace == "" || name == "" || uid == "" || address == "" || generation < 1 || interval < 1 {
		os.Exit(1)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(energy.AddToScheme(scheme))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: ":8443", SecureServing: true, CertDir: certPath, FilterProvider: filters.WithAuthenticationAndAuthorization}, HealthProbeBindAddress: ":8081"})
	if err != nil {
		os.Exit(1)
	}
	m := collector.NewMetrics(namespace, name, "tapo", time.Duration(interval)*time.Second)
	if err = metrics.Registry.Register(m); err != nil {
		os.Exit(1)
	}
	directClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		os.Exit(1)
	}
	var _ manager.LeaderElectionRunnable = (*worker)(nil)
	if err = mgr.Add(&worker{client: directClient, namespace: namespace, name: name, uid: uid, generation: generation, address: address, emailPath: emailPath, passwordPath: passwordPath, interval: time.Duration(interval) * time.Second, metrics: m}); err != nil {
		os.Exit(1)
	}
	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		os.Exit(1)
	}
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
