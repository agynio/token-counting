package tokenizer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
	_ "github.com/pkoukk/tiktoken-go-loader"
	_ "golang.org/x/image/webp"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const (
	imageTokenBase     = 85
	imageTokenPerTile  = 170
	maxImageBytes      = 10 * 1024 * 1024
	imageFetchTimeout  = 30 * time.Second
	maxImageLongSide   = 2048
	maxImageShortSide  = 768
	imageTileDimension = 512
)

// InvalidArgumentError represents a validation failure when parsing a message.
type InvalidArgumentError struct {
	message string
	err     error
}

func (e *InvalidArgumentError) Error() string {
	if e.err == nil {
		return e.message
	}
	return fmt.Sprintf("%s: %v", e.message, e.err)
}

func (e *InvalidArgumentError) Unwrap() error {
	return e.err
}

// InternalError represents a non-recoverable token counting failure.
type InternalError struct {
	message string
	err     error
}

func (e *InternalError) Error() string {
	if e.err == nil {
		return e.message
	}
	return fmt.Sprintf("%s: %v", e.message, e.err)
}

func (e *InternalError) Unwrap() error {
	return e.err
}

// Tokenizer counts tokens for OpenAI Responses API messages.
type Tokenizer struct {
	encoding   *tiktoken.Tiktoken
	httpClient *http.Client
}

// NewTokenizer constructs a Tokenizer configured with the o200k_base encoding.
func NewTokenizer() (*Tokenizer, error) {
	encoding, err := tiktoken.GetEncoding("o200k_base")
	if err != nil {
		return nil, fmt.Errorf("load encoding: %w", err)
	}

	return &Tokenizer{
		encoding: encoding,
		httpClient: &http.Client{
			Timeout: imageFetchTimeout,
		},
	}, nil
}

// CountMessageTokens parses a JSON-encoded message and returns the token count.
func (t *Tokenizer) CountMessageTokens(messageJSON []byte) (int, error) {
	var message messagePayload
	if err := json.Unmarshal(messageJSON, &message); err != nil {
		return 0, invalidArgument("invalid JSON", err)
	}

	roleTokens := t.encoding.Encode(message.Role, nil, nil)

	total := 3 + len(roleTokens)
	for _, item := range message.Content {
		count, err := t.countContentTokens(item)
		if err != nil {
			return 0, err
		}
		total += count
	}

	return total, nil
}

type messagePayload struct {
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Detail   string          `json:"detail"`
	ImageURL json.RawMessage `json:"image_url"`
}

type imageURLPayload struct {
	URL    string `json:"url"`
	FileID string `json:"file_id"`
	Detail string `json:"detail"`
}

func (t *Tokenizer) countContentTokens(item contentItem) (int, error) {
	switch item.Type {
	case "input_text", "output_text":
		return t.countTextTokens(item.Text)
	case "input_image":
		return t.countImageTokens(item)
	case "input_file":
		return 0, invalidArgument("unsupported content type: input_file", nil)
	case "":
		return 0, invalidArgument("unsupported content type: ", nil)
	default:
		return 0, invalidArgument(fmt.Sprintf("unsupported content type: %s", item.Type), nil)
	}
}

func (t *Tokenizer) countTextTokens(text string) (int, error) {
	encoded := t.encoding.Encode(text, nil, nil)
	return len(encoded), nil
}

func (t *Tokenizer) countImageTokens(item contentItem) (int, error) {
	imageURL, fileID, detail, err := parseImageURL(item.ImageURL)
	if err != nil {
		return 0, err
	}
	if fileID != "" {
		return 0, invalidArgument("file_id images not supported", nil)
	}
	if imageURL == "" {
		return 0, invalidArgument("image_url required", nil)
	}

	resolvedDetail := normalizeDetail(item.Detail)
	if resolvedDetail == "" {
		resolvedDetail = normalizeDetail(detail)
	}
	if resolvedDetail == "" || resolvedDetail == "high" || resolvedDetail == "auto" {
		return t.countHighDetailImageTokens(imageURL)
	}
	if resolvedDetail == "low" {
		return imageTokenBase, nil
	}

	return 0, invalidArgument(fmt.Sprintf("unsupported image detail: %s", resolvedDetail), nil)
}

