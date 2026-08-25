package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

type Client struct {
	botToken string
	chatID   string
	hc       *http.Client
}

func NewClient(botToken, chatID string) *Client {
	return &Client{
		botToken: botToken,
		chatID:   chatID,
		hc: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

type TelegramDocument struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
}

type TelegramMessage struct {
	MessageID int64             `json:"message_id"`
	Date      int64             `json:"date,omitempty"`
	Document  *TelegramDocument `json:"document,omitempty"`
}

type SendDocumentResponse struct {
	OK          bool             `json:"ok"`
	Description string           `json:"description,omitempty"`
	Result      *TelegramMessage `json:"result,omitempty"`
}

type GetFileResult struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

type GetFileResponse struct {
	OK          bool           `json:"ok"`
	Description string         `json:"description,omitempty"`
	Result      *GetFileResult `json:"result,omitempty"`
}

type DeleteMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      bool   `json:"result,omitempty"`
}

type Update struct {
	UpdateID    int64            `json:"update_id"`
	Message     *TelegramMessage `json:"message,omitempty"`
	ChannelPost *TelegramMessage `json:"channel_post,omitempty"`
}

type GetUpdatesResponse struct {
	OK          bool     `json:"ok"`
	Description string   `json:"description,omitempty"`
	Result      []Update `json:"result,omitempty"`
}

// UploadDocument uploads a file stream to Telegram using sendDocument API
func (c *Client) UploadDocument(fileName string, reader io.Reader) (*TelegramMessage, error) {
	if c.botToken == "" || c.chatID == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("chat_id", c.chatID); err != nil {
		return nil, fmt.Errorf("failed to write chat_id field: %w", err)
	}

	part, err := writer.CreateFormFile("document", filepath.Base(fileName))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("failed to copy file reader to form: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", c.botToken)
	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read telegram response: %w", err)
	}

	var tgResp SendDocumentResponse
	if err := json.Unmarshal(respBody, &tgResp); err != nil {
		return nil, fmt.Errorf("failed to parse telegram response JSON: %w", err)
	}

	if !tgResp.OK || tgResp.Result == nil {
		return nil, fmt.Errorf("telegram upload error: %s", tgResp.Description)
	}

	return tgResp.Result, nil
}

// GetFileInfo retrieves the file info (file_path, file_size) on Telegram servers for a file_id
func (c *Client) GetFileInfo(fileID string) (*GetFileResult, error) {
	if c.botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", c.botToken, url.QueryEscape(fileID))
	resp, err := c.hc.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call getFile API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read getFile response: %w", err)
	}

	var tgResp GetFileResponse
	if err := json.Unmarshal(body, &tgResp); err != nil {
		return nil, fmt.Errorf("failed to parse getFile JSON: %w", err)
	}

	if !tgResp.OK || tgResp.Result == nil {
		return nil, fmt.Errorf("telegram getFile error: %s", tgResp.Description)
	}

	return tgResp.Result, nil
}

// GetFileDownloadStream opens an HTTP stream to download a file from Telegram using its file_path
func (c *Client) GetFileDownloadStream(filePath string) (io.ReadCloser, int64, error) {
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", c.botToken, filePath)
	resp, err := c.hc.Get(downloadURL)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to initiate file download from telegram: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("telegram file download HTTP error status: %s", resp.Status)
	}

	return resp.Body, resp.ContentLength, nil
}

// DeleteMessage deletes a message posted in the Telegram channel/chat by message_id
func (c *Client) DeleteMessage(messageID int64) error {
	if c.botToken == "" || c.chatID == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", c.botToken)
	payload := map[string]interface{}{
		"chat_id":    c.chatID,
		"message_id": messageID,
	}

	jsonBytes, _ := json.Marshal(payload)
	resp, err := c.hc.Post(apiURL, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to call deleteMessage API: %w", err)
	}
	defer resp.Body.Close()

	var tgResp DeleteMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("failed to decode deleteMessage response: %w", err)
	}

	if !tgResp.OK {
		return fmt.Errorf("telegram deleteMessage response error: %s", tgResp.Description)
	}

	return nil
}

// GetUpdates fetches recent updates from Telegram to discover existing uploaded files
func (c *Client) GetUpdates(offset int64, limit int) ([]Update, error) {
	if c.botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&limit=%d", c.botToken, offset, limit)
	resp, err := c.hc.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call getUpdates API: %w", err)
	}
	defer resp.Body.Close()

	var tgResp GetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return nil, fmt.Errorf("failed to decode getUpdates response: %w", err)
	}

	if !tgResp.OK {
		return nil, fmt.Errorf("telegram getUpdates error: %s", tgResp.Description)
	}

	return tgResp.Result, nil
}
