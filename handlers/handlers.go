package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"tele_storage/config"
	"tele_storage/telegram"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	cfg    *config.Config
	tg     *telegram.Client
	store  *telegram.Store
	router *chi.Mux
}

func NewServer(cfg *config.Config, tg *telegram.Client, store *telegram.Store) *Server {
	s := &Server{
		cfg:    cfg,
		tg:     tg,
		store:  store,
		router: chi.NewRouter(),
	}

	s.setupRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) setupRoutes() {
	r := s.router

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Resource or endpoint not found: %s", r.URL.Path))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		respondError(w, http.StatusMethodNotAllowed, fmt.Sprintf("Method %s not allowed on %s", r.Method, r.URL.Path))
	})

	// API endpoints
	r.Route("/api", func(r chi.Router) {
		r.Post("/upload", s.handleUpload)
		r.Get("/download", s.handleDownloadFile)
		r.Get("/info", s.handleGetFileInfo)
		r.Delete("/messages/{message_id}", s.handleDeleteMessage)
	})

	// Public download shortcut endpoint
	r.Get("/download", s.handleDownloadFile)

	// Health check
	r.Get("/health", s.handleHealth)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// POST /api/upload - Upload file to Telegram
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	maxBytes := s.cfg.MaxUploadSizeMB * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("File exceeds maximum allowed size (%d MB) or invalid body", s.cfg.MaxUploadSizeMB))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Missing 'file' field in multipart form request")
		return
	}
	defer file.Close()

	// Detect content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Upload to Telegram
	tgMsg, err := s.tg.UploadDocument(header.Filename, file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to upload file to Telegram: %v", err))
		return
	}

	if tgMsg.Document == nil {
		respondError(w, http.StatusInternalServerError, "Telegram response did not return a valid document object")
		return
	}

	doc := tgMsg.Document
	fileSize := doc.FileSize
	if fileSize <= 0 {
		fileSize = header.Size
	}

	downloadURL := fmt.Sprintf("/api/download?file_id=%s&file_name=%s", doc.FileID, url.QueryEscape(header.Filename))
	now := time.Now()

	item := telegram.FileItem{
		FileID:       doc.FileID,
		FileUniqueID: doc.FileUniqueID,
		MessageID:    tgMsg.MessageID,
		FileName:     header.Filename,
		FileSize:     fileSize,
		MimeType:     contentType,
		CreatedAt:    now,
		DownloadURL:  downloadURL,
	}

	// Store in memory
	s.store.Add(item)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "success",
		"data":   item,
	})
}

// GET /api/download?file_id=...&file_name=... - Stream file directly from Telegram
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("file_id")
	if fileID == "" {
		respondError(w, http.StatusBadRequest, "Missing required query parameter 'file_id'")
		return
	}

	fileName := r.URL.Query().Get("file_name")
	if fileName == "" {
		if item, found := s.store.GetByFileID(fileID); found && item.FileName != "" {
			fileName = item.FileName
		}
	}

	// Fetch file path info from Telegram
	fileInfo, err := s.tg.GetFileInfo(fileID)
	if err != nil {
		respondError(w, http.StatusBadGateway, fmt.Sprintf("Telegram getFile error: %v", err))
		return
	}

	if fileName == "" && fileInfo.FilePath != "" {
		fileName = filepath.Base(fileInfo.FilePath)
	}
	if fileName == "" {
		fileName = "downloaded_file"
	}

	// Append file extension from Telegram's file_path if missing from fileName
	if filepath.Ext(fileName) == "" && fileInfo.FilePath != "" && filepath.Ext(fileInfo.FilePath) != "" {
		fileName += filepath.Ext(fileInfo.FilePath)
	}

	// Get stream from Telegram server
	stream, contentLength, err := s.tg.GetFileDownloadStream(fileInfo.FilePath)
	if err != nil {
		respondError(w, http.StatusBadGateway, fmt.Sprintf("Failed to fetch stream from Telegram: %v", err))
		return
	}
	defer stream.Close()

	// Set content type and disposition headers
	contentType := mime.TypeByExtension(filepath.Ext(fileName))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	} else if fileInfo.FileSize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.FileSize, 10))
	}

	disposition := "inline"
	if r.URL.Query().Get("dl") == "1" {
		disposition = "attachment"
	}
	escapedName := url.PathEscape(fileName)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, fileName, escapedName))

	// Stream to client
	_, _ = io.Copy(w, stream)
}

// GET /api/info?file_id=... - Get Telegram file metadata dynamically
func (s *Server) handleGetFileInfo(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("file_id")
	if fileID == "" {
		respondError(w, http.StatusBadRequest, "Missing required query parameter 'file_id'")
		return
	}

	info, err := s.tg.GetFileInfo(fileID)
	if err != nil {
		respondError(w, http.StatusBadGateway, fmt.Sprintf("Failed to get file info from Telegram: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"file_id":   info.FileID,
			"file_path": info.FilePath,
			"file_size": info.FileSize,
		},
	})
}

// DELETE /api/messages/{message_id} - Delete message on Telegram by ID
func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	msgIDStr := chi.URLParam(r, "message_id")
	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid message_id parameter")
		return
	}

	if err := s.tg.DeleteMessage(msgID); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete message on Telegram: %v", err))
		return
	}

	// Remove from in-memory store
	s.store.RemoveByMessageID(msgID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Telegram message %d deleted successfully", msgID),
	})
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, statusCode int, message string) {
	respondJSON(w, statusCode, map[string]interface{}{
		"status":  "error",
		"message": message,
	})
}
