package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	tokencountingv1 "github.com/agynio/token-counting/.gen/go/agynio/api/token_counting/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/agynio/token-counting/internal/tokenizer"
)

type Server struct {
	tokencountingv1.UnimplementedTokenCountingServiceServer
}

func New() *Server {
	return &Server{}
}

func (s *Server) CountTokens(ctx context.Context, req *tokencountingv1.CountTokensRequest) (*tokencountingv1.CountTokensResponse, error) {
	model, err := parseModel(req.GetModel())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "model: %v", err)
	}

	messages, err := parseMessages(req.GetMessages())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "messages: %v", err)
	}

	return &tokencountingv1.CountTokensResponse{Tokens: tokenizer.CountTokens(model, messages)}, nil
}

func parseModel(model tokencountingv1.TokenCountingModel) (tokenizer.Model, error) {
	switch model {
	case tokencountingv1.TokenCountingModel_TOKEN_COUNTING_MODEL_GPT_5:
		return tokenizer.ModelGPT5, nil
	case tokencountingv1.TokenCountingModel_TOKEN_COUNTING_MODEL_UNSPECIFIED:
		return "", fmt.Errorf("model must be provided")
	default:
		return "", fmt.Errorf("unsupported model %q", model.String())
	}
}

func parseMessages(raw [][]byte) ([]tokenizer.Message, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	messages := make([]tokenizer.Message, len(raw))
	for i, payload := range raw {
		cleaned := bytes.TrimSpace(payload)
		if len(cleaned) == 0 {
			return nil, fmt.Errorf("message %d is empty", i)
		}
		if !json.Valid(cleaned) {
			return nil, fmt.Errorf("message %d is not valid json", i)
		}
		messages[i] = tokenizer.NewMessage(cleaned)
	}

	return messages, nil
}
