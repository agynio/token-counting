package server_test

import (
	"context"
	"net"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	token_countingv1 "github.com/agynio/token-counting/internal/.gen/agynio/api/token_counting/v1"
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

	resp, err := client.CountTokens(context.Background(), &token_countingv1.CountTokensRequest{
		Model:    token_countingv1.Model_MODEL_GPT_5,
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

	_, err := client.CountTokens(context.Background(), &token_countingv1.CountTokensRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertStatusCode(t, err, codes.InvalidArgument)
}

func TestCountTokensEmptyMessages(t *testing.T) {
	tokenizerInstance := newTokenizer(t)
	client, cleanup := startTestServer(t, tokenizerInstance)
	defer cleanup()

	_, err := client.CountTokens(context.Background(), &token_countingv1.CountTokensRequest{
		Model: token_countingv1.Model_MODEL_GPT_5,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertStatusCode(t, err, codes.InvalidArgument)
}

func startTestServer(t *testing.T, tokenizerInstance *tokenizer.Tokenizer) (token_countingv1.TokenCountingServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	token_countingv1.RegisterTokenCountingServiceServer(grpcServer, server.New(tokenizerInstance, zap.NewNop()))

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

	client := token_countingv1.NewTokenCountingServiceClient(conn)
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
