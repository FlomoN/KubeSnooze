package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

type DeploymentReconciler struct {
	client.Client
	deployments  map[string]map[string]bool // namespace/name -> exists
	allZero      bool
	timer        *time.Timer
	timerStarted bool
}

func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the deployment
	var deployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if this deployment is being watched
	if _, exists := r.deployments[fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)]; !exists {
		return ctrl.Result{}, nil
	}

	// Update tracked state
	allZero := true
	for nsName := range r.deployments {
		ns, name, _ := strings.Cut(nsName, "/")
		var d appsv1.Deployment
		if err := r.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &d); err != nil {
			logger.Error(err, "failed to get deployment", "deployment", nsName)
			continue
		}
		if d.Spec.Replicas != nil && *d.Spec.Replicas > 0 {
			allZero = false
			break
		}
	}

	// Handle state change
	if allZero && !r.allZero {
		logger.Info("All watched deployments scaled to 0")
		if !r.timerStarted {
			r.startTimer()
		}
	} else if !allZero && r.allZero {
		if r.timer != nil {
			r.timer.Stop()
			fmt.Println("Timer canceled")
			r.timerStarted = false
		}
	}
	r.allZero = allZero

	return ctrl.Result{}, nil
}

func (r *DeploymentReconciler) startTimer() {
	duration, err := time.ParseDuration(os.Getenv("TIMER_DURATION"))
	if err != nil {
		duration = 1 * time.Hour // default
	}

	r.timer = time.NewTimer(duration)
	r.timerStarted = true
	go func() {
		<-r.timer.C
		fmt.Println("Timer expired")
	}()
}

func loadEnv() error {
	if envPath := os.Getenv("ENV_FILE"); envPath != "" {
		return godotenv.Load(envPath)
	}

	// Try to load .env from the same directory as the binary
	if ex, err := os.Executable(); err == nil {
		if err := godotenv.Load(filepath.Join(filepath.Dir(ex), ".env")); err == nil {
			return nil
		}
	}

	// Try to load .env from current directory
	return godotenv.Load()
}

func main() {
	ctrl.SetLogger(zap.New())
	logger := ctrl.Log.WithName("kubesnooze")

	// Load environment variables from .env file if it exists
	if err := loadEnv(); err != nil {
		logger.Info("No .env file loaded, using environment variables")
	}

	// Parse watched deployments from env
	watchedDeployments := make(map[string]map[string]bool)
	deploymentsEnv := os.Getenv("WATCHED_DEPLOYMENTS")
	if deploymentsEnv == "" {
		logger.Error(nil, "WATCHED_DEPLOYMENTS environment variable is required")
		os.Exit(1)
	}

	for _, d := range strings.Split(deploymentsEnv, ",") {
		if strings.Count(d, "/") != 1 {
			logger.Error(nil, "Invalid deployment format. Use namespace/name", "deployment", d)
			continue
		}
		watchedDeployments[d] = make(map[string]bool)
	}

	if _, exists := os.LookupEnv("TIMER_DURATION"); !exists {
		logger.Info("TIMER_DURATION not set, using default of 1h")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		logger.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := &DeploymentReconciler{
		Client:      mgr.GetClient(),
		deployments: watchedDeployments,
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(reconciler); err != nil {
		logger.Error(err, "unable to create controller")
		os.Exit(1)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "problem running manager")
		os.Exit(1)
	}
}
