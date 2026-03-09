package server

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	token_countingv1 "github.com/agynio/token-counting/internal/.gen/agynio/api/token_counting/v1"
	"github.com/agynio/token-counting/internal/tokenizer"
)

// Server implements the TokenCountingService gRPC handlers.
type Server struct {
	token_countingv1.UnimplementedTokenCountingServiceServer

	tokenizer *tokenizer.Tokenizer
	logger    *zap.Logger
}

// New constructs a Server with the provided dependencies.
func New(tokenizer *tokenizer.Tokenizer, logger *zap.Logger) *Server {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Server{tokenizer: tokenizer, logger: logger}
}

// CountTokens validates the request and returns per-message token counts.
func (s *Server) CountTokens(ctx context.Context, req *token_countingv1.CountTokensRequest) (*token_countingv1.CountTokensResponse, error) {
	if err := validateCountTokensRequest(req); err != nil {
		return nil, err
	}

	counts := make([]int32, len(req.GetMessages()))
	for i, message := range req.GetMessages() {
		count, err := s.tokenizer.CountMessageTokens(message)
		if err != nil {
			return nil, s.wrapTokenizerError(i, err)
		}
		counts[i] = int32(count)
	}

	return &token_countingv1.CountTokensResponse{Tokens: counts}, nil
}

func validateCountTokensRequest(req *token_countingv1.CountTokensRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request required")
	}
	if req.GetModel() == token_countingv1.Model_MODEL_UNSPECIFIED {
		return status.Error(codes.InvalidArgument, "model is required")
	}
	if len(req.GetMessages()) == 0 {
		return status.Error(codes.InvalidArgument, "at least one message required")
	}
	return nil
}

func (s *Server) wrapTokenizerError(index int, err error) error {
	message := fmt.Sprintf("message %d: %s", index+1, err.Error())

	var invalidErr *tokenizer.InvalidArgumentError
	if errors.As(err, &invalidErr) {
		return status.Error(codes.InvalidArgument, message)
	}

	var internalErr *tokenizer.InternalError
	if errors.As(err, &internalErr) {
		s.logger.Error("token counting failed", zap.Error(err))
		return status.Error(codes.Internal, message)
	}

	s.logger.Error("token counting failed", zap.Error(err))
	return status.Error(codes.Internal, message)
}
