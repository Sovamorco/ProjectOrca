package main

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"ProjectOrca/config"
	"ProjectOrca/migrations"
	pb "ProjectOrca/proto"
	orca "ProjectOrca/service"
	"ProjectOrca/store"

	"github.com/hashicorp/vault-client-go"
	"github.com/joomcode/errorx"
	gvault "github.com/sovamorco/gommon/vault"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

const (
	MigrationsTimeout = time.Second * 300
	ShutdownTimeout   = time.Second * 60
)

func main() {
	coreLogger, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.DebugLevel))
	if err != nil {
		panic(err)
	}

	logger := coreLogger.Sugar()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var vc *vault.Client

	var cfg *config.Config

	vc, err = gvault.ClientFromEnv(ctx)
	if err != nil {
		logger.Debugf("Failed to create vault client: %+v", err)

		cfg, err = config.LoadConfig(ctx)
	} else {
		cfg, err = config.LoadConfigVault(ctx, vc)
	}

	if err != nil {
		logger.Fatalf("Error loading config: %+v", err)
	}

	err = run(ctx, logger, vc, cfg)
	if err != nil {
		logger.Fatalf("Error running: %+v", err)
	}
}

func run(
	ctx context.Context, logger *zap.SugaredLogger, vc *vault.Client, cfg *config.Config,
) error {
	st := store.NewStore(ctx, logger, &store.Config{
		DB:     cfg.DB,
		Broker: cfg.Redis,
	}, vc)

	err := doMigrate(ctx, logger, st.DB)
	if err != nil {
		return errorx.Decorate(err, "do migrations")
	}

	port := 8590

	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return errorx.Decorate(err, "listen")
	}

	srv, err := orca.New(ctx, logger, st, cfg)
	if err != nil {
		return errorx.Decorate(err, "create orca server")
	}

	err = srv.Init(ctx)
	if err != nil {
		return errorx.Decorate(err, "init orca server")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterOrcaServer(grpcServer, srv)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- grpcServer.Serve(lis)
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()

		srv.ShutdownStreams()

		grpcServer.GracefulStop()

		srv.Shutdown(shutdownCtx)
	}()

	logger.Infof("Started gRPC server on port %d", port)

	select {
	case err = <-serverErrors:
		return errorx.Decorate(err, "listen")
	case <-ctx.Done():
		logger.Infof("Exiting: Context cancelled")

		return nil
	}
}

func doMigrate(ctx context.Context, logger *zap.SugaredLogger, db *bun.DB) error {
	logger = logger.Named("migrate")

	migrateContext, migrateCancel := context.WithTimeout(ctx, MigrationsTimeout)
	defer migrateCancel()

	m, err := migrations.NewMigrations()
	if err != nil {
		return errorx.Decorate(err, "get migrations")
	}

	migrator := migrate.NewMigrator(db, m, migrate.WithMarkAppliedOnSuccess(true))

	err = migrator.Init(migrateContext)
	if err != nil {
		return errorx.Decorate(err, "migrate init")
	}

	if err = migrator.Lock(ctx); err != nil {
		return errorx.Decorate(err, "lock")
	}

	defer func(migrator *migrate.Migrator, ctx context.Context) {
		err := migrator.Unlock(ctx)
		if err != nil {
			logger.Errorf("Error unlocking migrator: %+v", err)
		}
	}(migrator, ctx)

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return errorx.Decorate(err, "migrate")
	}

	if group.IsZero() {
		logger.Debug("No migrations to run")
	} else {
		logger.Infof("Ran %d migrations", len(group.Migrations))
	}

	return nil
}
