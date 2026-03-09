package tokenizer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
)

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

func TestCountMessageTokensImageLowDetailObjectForm(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := dataURLForPNG(t, 1, 1)
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":{"url":"%s","detail":"low"}}]}`,
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

func TestCountMessageTokensImageHighDetailDefault(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := dataURLForPNG(t, 1, 1)
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"%s"}]}`,
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

func TestCountMessageTokensHTTPImage(t *testing.T) {
	imageBytes := pngBytes(t, 1, 1)
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" {
			return nil, fmt.Errorf("unexpected scheme: %s", req.URL.Scheme)
		}
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(bytes.NewReader(imageBytes)),
			ContentLength: int64(len(imageBytes)),
			Header:        http.Header{"Content-Type": []string{"image/png"}},
		}, nil
	})}

	count := newTestTokenizer(t, WithHTTPClient(client))
	message := []byte(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":{"url":"https://example.com/image.png","detail":"high"}}]}`)

	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expected := 3 + tokensFor(t, count, "user") + imageTokenBase + imageTokenPerTile
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func TestCountMessageTokensMultiTileImage(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := dataURLForPNG(t, 2000, 1500)
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"%s"}]}`,
		imageURL))

	got, err := count.CountMessageTokens(message)
	if err != nil {
		t.Fatalf("CountMessageTokens returned error: %v", err)
	}

	expectedTiles := 4
	expected := 3 + tokensFor(t, count, "user") + imageTokenBase + (imageTokenPerTile * expectedTiles)
	if got != expected {
		t.Fatalf("expected %d tokens, got %d", expected, got)
	}
}

func TestCountMessageTokensMixedContent(t *testing.T) {
	count := newTestTokenizer(t)

	imageURL := dataURLForPNG(t, 1, 1)
	message := []byte(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"},{"type":"input_image","image_url":{"url":"%s","detail":"low"}}]}`,
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

func TestCountMessageTokensFileIDRejected(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"user","content":[{"type":"input_image","image_url":{"file_id":"file_123"}}]}`)
	_, err := count.CountMessageTokens(message)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %v", err)
	}
	if !strings.Contains(err.Error(), "file_id images not supported") {
		t.Fatalf("unexpected error: %v", err)
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

func TestCountMessageTokensEmptyContentType(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"user","content":[{"type":""}]}`)
	_, err := count.CountMessageTokens(message)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %v", err)
	}
	if !strings.Contains(err.Error(), "content type is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountMessageTokensMissingRole(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"","content":[{"type":"input_text","text":"Hello"}]}`)
	_, err := count.CountMessageTokens(message)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %v", err)
	}
	if !strings.Contains(err.Error(), "role is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountMessageTokensMissingContent(t *testing.T) {
	count := newTestTokenizer(t)

	message := []byte(`{"type":"message","role":"user","content":[]}`)
	_, err := count.CountMessageTokens(message)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var invalidErr *InvalidArgumentError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected InvalidArgumentError, got %v", err)
	}
	if !strings.Contains(err.Error(), "content is required") {
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestTokenizer(t *testing.T, opts ...Option) *Tokenizer {
	t.Helper()
	tokenizer, err := NewTokenizer(opts...)
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

func dataURLForPNG(t *testing.T, width, height int) string {
	encoded := base64.StdEncoding.EncodeToString(pngBytes(t, width, height))
	return "data:image/png;base64," + encoded
}

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}
	return buffer.Bytes()
}
