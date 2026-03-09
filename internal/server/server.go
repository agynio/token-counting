package server

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tokencountingv1 "github.com/agynio/token-counting/internal/.gen/agynio/api/token_counting/v1"
	"github.com/agynio/token-counting/internal/tokenizer"
)

// TokenCounter describes the tokenizer behavior needed by the server.
type TokenCounter interface {
	CountMessageTokens(messageJSON []byte) (int, error)
}

// Option mutates server configuration.
type Option func(*Server)

// WithLogger overrides the logger used by the server.
func WithLogger(logger *zap.Logger) Option {
	return func(s *Server) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// Server implements the TokenCountingService gRPC handlers.
type Server struct {
	tokencountingv1.UnimplementedTokenCountingServiceServer

	counter TokenCounter
	logger  *zap.Logger
}

// New constructs a Server with the provided dependencies.
func New(counter TokenCounter, opts ...Option) *Server {
	if counter == nil {
		panic("token counter is required")
	}
	s := &Server{
		counter: counter,
		logger:  zap.NewNop(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CountTokens validates the request and returns per-message token counts.
func (s *Server) CountTokens(ctx context.Context, req *tokencountingv1.CountTokensRequest) (*tokencountingv1.CountTokensResponse, error) {
	if err := validateCountTokensRequest(req); err != nil {
		return nil, err
	}

	counts := make([]int32, len(req.GetMessages()))
	for i, message := range req.GetMessages() {
		count, err := s.counter.CountMessageTokens(message)
		if err != nil {
			return nil, s.wrapTokenizerError(i, err)
		}
		counts[i] = int32(count)
	}

	return &tokencountingv1.CountTokensResponse{Tokens: counts}, nil
}

func validateCountTokensRequest(req *tokencountingv1.CountTokensRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request required")
	}
	if req.GetModel() == tokencountingv1.Model_MODEL_UNSPECIFIED {
		return status.Error(codes.InvalidArgument, "model is required")
	}
	if len(req.GetMessages()) == 0 {
		return status.Error(codes.InvalidArgument, "at least one message required")
	}
	return nil
}

func (s *Server) wrapTokenizerError(index int, err error) error {
	message := fmt.Sprintf("message %d: %s", index, err.Error())

	var invalidErr *tokenizer.InvalidArgumentError
	if errors.As(err, &invalidErr) {
		return status.Error(codes.InvalidArgument, message)
	}

	var internalErr *tokenizer.InternalError
	if errors.As(err, &internalErr) {
		s.logger.Error("token counting failed", zap.Error(err))
		return status.Error(codes.Internal, message)
	}

	s.logger.Warn("token counting failed with unknown error", zap.Error(err), zap.String("type", fmt.Sprintf("%T", err)))
	return status.Error(codes.Internal, message)
}
