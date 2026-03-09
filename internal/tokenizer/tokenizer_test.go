package tokenizer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const smallPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGMAAQAABQABDQottAAAAABJRU5ErkJggg=="

func TestCountMessageTokensTextOnly(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello world"}]}`)
	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expected := 3 + tokensFor(t, count, "user") + tokensFor(t, count, "Hello world")
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func TestCountMessageTokensImageLowDetail(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := "data:image/png;base64," + smallPNGBase64
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"%s","detail":"low"}]}`,
		imageURL))

	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expected := 3 + tokensFor(t, count, "user") + imageTokenBase
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func TestCountMessageTokensImageHighDetail(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := "data:image/png;base64," + smallPNGBase64
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"%s","detail":"high"}]}`,
		imageURL))

	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expected := 3 + tokensFor(t, count, "user") + imageTokenBase + imageTokenPerTile
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func TestCountMessageTokensMixedContent(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := "data:image/png;base64," + smallPNGBase64
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"},{"type":"input_image","image_url":"%s","detail":"low"}]}`,
		imageURL))

	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expected := 3 + tokensFor(t, count, "user") + tokensFor(t, count, "Hello") + imageTokenBase
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func TestCountMessageTokensUnsupportedContentType(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"user","content":[{"type":"input_file"}]}`)
	_, err := count.CountMessageTokens(message)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported content type: input_file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountMessageTokensInvalidJSON(t *testing.T) {
	count := newTestTokenizer(t)

	_, err := count.CountMessageTokens([]byte("{invalid"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountMessageTokensIncludesOverhead(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}`)
	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expected := 3 + tokensFor(t, count, "assistant")
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func newTestTokenizer(t *testing.T) *Tokenizer {
	t.Helper()
	tokenizer, err := NewTokenizer()
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}
	return tokenizer
}

func tokensFor(t *testing.T, tokenizer *Tokenizer, text string) int {
	t.Helper()
	encoded := tokenizer.encoding.Encode(text, nil, nil)
	return len(encoded)
}
