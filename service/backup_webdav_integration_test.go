package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWebDAVClientUploadAndRetention(t *testing.T) {
	var mu sync.Mutex
	files := map[string][]byte{}
	modTimes := map[string]time.Time{}
	putCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case http.MethodPut:
			putCount++
			if putCount == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			body, _ := io.ReadAll(r.Body)
			files[r.URL.Path] = body
			modTimes[r.URL.Path] = time.Now().Add(-48 * time.Hour)
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			delete(files, r.URL.Path)
			delete(modTimes, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(207)
			items := make([]string, 0, len(files))
			for k := range files {
				items = append(items, k)
			}
			xmlBody := `<multistatus xmlns="DAV:">`
			for _, item := range items {
				xmlBody += `<response><href>` + item + `</href><propstat><prop><getcontentlength>` + strconv.Itoa(len(files[item])) + `</getcontentlength><getlastmodified>` + modTimes[item].UTC().Format(time.RFC1123) + `</getlastmodified><resourcetype></resourcetype></prop></propstat></response>`
			}
			xmlBody += `</multistatus>`
			_, _ = w.Write([]byte(xmlBody))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	cfg := BackupConfig{WebDAVURL: server.URL, WebDAVBasePath: "/backup", RetentionDays: 1, RetentionMaxFiles: 1}
	client, err := newWebDAVClient(cfg)
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if err := client.ensureCollection(cfg.WebDAVBasePath); err != nil {
		t.Fatalf("mkcol failed: %v", err)
	}

	payload := []byte("hello")
	tmpFile, err := os.CreateTemp(t.TempDir(), "dav-payload-*.bin")
	if err != nil {
		t.Fatalf("create temp payload failed: %v", err)
	}
	if _, err := tmpFile.Write(payload); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	_ = tmpFile.Close()

	remotePath := path.Join(cfg.WebDAVBasePath, "a.bin")
	if err := client.uploadFile(tmpFile.Name(), remotePath); err == nil {
		t.Fatalf("expected first upload to fail due to simulated intermittent failure")
	}
	if err := client.uploadFile(tmpFile.Name(), remotePath); err != nil {
		t.Fatalf("second upload should succeed: %v", err)
	}

	if err := applyWebDAVRetention(client, cfg.WebDAVBasePath, cfg.RetentionDays, cfg.RetentionMaxFiles); err != nil {
		t.Fatalf("retention failed: %v", err)
	}
}

func TestWebDAVFailureModes(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer authServer.Close()
	cfg := BackupConfig{WebDAVURL: authServer.URL}
	client, err := newWebDAVClient(cfg)
	if err != nil {
		t.Fatalf("create auth client failed: %v", err)
	}
	if err := client.ensureCollection("/backup"); err == nil {
		t.Fatalf("expected unauthorized error")
	}

	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer timeoutServer.Close()
	timeoutClient, err := newWebDAVClient(BackupConfig{WebDAVURL: timeoutServer.URL})
	if err != nil {
		t.Fatalf("create timeout client failed: %v", err)
	}
	timeoutClient.httpClient.Timeout = 50 * time.Millisecond
	if err := timeoutClient.ensureCollection("/backup"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}
