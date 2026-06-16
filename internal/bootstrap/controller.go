// Package bootstrap starts a Kargo controller whose promotion engine can
// dispatch CustomPromotionStep-defined steps to plugin sidecars over gRPC.
//
// The wiring mirrors cmd/controlplane/controller.go from Kargo so the controller
// is a faithful Kargo controller — including the optional Argo CD and Argo
// Rollouts integrations (their schemes are registered and a separate Argo CD
// manager is started when those CRDs are present). The only meaningful delta is
// the promotion engine, which is built on a pluginstep.DynamicRegistry (marked
// "kargo-plugin-ext:").
package bootstrap

import (
	"context"
	"fmt"
	stdos "os"
	stdruntime "runtime"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	libCluster "sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	rollouts "github.com/akuity/kargo/api/stubs/rollouts/v1alpha1"
	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	libargocd "github.com/akuity/kargo/pkg/argocd"
	"github.com/akuity/kargo/pkg/controller"
	argocd "github.com/akuity/kargo/pkg/controller/argocd/api/v1alpha1"
	"github.com/akuity/kargo/pkg/controller/promotions"
	"github.com/akuity/kargo/pkg/controller/stages"
	"github.com/akuity/kargo/pkg/controller/warehouses"
	"github.com/akuity/kargo/pkg/credentials"
	credsdb "github.com/akuity/kargo/pkg/credentials/kubernetes"
	"github.com/akuity/kargo/pkg/health"
	healthCheckers "github.com/akuity/kargo/pkg/health/checker/builtin"
	"github.com/akuity/kargo/pkg/heartbeat"
	"github.com/akuity/kargo/pkg/indexer"
	"github.com/akuity/kargo/pkg/logging"
	"github.com/akuity/kargo/pkg/os"
	"github.com/akuity/kargo/pkg/promotion"
	kargokube "github.com/akuity/kargo/pkg/server/kubernetes"
	"github.com/akuity/kargo/pkg/subscription"
	"github.com/akuity/kargo/pkg/types"

	// Register Kargo's built-in step runners and credential providers, exactly as
	// the upstream controller does.
	_ "github.com/akuity/kargo/pkg/credentials/acr"
	_ "github.com/akuity/kargo/pkg/credentials/basic"
	_ "github.com/akuity/kargo/pkg/credentials/ecr"
	_ "github.com/akuity/kargo/pkg/credentials/gar"
	_ "github.com/akuity/kargo/pkg/credentials/github"
	_ "github.com/akuity/kargo/pkg/credentials/ssh"
	_ "github.com/akuity/kargo/pkg/promotion/runner/builtin"

	kargoext "github.com/shamsalmon/kargo-plugin-ext/api/v1alpha1"
	"github.com/shamsalmon/kargo-plugin-ext/pkg/pluginstep"
)

// Options configures the controller.
type Options struct {
	IsDefaultController bool
	ShardName           string

	ControlPlaneKubeConfig string
	QPS                    float32
	Burst                  int

	ArgoCDEnabled       bool
	ArgoCDKubeConfig    string
	ArgoCDNamespaceOnly bool

	MetricsBindAddress string
	PprofBindAddress   string

	Logger *logging.Logger
}

// Run completes Options from the environment, starts the controller, and blocks
// until the context is canceled or a manager exits.
func Run(ctx context.Context) error {
	o := &Options{}
	o.complete()

	startupLogger := o.Logger.WithValues(
		"GOMAXPROCS", stdruntime.GOMAXPROCS(0),
		"GOMEMLIMIT", os.GetEnv("GOMEMLIMIT", ""),
		"defaultController", o.IsDefaultController,
	)
	if o.ShardName != "" {
		startupLogger = startupLogger.WithValues("shard", o.ShardName)
	}
	startupLogger.Info("Starting kargo-plugin-ext controller")

	return o.run(ctx)
}

func (o *Options) complete() {
	o.IsDefaultController = types.MustParseBool(os.GetEnv("IS_DEFAULT_CONTROLLER", "false"))
	o.ShardName = os.GetEnv("SHARD_NAME", "")

	o.ControlPlaneKubeConfig = os.GetEnv("KUBECONFIG", "")
	o.QPS = types.MustParseFloat32(os.GetEnv("KUBE_API_QPS", "50.0"))
	o.Burst = types.MustParseInt(os.GetEnv("KUBE_API_BURST", "300"))

	o.ArgoCDEnabled = types.MustParseBool(os.GetEnv("ARGOCD_INTEGRATION_ENABLED", "true"))
	o.ArgoCDKubeConfig = os.GetEnv("ARGOCD_KUBECONFIG", "")
	o.ArgoCDNamespaceOnly = types.MustParseBool(os.GetEnv("ARGOCD_WATCH_ARGOCD_NAMESPACE_ONLY", "false"))

	o.MetricsBindAddress = os.GetEnv("METRICS_BIND_ADDRESS", "0")
	o.PprofBindAddress = os.GetEnv("PPROF_BIND_ADDRESS", "")

	logLevel, logFormat := getLogVars()
	o.Logger = logging.NewLoggerOrDie(logLevel, logFormat)
}

