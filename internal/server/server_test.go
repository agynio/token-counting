package server_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	tokencountingv1 "github.com/agynio/token-counting/internal/.gen/agynio/api/token_counting/v1"
	"github.com/agynio/token-counting/internal/server"
	"github.com/agynio/token-counting/internal/tokenizer"
)

const bufSize = 1024 * 1024

func TestCountTokensSuccess(t *testing.T) {
	tokenizerInstance := newTokenizer(t)
	client, cleanup := startTestServer(t, tokenizerInstance)
	defer cleanup()

	messageA := []byte(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}`)
	messageB := []byte(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"World"}]}`)

	expectedA := countTokens(t, tokenizerInstance, messageA)
	expectedB := countTokens(t, tokenizerInstance, messageB)

	resp, err := client.CountTokens(context.Background(), &tokencountingv1.CountTokensRequest{
		Model:    tokencountingv1.Model_MODEL_GPT_5,
		Messages: [][]byte{messageA, messageB},
	})
	if err != nil {
		t.Fatalf("CountTokens returned error: %v", err)
	}

	if len(resp.GetTokens()) != 2 {
		t.Fatalf("expected 2 tokens entries, got %d", len(resp.GetTokens()))
	}
	if resp.GetTokens()[0] != int32(expectedA) {
		t.Fatalf("unexpected tokens[0]: got %d want %d", resp.GetTokens()[0], expectedA)
	}
	if resp.GetTokens()[1] != int32(expectedB) {
		t.Fatalf("unexpected tokens[1]: got %d want %d", resp.GetTokens()[1], expectedB)
	}
}

func TestCountTokensModelUnspecified(t *testing.T) {
	tokenizerInstance := newTokenizer(t)
	client, cleanup := startTestServer(t, tokenizerInstance)
	defer cleanup()

	_, err := client.CountTokens(context.Background(), &tokencountingv1.CountTokensRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertStatusCode(t, err, codes.InvalidArgument)
}

func TestCountTokensEmptyMessages(t *testing.T) {
	tokenizerInstance := newTokenizer(t)
	client, cleanup := startTestServer(t, tokenizerInstance)
	defer cleanup()

	_, err := client.CountTokens(context.Background(), &tokencountingv1.CountTokensRequest{
		Model: tokencountingv1.Model_MODEL_GPT_5,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertStatusCode(t, err, codes.InvalidArgument)
}

func TestCountTokensInvalidArgumentMapping(t *testing.T) {
	counter := &stubCounter{err: tokenizer.NewInvalidArgumentError("unsupported content type: input_file", nil)}
	client, cleanup := startTestServer(t, counter)
	defer cleanup()

	_, err := client.CountTokens(context.Background(), &tokencountingv1.CountTokensRequest{
		Model:    tokencountingv1.Model_MODEL_GPT_5,
		Messages: [][]byte{[]byte("{}")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertStatusCode(t, err, codes.InvalidArgument)
	assertStatusMessage(t, err, "message 0: unsupported content type: input_file")
}

func TestCountTokensInternalErrorMapping(t *testing.T) {
	counter := &stubCounter{err: tokenizer.NewInternalError("token counting failed", errors.New("boom"))}
	client, cleanup := startTestServer(t, counter)
	defer cleanup()

	_, err := client.CountTokens(context.Background(), &tokencountingv1.CountTokensRequest{
		Model:    tokencountingv1.Model_MODEL_GPT_5,
		Messages: [][]byte{[]byte("{}")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertStatusCode(t, err, codes.Internal)
	assertStatusMessageContains(t, err, "message 0: token counting failed")
}

func startTestServer(t *testing.T, counter server.TokenCounter) (tokencountingv1.TokenCountingServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	tokencountingv1.RegisterTokenCountingServiceServer(grpcServer, server.New(counter, server.WithLogger(zap.NewNop())))

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}

	client := tokencountingv1.NewTokenCountingServiceClient(conn)
	cleanup := func() {
		conn.Close()
		listener.Close()
		grpcServer.Stop()
	}

	return client, cleanup
}

func newTokenizer(t *testing.T) *tokenizer.Tokenizer {
	t.Helper()
	instance, err := tokenizer.NewTokenizer()
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}
	return instance
}

func countTokens(t *testing.T, tokenizerInstance *tokenizer.Tokenizer, message []byte) int {
	t.Helper()
	count, err := tokenizerInstance.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}
	return count
}

func assertStatusCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	statusErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected status error, got %v", err)
	}
	if statusErr.Code() != code {
		t.Fatalf("expected code %v, got %v", code, statusErr.Code())
	}
}

func assertStatusMessage(t *testing.T, err error, message string) {
	t.Helper()
	statusErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected status error, got %v", err)
	}
	if statusErr.Message() != message {
		t.Fatalf("expected message %q, got %q", message, statusErr.Message())
	}
}

func assertStatusMessageContains(t *testing.T, err error, substring string) {
	t.Helper()
	statusErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected status error, got %v", err)
	}
	if !strings.Contains(statusErr.Message(), substring) {
		t.Fatalf("expected message to contain %q, got %q", substring, statusErr.Message())
	}
}

type stubCounter struct {
	count int
	err   error
}

func (s *stubCounter) CountMessageTokens(_ []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.count, nil
}
