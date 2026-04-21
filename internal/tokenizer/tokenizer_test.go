package tokenizer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	path := filepath.Join("testdata", "o200k_base.tiktoken")
	_ = os.Setenv(bpePathEnv, path)
	os.Exit(m.Run())
}

func TestCountTokensTextMessage(t *testing.T) {
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_text", "text": "Hello"},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(4)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestCountTokensImageMessage(t *testing.T) {
	imageURL := buildPNGDataURL(t, 2048, 1024)
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_image", "image_url": imageURL},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(913)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestCountTokensImageDetailLow(t *testing.T) {
	imageURL := buildPNGDataURL(t, 512, 512)
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_image", "image_url": imageURL, "detail": "low"},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(73)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestParseImageDetailInvalid(t *testing.T) {
	imageURL := buildPNGDataURL(t, 512, 512)
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_image", "image_url": imageURL, "detail": "medium"},
	})
	if _, err := ParseMessage(message); err == nil {
		t.Fatalf("expected error for unsupported image detail")
	}
}

func TestCountTokensFilePDF(t *testing.T) {
	pdfBytes := buildSimplePDF(t, "Hello PDF", 612, 792)
	fileData := base64.StdEncoding.EncodeToString(pdfBytes)
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_file", "filename": "sample.pdf", "file_data": fileData},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(635)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestCountTokensFilePDFMultiplePages(t *testing.T) {
	pdfBytes := buildPDFWithPages(t, []string{"Hello PDF", "Second Page"}, 612, 792)
	fileData := base64.StdEncoding.EncodeToString(pdfBytes)
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_file", "filename": "sample.pdf", "file_data": fileData},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(1267)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestCountTokensFunctionCallOutputArray(t *testing.T) {
	imageURL := buildPNGDataURL(t, 256, 256)
	output := []any{
		map[string]any{"type": "input_text", "text": "Hello"},
		map[string]any{"type": "input_image", "image_url": imageURL},
	}
	payload := map[string]any{
		"type":   "function_call_output",
		"output": output,
	}
	data := marshalJSON(t, payload)
	msg := parseMessage(t, data)
	counts := countTokens(t, []Message{msg})

	expected := int32(634)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestCountTokensAudioFile(t *testing.T) {
	fileData := base64.StdEncoding.EncodeToString([]byte("audio"))
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_file", "filename": "sound.mp3", "file_data": fileData},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(gpt5MessageOverheadTokens)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
	}
}

func TestCountTokensUnknownFileExtension(t *testing.T) {
	fileData := base64.StdEncoding.EncodeToString([]byte("binary"))
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_file", "filename": "blob.bin", "file_data": fileData},
	})
	msg := parseMessage(t, message)
	_, err := CountTokens(context.Background(), ModelGPT5, []Message{msg})
	if err == nil {
		t.Fatalf("expected error for unsupported extension")
	}
}

func TestGPT5ImageTokensGolden(t *testing.T) {
	if got := gpt5ImageTokens(2048, 1024); got != 910 {
		t.Fatalf("expected 910 tokens for 2048x1024, got %d", got)
	}
	if got := gpt5ImageTokens(256, 256); got != 630 {
		t.Fatalf("expected 630 tokens for 256x256, got %d", got)
	}
}

func TestExtractTextFromContent(t *testing.T) {
	content := []byte("BT /F1 12 Tf 72 720 Td (Hello) Tj 10 0 Td (World) Tj [(PDF ) 120 <54657374>] TJ ET")
	text, err := extractTextFromContent(content)
	if err != nil {
		t.Fatalf("extract text: %v", err)
	}
	if text != "Hello World PDF Test" {
		t.Fatalf("expected extracted text to be %q, got %q", "Hello World PDF Test", text)
	}
}

func buildMessage(t *testing.T, role string, content []any) []byte {
	payload := map[string]any{
		"type":    "message",
		"role":    role,
		"content": content,
	}
	return marshalJSON(t, payload)
}

func marshalJSON(t *testing.T, payload any) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func parseMessage(t *testing.T, payload []byte) Message {
	msg, err := ParseMessage(payload)
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	return msg
}

func countTokens(t *testing.T, messages []Message) []int32 {
	counts, err := CountTokens(context.Background(), ModelGPT5, messages)
	if err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	return counts
}

func buildPNGDataURL(t *testing.T, width, height int) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return fmt.Sprintf("data:image/png;base64,%s", encoded)
}

func buildSimplePDF(t *testing.T, text string, width, height float64) []byte {
	return buildPDFWithPages(t, []string{text}, width, height)
}

func buildPDFWithPages(t *testing.T, texts []string, width, height float64) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	pageCount := len(texts)
	if pageCount == 0 {
		t.Fatalf("at least one page is required")
	}
	pageIDs := make([]int, pageCount)
	contentIDs := make([]int, pageCount)
	objectID := 4
	for i := 0; i < pageCount; i++ {
		pageIDs[i] = objectID
		objectID++
		contentIDs[i] = objectID
		objectID++
	}
	totalObjects := objectID - 1
	offsets := make([]int, totalObjects+1)
	writeObject := func(id int, content string) {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, content)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", joinObjectRefs(pageIDs), pageCount))
	writeObject(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i, text := range texts {
		pageID := pageIDs[i]
		contentID := contentIDs[i]
		page := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", width, height, contentID)
		writeObject(pageID, page)
		streamData := pdfContentStream(text)
		stream := fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(streamData), streamData)
		writeObject(contentID, stream)
	}
	startXref := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", totalObjects+1))
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= totalObjects; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	buf.WriteString("trailer\n")
	buf.WriteString(fmt.Sprintf("<< /Root 1 0 R /Size %d >>\n", totalObjects+1))
	buf.WriteString("startxref\n")
	buf.WriteString(fmt.Sprintf("%d\n", startXref))
	buf.WriteString("%%EOF\n")
	return buf.Bytes()
}

func pdfContentStream(text string) string {
	content := fmt.Sprintf("BT /F1 24 Tf 72 720 Td (%s) Tj ET", escapePDFString(text))
	return content + "\n"
}

func joinObjectRefs(ids []int) string {
	refs := make([]string, len(ids))
	for i, id := range ids {
		refs[i] = fmt.Sprintf("%d 0 R", id)
	}
	return strings.Join(refs, " ")
}

func escapePDFString(text string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")
	return replacer.Replace(text)
}
