package tokenizer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestCountTokensTextMessage(t *testing.T) {
	message := buildMessage(t, "user", []any{
		map[string]any{"type": "input_text", "text": "Hello"},
	})
	msg := parseMessage(t, message)
	counts := countTokens(t, []Message{msg})

	expected := int32(gpt5MessageOverheadTokens + tokenCountForText(t, "Hello"))
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

	expected := int32(gpt5MessageOverheadTokens + gpt5ImageTokens(2048, 1024))
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

	expected := int32(gpt5MessageOverheadTokens + gpt5ImageBaseTokens)
	if counts[0] != expected {
		t.Fatalf("expected %d tokens, got %d", expected, counts[0])
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

	expected := int32(gpt5MessageOverheadTokens + tokenCountForText(t, "Hello PDF") + pageImageTokens(types.Dim{Width: 612, Height: 792}))
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

	expected := int32(gpt5MessageOverheadTokens + tokenCountForText(t, "Hello") + gpt5ImageTokens(256, 256))
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

func tokenCountForText(t *testing.T, text string) int {
	enc, err := encodingForModel(ModelGPT5)
	if err != nil {
		t.Fatalf("load tokenizer: %v", err)
	}
	return len(enc.Encode(text, nil, nil))
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
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 6)
	writeObject := func(id int, content string) {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, content)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", width, height))
	writeObject(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	content := fmt.Sprintf("BT /F1 24 Tf 72 720 Td (%s) Tj ET", escapePDFString(text))
	streamData := content + "\n"
	stream := fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(streamData), streamData)
	writeObject(5, stream)
	startXref := buf.Len()
	buf.WriteString("xref\n0 6\n")
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i < 6; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	buf.WriteString("trailer\n")
	buf.WriteString("<< /Root 1 0 R /Size 6 >>\n")
	buf.WriteString("startxref\n")
	buf.WriteString(fmt.Sprintf("%d\n", startXref))
	buf.WriteString("%%EOF\n")
	return buf.Bytes()
}

func escapePDFString(text string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")
	return replacer.Replace(text)
}
