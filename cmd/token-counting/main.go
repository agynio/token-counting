package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	token_countingv1 "github.com/agynio/token-counting/internal/.gen/agynio/api/token_counting/v1"
	"github.com/agynio/token-counting/internal/config"
	"github.com/agynio/token-counting/internal/logging"
	"github.com/agynio/token-counting/internal/server"
	"github.com/agynio/token-counting/internal/tokenizer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "token-counting service failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	tokenizerInstance, err := tokenizer.NewTokenizer()
	if err != nil {
		return fmt.Errorf("init tokenizer: %w", err)
	}

	grpcServer := grpc.NewServer()
	token_countingv1.RegisterTokenCountingServiceServer(grpcServer, server.New(tokenizerInstance, logger))

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPCAddr, err)
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("gRPC server starting", zap.String("addr", cfg.GRPCAddr))
		err := grpcServer.Serve(listener)
		if errors.Is(err, grpc.ErrServerStopped) {
			err = nil
		}
		serverErr <- err
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		grpcServer.GracefulStop()
		err := <-serverErr
		if err != nil {
			return err
		}
	}

	return nil
}
