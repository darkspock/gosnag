package upload

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxUploadSize = 10 << 20 // 10 MB

// allowedMIME maps detected MIME types to safe file extensions.
var allowedMIME = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type Handler struct {
	uploadDir string
	baseURL   string
}

func NewHandler(uploadDir, baseURL string) *Handler {
	os.MkdirAll(uploadDir, 0755)
	return &Handler{uploadDir: uploadDir, baseURL: baseURL}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 10 MB)")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	// Read first 512 bytes to detect actual content type via magic bytes
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	head = head[:n]

	detected := http.DetectContentType(head)
	ext, ok := allowedMIME[detected]
	if !ok {
		writeError(w, http.StatusBadRequest, "only image files are allowed (png, jpg, gif, webp); detected: "+detected)
		return
	}

	// Seek back to start so we copy the full file
	file.Seek(0, io.SeekStart)

	// Generate safe filename (random, controlled extension)
	b := make([]byte, 16)
	rand.Read(b)
	filename := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), hex.EncodeToString(b), ext)

	dst, err := os.Create(filepath.Join(h.uploadDir, filename))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	url := h.baseURL + "/uploads/" + filename

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// ServeUploads returns an http.Handler that serves uploaded files with safe headers.
func ServeUploads(dir string) http.Handler {
	fs := http.Dir(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Force download for anything not obviously an image extension
		name := filepath.Base(r.URL.Path)
		lname := strings.ToLower(name)
		isImage := strings.HasSuffix(lname, ".png") || strings.HasSuffix(lname, ".jpg") ||
			strings.HasSuffix(lname, ".jpeg") || strings.HasSuffix(lname, ".gif") ||
			strings.HasSuffix(lname, ".webp")

		if !isImage {
			w.Header().Set("Content-Disposition", "attachment; filename="+name)
		}
		// Prevent MIME sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		http.FileServer(fs).ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
