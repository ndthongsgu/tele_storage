package telegram

import (
	"fmt"
	"net/url"
	"sync"
	"time"
)

type FileItem struct {
	FileID       string    `json:"file_id"`
	FileUniqueID string    `json:"file_unique_id,omitempty"`
	MessageID    int64     `json:"message_id"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
	DownloadURL  string    `json:"download_url"`
}

type PaginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

type Store struct {
	mu    sync.RWMutex
	files []FileItem
}

func NewStore() *Store {
	return &Store{
		files: make([]FileItem, 0),
	}
}

func (s *Store) GetByFileID(fileID string) (FileItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, f := range s.files {
		if f.FileID == fileID {
			return f, true
		}
	}
	return FileItem{}, false
}

func (s *Store) Add(item FileItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, f := range s.files {
		if (item.FileID != "" && f.FileID == item.FileID) || (item.MessageID > 0 && f.MessageID == item.MessageID) {
			s.files[i] = item
			return
		}
	}

	// Prepend newest first
	s.files = append([]FileItem{item}, s.files...)
}

func (s *Store) RemoveByMessageID(msgID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	newFiles := make([]FileItem, 0, len(s.files))
	removed := false
	for _, f := range s.files {
		if f.MessageID == msgID {
			removed = true
			continue
		}
		newFiles = append(newFiles, f)
	}
	s.files = newFiles
	return removed
}

func (s *Store) GetPaginated(page, limit int) ([]FileItem, PaginationMeta) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	totalItems := len(s.files)
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}

	start := (page - 1) * limit
	if start > totalItems {
		start = totalItems
	}
	end := start + limit
	if end > totalItems {
		end = totalItems
	}

	items := make([]FileItem, 0)
	if start < totalItems {
		items = s.files[start:end]
	}

	meta := PaginationMeta{
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return items, meta
}

// SyncFromTelegramInBg periodically checks Telegram updates to populate memory store with documents
func (s *Store) SyncFromTelegram(tg *Client) {
	updates, err := tg.GetUpdates(0, 100)
	if err != nil {
		return
	}

	for _, u := range updates {
		msg := u.Message
		if msg == nil {
			msg = u.ChannelPost
		}
		if msg != nil && msg.Document != nil {
			doc := msg.Document
			date := time.Now()
			if msg.Date > 0 {
				date = time.Unix(msg.Date, 0)
			}
			dlURL := fmt.Sprintf("/api/download?file_id=%s", doc.FileID)
			if doc.FileName != "" {
				dlURL = fmt.Sprintf("/api/download?file_id=%s&file_name=%s", doc.FileID, url.QueryEscape(doc.FileName))
			}
			s.Add(FileItem{
				FileID:       doc.FileID,
				FileUniqueID: doc.FileUniqueID,
				MessageID:    msg.MessageID,
				FileName:     doc.FileName,
				FileSize:     doc.FileSize,
				MimeType:     doc.MimeType,
				CreatedAt:    date,
				DownloadURL:  dlURL,
			})
		}
	}
}
