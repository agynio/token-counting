//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	tokencountingv1 "github.com/agynio/token-counting/.gen/go/agynio/api/token_counting/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCountTokensE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(tokenCountingAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("create token-counting client: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	client := tokencountingv1.NewTokenCountingServiceClient(conn)

	messages := [][]byte{
		[]byte(`{"type":"message","role":"system","content":[{"type":"input_text","text":"You are a helpful assistant."}]}`),
		[]byte(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}`),
	}

	resp, err := client.CountTokens(ctx, &tokencountingv1.CountTokensRequest{
		Model:    tokencountingv1.TokenCountingModel_TOKEN_COUNTING_MODEL_GPT_5,
		Messages: messages,
	})
	if err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if len(resp.Tokens) != len(messages) {
		t.Fatalf("expected %d tokens, got %d", len(messages), len(resp.Tokens))
	}
	expected := []int32{12, 4}
	for i, token := range resp.Tokens {
		if token != expected[i] {
			t.Fatalf("expected token count %d at %d, got %d", expected[i], i, token)
		}
	}
}
