package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	tokencountingv1 "github.com/agynio/token-counting/.gen/go/agynio/api/token_counting/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/agynio/token-counting/internal/config"
	"github.com/agynio/token-counting/internal/logging"
	"github.com/agynio/token-counting/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("token-counting: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() {
		_ = logger.Sync()
	}()

	grpcServer := grpc.NewServer()
	tokencountingv1.RegisterTokenCountingServiceServer(grpcServer, server.New())

	lis, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPCAddress, err)
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		grpcServer.GracefulStop()
	}()

	logger.Info("TokenCountingService listening", zap.String("address", cfg.GRPCAddress))

	if err := grpcServer.Serve(lis); err != nil {
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