func (t *Tokenizer) countHighDetailImageTokens(imageURL string) (int, error) {
	width, height, err := t.resolveImageDimensions(imageURL)
	if err != nil {
		return 0, err
	}

	scaledWidth, scaledHeight := scaleImageDimensions(width, height)
	tilesWide := int(math.Ceil(scaledWidth / imageTileDimension))
	tilesHigh := int(math.Ceil(scaledHeight / imageTileDimension))
	if tilesWide <= 0 || tilesHigh <= 0 {
		return 0, invalidArgument("failed to decode image", nil)
	}

	tileCount := tilesWide * tilesHigh
	return imageTokenBase + (imageTokenPerTile * tileCount), nil
}

func (t *Tokenizer) resolveImageDimensions(imageURL string) (int, int, error) {
	if strings.HasPrefix(imageURL, "data:") {
		return decodeDataURLDimensions(imageURL)
	}

	parsed, err := url.Parse(imageURL)
	if err != nil {
		return 0, 0, invalidArgument("invalid image URL", err)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return t.fetchImageDimensions(imageURL)
	case "":
		return 0, 0, invalidArgument("invalid image URL", nil)
	default:
		return 0, 0, invalidArgument(fmt.Sprintf("unsupported image URL scheme: %s", parsed.Scheme), nil)
	}
}

func (t *Tokenizer) fetchImageDimensions(imageURL string) (int, int, error) {
	resp, err := t.httpClient.Get(imageURL)
	if err != nil {
		return 0, 0, internalError("failed to fetch image", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, 0, internalError(fmt.Sprintf("failed to fetch image: status %d", resp.StatusCode), nil)
	}

	data, err := readLimited(resp.Body, maxImageBytes)
	if err != nil {
		return 0, 0, internalError("failed to fetch image", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, invalidArgument("failed to decode image", err)
	}

	return cfg.Width, cfg.Height, nil
}

func decodeDataURLDimensions(imageURL string) (int, int, error) {
	comma := strings.Index(imageURL, ",")
	if comma == -1 {
		return 0, 0, invalidArgument("failed to decode image", nil)
	}
	meta := imageURL[:comma]
	data := imageURL[comma+1:]
	if !strings.Contains(meta, ";base64") {
		return 0, 0, invalidArgument("failed to decode image", nil)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return 0, 0, invalidArgument("failed to decode image", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil {
		return 0, 0, invalidArgument("failed to decode image", err)
	}

	return cfg.Width, cfg.Height, nil
}

func parseImageURL(raw json.RawMessage) (string, string, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", "", "", invalidArgument("image_url required", nil)
	}

	var urlValue string
	if err := json.Unmarshal(raw, &urlValue); err == nil {
		return urlValue, "", "", nil
	}

	var payload imageURLPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", "", invalidArgument("invalid image URL", err)
	}

	return payload.URL, payload.FileID, payload.Detail, nil
}

func normalizeDetail(detail string) string {
	return strings.ToLower(strings.TrimSpace(detail))
}

func scaleImageDimensions(width, height int) (float64, float64) {
	w := float64(width)
	h := float64(height)
	longSide := math.Max(w, h)
	if longSide > maxImageLongSide {
		scale := maxImageLongSide / longSide
		w *= scale
		h *= scale
	}
	shortSide := math.Min(w, h)
	if shortSide > maxImageShortSide {
		scale := maxImageShortSide / shortSide
		w *= scale
		h *= scale
	}
	return w, h
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("image exceeds %d bytes", limit)
	}
	return data, nil
}

func invalidArgument(message string, err error) error {
	return &InvalidArgumentError{message: message, err: err}
}

func internalError(message string, err error) error {
	return &InternalError{message: message, err: err}
}
