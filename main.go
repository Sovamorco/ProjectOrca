package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ProjectOrca/config"
	"ProjectOrca/migrations"
	pb "ProjectOrca/proto"
	orca "ProjectOrca/service"
	"ProjectOrca/store"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"google.golang.org/grpc"
)

const (
	MigrationsTimeout = time.Second * 300
	ShutdownTimeout   = time.Second * 60
)

type zerologWriter struct {
	io.Writer
	errWriter io.Writer
}

func (z *zerologWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	writer := z.Writer
	if level == zerolog.ErrorLevel {
		writer = z.errWriter
	}

	n, err := writer.Write(p)
	if err != nil {
		return 0, errorx.Decorate(err, "write")
	}

	return n, nil
}

func main() {
	//nolint:reassign // that's the way of zerolog.
	zerolog.ErrorStackMarshaler = utils.MarshalErrorxStack

	writer := zerologWriter{
		Writer:    os.Stdout,
		errWriter: os.Stderr,
	}

	logger := zerolog.New(writer).With().Caller().Timestamp().Stack().Logger().Level(zerolog.DebugLevel)

	log.Logger = logger

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Error loading config")
	}

	if cfg.UseDevLogger {
		//nolint:exhaustruct
		logger = logger.Level(zerolog.TraceLevel).Output(zerolog.ConsoleWriter{Out: os.Stderr})
		log.Logger = logger
	}

	ctx = logger.WithContext(ctx)

	err = run(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Error running")
	}
}

func run(
	ctx context.Context, cfg *config.Config,
) error {
	logger := zerolog.Ctx(ctx)

	st := store.NewStore(&store.Config{
		DB:     cfg.DB,
		Broker: cfg.Redis,
	})

	err := doMigrate(ctx, st.DB)
	if err != nil {
		return errorx.Decorate(err, "do migrations")
	}

	port := 8590

	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return errorx.Decorate(err, "listen")
	}

	srv, err := orca.New(ctx, st, cfg)
	if err != nil {
		return errorx.Decorate(err, "create orca server")
	}

	err = srv.Init(ctx)
	if err != nil {
		return errorx.Decorate(err, "init orca server")
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(srv.UnaryInterceptor),
		grpc.StreamInterceptor(srv.StreamInterceptor),
	)
	pb.RegisterOrcaServer(grpcServer, srv)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- grpcServer.Serve(lis)
	}()

	defer shutdown(ctx, srv, grpcServer)

	logger.Info().Int("port", port).Msg("Started gRPC server")

	select {
	case err = <-serverErrors:
		return errorx.Decorate(err, "listen")
	case <-ctx.Done():
		logger.Info().Msg("Exiting: Context cancelled")

		return nil
	}
}

func shutdown(ctx context.Context, srv *orca.Orca, grpcServer *grpc.Server) {
	parent := context.WithoutCancel(ctx)

	shutdownCtx, cancel := context.WithTimeout(parent, ShutdownTimeout)
	defer cancel()

	srv.ShutdownStreams()

	grpcServer.GracefulStop()

	srv.Shutdown(shutdownCtx)
}

func doMigrate(ctx context.Context, db *bun.DB) error {
	logger := zerolog.Ctx(ctx)

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
			logger.Error().Err(err).Msg("Error unlocking migrator")
		}
	}(migrator, ctx)

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return errorx.Decorate(err, "migrate")
	}

	if group.IsZero() {
		logger.Debug().Msg("No migrations to run")
	} else {
		logger.Info().Int("migrations", len(group.Migrations)).Msg("Ran migrations")
	}

	return nil
}
