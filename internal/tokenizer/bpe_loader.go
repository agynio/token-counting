package tokenizer

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

const bpeDownloadTimeout = 30 * time.Second

var bpeLoaderOnce sync.Once

func configureBpeLoader() {
	bpeLoaderOnce.Do(func() {
		client := &http.Client{Timeout: bpeDownloadTimeout}
		tiktoken.SetBpeLoader(&timeoutBpeLoader{client: client})
	})
}

type timeoutBpeLoader struct {
	client *http.Client
}

func (l *timeoutBpeLoader) LoadTiktokenBpe(path string) (map[string]int, error) {
	contents, err := readFileCached(path, l.client)
	if err != nil {
		return nil, err
	}
	return parseBpeContents(contents)
}

func readFileCached(path string, client *http.Client) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("bpe path cannot be empty")
	}
	cacheDir := strings.TrimSpace(os.Getenv("TIKTOKEN_CACHE_DIR"))
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("DATA_GYM_CACHE_DIR"))
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "data-gym-cache")
	}
	cacheKey := fmt.Sprintf("%x", sha1.Sum([]byte(path)))
	cachePath := filepath.Join(cacheDir, cacheKey)
	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}

	data, err := readFile(path, client)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	tmpPath := fmt.Sprintf("%s.tmp", cachePath)
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write cache file: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("rename cache file: %w", err)
	}
	return data, nil
}

func readFile(path string, client *http.Client) ([]byte, error) {
	trimmed := strings.TrimSpace(path)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Get(trimmed)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("download bpe: status %s", resp.Status)
		}
		return io.ReadAll(resp.Body)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func parseBpeContents(contents []byte) (map[string]int, error) {
	lines := strings.Split(string(contents), "\n")
	bpeRanks := make(map[string]int, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid bpe line: %q", line)
		}
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, err
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		bpeRanks[string(token)] = rank
	}
	return bpeRanks, nil
}