func (o *Options) run(ctx context.Context) error {
	kargoMgr, localClusterClient, stagesReconcilerCfg, err := o.setupKargoManager(
		ctx,
		stages.ReconcilerConfigFromEnv(),
	)
	if err != nil {
		return fmt.Errorf("error initializing Kargo controller manager: %w", err)
	}

	argocdMgr, err := o.setupArgoCDManager(ctx)
	if err != nil {
		return fmt.Errorf("error initializing Argo CD Application controller manager: %w", err)
	}

	credentialsDB := credsdb.NewDatabase(
		kargoMgr.GetClient(),
		localClusterClient,
		credentials.DefaultProviderRegistry,
		credsdb.DatabaseConfigFromEnv(),
	)

	if err := o.setupReconcilers(
		ctx,
		kargoMgr,
		argocdMgr,
		credentialsDB,
		stagesReconcilerCfg,
	); err != nil {
		return fmt.Errorf("error setting up reconcilers: %w", err)
	}

	return o.startManagers(ctx, kargoMgr, argocdMgr)
}

func (o *Options) setupKargoManager(
	ctx context.Context,
	stagesReconcilerCfg stages.ReconcilerConfig,
) (manager.Manager, client.Client, stages.ReconcilerConfig, error) {
	restCfg, err := kargokube.GetRestConfig(ctx, o.ControlPlaneKubeConfig)
	if err != nil {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error loading REST config for Kargo controller manager: %w", err)
	}
	kargokube.ConfigureQPSBurst(ctx, restCfg, o.QPS, o.Burst)
	restCfg.ContentType = runtime.ContentTypeJSON

	scheme := runtime.NewScheme()
	if err = registerCoreTypes(scheme); err != nil {
		return nil, nil, stagesReconcilerCfg, fmt.Errorf("error building scheme: %w", err)
	}
	// kargo-plugin-ext: register the Argo Rollouts scheme so the Stage reconciler
	// can watch AnalysisRun resources when the integration is enabled and present.
	if stagesReconcilerCfg.RolloutsIntegrationEnabled {
		var exists bool
		if exists, err = argoRolloutsExists(ctx, restCfg); exists {
			o.Logger.Info("Argo Rollouts integration is enabled")
			if err = rollouts.AddToScheme(scheme); err != nil {
				return nil, nil, stagesReconcilerCfg,
					fmt.Errorf("error adding Argo Rollouts API to scheme: %w", err)
			}
		} else {
			if err != nil {
				return nil, nil, stagesReconcilerCfg,
					fmt.Errorf("unable to determine if Argo Rollouts is installed: %w", err)
			}
			stagesReconcilerCfg.RolloutsIntegrationEnabled = false
			o.Logger.Info(
				"Argo Rollouts integration was enabled, but no Argo Rollouts " +
					"CRDs were found. Proceeding without Argo Rollouts integration.",
			)
		}
	}

	cacheOpts := cache.Options{}
	shardReq, err := controller.GetShardRequirement(
		stagesReconcilerCfg.ShardName,
		stagesReconcilerCfg.IsDefaultController,
	)
	if err != nil {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error getting shard requirement: %w", err)
	}
	if shardReq != nil {
		shardSelector := labels.NewSelector().Add(*shardReq)
		cacheOpts.ByObject = map[client.Object]cache.ByObject{
			&kargoapi.Stage{}:     {Label: shardSelector},
			&kargoapi.Promotion{}: {Label: shardSelector},
		}
	}

	mgr, err := ctrl.NewManager(
		restCfg,
		ctrl.Options{
			Scheme:           scheme,
			Metrics:          server.Options{BindAddress: o.MetricsBindAddress},
			PprofBindAddress: o.PprofBindAddress,
			Client: client.Options{
				Cache: &client.CacheOptions{
					DisableFor: []client.Object{
						&corev1.Secret{},
						&corev1.ConfigMap{},
						&coordinationv1.Lease{},
					},
				},
			},
			Cache: cacheOpts,
		},
	)
	if err != nil {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error creating Kargo controller manager: %w", err)
	}

	if o.ControlPlaneKubeConfig == "" ||
		os.GetEnv("LOCAL_CLUSTER_CREDS_FALLBACK", "false") != "true" {
		return mgr, nil, stagesReconcilerCfg, nil
	}

	if restCfg, err = kargokube.GetRestConfig(ctx, ""); err != nil {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error loading REST config for local cluster client: %w", err)
	}
	scheme = runtime.NewScheme()
	if err = corev1.AddToScheme(scheme); err != nil {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error adding Kubernetes core API to local client scheme: %w", err)
	}
	localCluster, err := libCluster.New(
		restCfg,
		func(clusterOptions *libCluster.Options) {
			clusterOptions.Scheme = scheme
			clusterOptions.Client = client.Options{
				Cache: &client.CacheOptions{
					DisableFor: []client.Object{&corev1.Secret{}},
				},
			}
		},
	)
	if err != nil {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error creating Kubernetes client for local cluster: %w", err)
	}
	go func() {
		_ = localCluster.Start(ctx)
	}()
	if !localCluster.GetCache().WaitForCacheSync(ctx) {
		return nil, nil, stagesReconcilerCfg,
			fmt.Errorf("error waiting for local cluster cache to sync")
	}
	return mgr, localCluster.GetClient(), stagesReconcilerCfg, nil
}

func (o *Options) setupArgoCDManager(ctx context.Context) (manager.Manager, error) {
	if !o.ArgoCDEnabled {
		o.Logger.Info("Argo CD integration is disabled")
		return nil, nil
	}

	restCfg, err := kargokube.GetRestConfig(ctx, o.ArgoCDKubeConfig)
	if err != nil {
		return nil, fmt.Errorf("error loading REST config for Argo CD controller manager: %w", err)
	}
	kargokube.ConfigureQPSBurst(ctx, restCfg, o.QPS, o.Burst)
	restCfg.ContentType = runtime.ContentTypeJSON

	argocdNamespace := libargocd.Namespace()

	var exists bool
	if exists, err = argoCDExists(ctx, restCfg, argocdNamespace); !exists || err != nil {
		if err != nil {
			return nil, fmt.Errorf("unable to determine if Argo CD is installed: %w", err)
		}
		o.Logger.Info(
			"Argo CD integration was enabled, but no Argo CD CRDs were found. " +
				"Proceeding without Argo CD integration.",
		)
		return nil, nil
	}

	o.Logger.Info("Argo CD integration is enabled")

	scheme := runtime.NewScheme()
	if err = corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding Kubernetes core API to Argo CD scheme: %w", err)
	}
	if err = argocd.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding Argo CD API to scheme: %w", err)
	}
	cacheOpts := cache.Options{}
	if o.ArgoCDNamespaceOnly {
		cacheOpts.DefaultNamespaces = map[string]cache.Config{argocdNamespace: {}}
	}

	return ctrl.NewManager(
		restCfg,
		ctrl.Options{
			Scheme:  scheme,
			Metrics: server.Options{BindAddress: "0"},
			Cache:   cacheOpts,
		},
	)
}

func (o *Options) setupReconcilers(
	ctx context.Context,
	kargoMgr, argocdMgr manager.Manager,
	credentialsDB credentials.Database,
	stagesReconcilerCfg stages.ReconcilerConfig,
) error {
	var argoCDClient client.Client
	if argocdMgr != nil {
		argoCDClient = argocdMgr.GetClient()
	}

	healthCheckers.Initialize(argoCDClient)

	sharedIndexer := indexer.NewSharedFieldIndexer(kargoMgr.GetFieldIndexer())

	if promotionsReconcilerCfg := promotions.ReconcilerConfigFromEnv(); promotionsReconcilerCfg.Enable {
		// kargo-plugin-ext: the only custom seam — an engine built on a
		// DynamicRegistry that resolves CustomPromotionStep resources and
		// dispatches them to go-plugins.
		registry := pluginstep.NewDynamicRegistry(
			promotion.DefaultStepRunnerRegistry,
			kargoMgr.GetClient(),
			os.GetEnv("PLUGIN_SOCKET_DIR", "/run/plugins"),
		)
		engine := pluginstep.NewEngine(
			registry,
			kargoMgr.GetClient(),
			argoCDClient,
			credentialsDB,
			promotion.NewGitUserResolver(
				kargoMgr.GetClient(),
				os.GetEnv("SYSTEM_RESOURCES_NAMESPACE", ""),
				promotion.GitUserFromEnv(),
			),
			promotion.DefaultExprDataCacheFn,
		)

		if err := promotions.SetupReconcilerWithManager(
			ctx,
			kargoMgr,
			argocdMgr,
			engine,
			promotionsReconcilerCfg,
		); err != nil {
			return fmt.Errorf("error setting up Promotions reconciler: %w", err)
		}
	}

	if err := stages.NewRegularStageReconciler(
		stagesReconcilerCfg,
		health.NewAggregatingChecker(),
	).SetupWithManager(
		ctx,
		kargoMgr,
		argocdMgr,
		sharedIndexer,
	); err != nil {
		return fmt.Errorf("error setting up regular Stages reconciler: %w", err)
	}

	if err := stages.NewControlFlowStageReconciler(stagesReconcilerCfg).SetupWithManager(
		ctx,
		kargoMgr,
		sharedIndexer,
	); err != nil {
		return fmt.Errorf("error setting up control flow Stages reconciler: %w", err)
	}

	if err := warehouses.SetupReconcilerWithManager(
		ctx,
		kargoMgr,
		credentialsDB,
		subscription.DefaultSubscriberRegistry,
		warehouses.ReconcilerConfigFromEnv(),
	); err != nil {
		return fmt.Errorf("error setting up Warehouses reconciler: %w", err)
	}

	renewer := heartbeat.NewRenewer(
		kargoMgr.GetClient(),
		os.GetEnv("KARGO_NAMESPACE", "kargo"),
		o.ShardName,
		time.Duration(os.GetEnvInt("HEARTBEAT_TTL_SECONDS", 120))*time.Second,
	)
	if err := kargoMgr.Add(renewer); err != nil {
		return fmt.Errorf("error registering shard liveness heartbeat renewer: %w", err)
	}

	return nil
}

func (o *Options) startManagers(ctx context.Context, kargoMgr, argocdMgr manager.Manager) error {
	var (
		errChan = make(chan error)
		wg      = sync.WaitGroup{}
	)

	if argocdMgr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := argocdMgr.Start(ctx); err != nil {
				errChan <- fmt.Errorf("error starting Argo CD manager: %w", err)
				return
			}
			if !argocdMgr.GetCache().WaitForCacheSync(ctx) {
				errChan <- fmt.Errorf("failed to wait for Argo CD cache to sync")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := kargoMgr.Start(ctx); err != nil {
			errChan <- fmt.Errorf("error starting Kargo manager: %w", err)
			return
		}
		if !kargoMgr.GetCache().WaitForCacheSync(ctx) {
			errChan <- fmt.Errorf("failed to wait for Kargo cache to sync")
		}
	}()

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case err := <-errChan:
		return err
	case <-doneCh:
		return nil
	}
}

// registerCoreTypes adds the API types the controller always needs to scheme.
// Argo CD and Argo Rollouts types are registered separately (and conditionally)
// in the Argo CD manager and the Kargo manager respectively.
func registerCoreTypes(scheme *runtime.Scheme) error {
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		coordinationv1.AddToScheme,
		kargoapi.AddToScheme,
		kargoext.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			return err
		}
	}
	return nil
}

// --- helpers ported from cmd/controlplane/utils.go ---

func argoCDExists(ctx context.Context, restCfg *rest.Config, namespace string) (bool, error) {
	c, err := dynamic.NewForConfig(restCfg)
	if err == nil {
		if _, err = c.Resource(schema.GroupVersionResource{
			Group:    "argoproj.io",
			Version:  "v1alpha1",
			Resource: "applications",
		}).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			return true, nil
		}
	}
	return false, client.IgnoreNotFound(err)
}

func argoRolloutsExists(ctx context.Context, restCfg *rest.Config) (bool, error) {
	c, err := dynamic.NewForConfig(restCfg)
	if err == nil {
		if _, err = c.Resource(schema.GroupVersionResource{
			Group:    "argoproj.io",
			Version:  "v1alpha1",
			Resource: "analysistemplates",
		}).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			return true, nil
		}
	}
	return false, client.IgnoreNotFound(err)
}

func getLogVars() (logging.Level, logging.Format) {
	logLevelStr := os.GetEnv(logging.LogLevelEnvVar, "info")
	logLevel, err := logging.ParseLevel(logLevelStr)
	if err != nil {
		fmt.Fprintf(stdos.Stderr, "invalid LOG_LEVEL %q, defaulting to info: %v\n", logLevelStr, err)
		logLevel = logging.InfoLevel
	}
	logFormatStr := os.GetEnv(logging.LogFormatEnvVar, string(logging.DefaultFormat))
	logFormat, err := logging.ParseFormat(logFormatStr)
	if err != nil {
		fmt.Fprintf(stdos.Stderr, "invalid LOG_FORMAT %q, defaulting to %q: %v\n", logFormatStr, logging.DefaultFormat, err)
		logFormat = logging.DefaultFormat
	}
	return logLevel, logFormat
}
