package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gfaivre/ktools/internal/config"
	"github.com/gfaivre/ktools/internal/logging"
	"golang.org/x/time/rate"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	driveID    int
	limiter    *rate.Limiter
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    cfg.BaseURL,
		token:      cfg.APIToken,
		driveID:    cfg.DriveID,
		limiter:    rate.NewLimiter(rate.Limit(10), 20), // 10 req/s, burst 20
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	// Buffer le body pour pouvoir le relire en cas de retry
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("body read error: %w", err)
		}
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Rate limiting
		logging.Debug("waiting for rate limiter", "method", method, "path", path)
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
		logging.Debug("sending request", "method", method, "path", path, "attempt", attempt+1)

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		// Timeout per request (30s) - more reliable than http.Client timeout
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, method, url, reqBody)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("request creation error: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			cancel()
			// Check if it's a timeout
			if reqCtx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("request timeout after 30s: %s %s", method, path)
			}
			return nil, fmt.Errorf("HTTP request error: %w", err)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("response read error: %w", err)
		}

		// Retry sur 429 avec backoff exponentiel
		if resp.StatusCode == 429 {
			if attempt < maxRetries-1 {
				delay := time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			return nil, fmt.Errorf("API rate limited (429) after %d retries", maxRetries)
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
		}

		return data, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}

type APIResponse[T any] struct {
	Result string `json:"result"`
	Data   T      `json:"data"`
}

type File struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Visibility     string `json:"visibility"`
	DriveID        int    `json:"drive_id"`
	Depth          int    `json:"depth"`
	CreatedBy      int    `json:"created_by"`
	CreatedAt      int64  `json:"created_at"`
	AddedAt        int64  `json:"added_at"`
	LastModifiedAt int64  `json:"last_modified_at"`
	LastModifiedBy int    `json:"last_modified_by"`
	RevisedAt      int64  `json:"revised_at"`
	UpdatedAt      int64  `json:"updated_at"`
	ParentID       int    `json:"parent_id"`
	Color          string `json:"color,omitempty"`
}

func (c *Client) GetFile(ctx context.Context, fileID int) (*File, error) {
	path := fmt.Sprintf("/3/drive/%d/files/%d", c.driveID, fileID)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[File]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("API error: %s", resp.Result)
	}

	return &resp.Data, nil
}

type ListFilesResponse struct {
	Result     string `json:"result"`
	Data       []File `json:"data"`
	Cursor     string `json:"cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	ResponseAt int64  `json:"response_at"`
}

func (c *Client) ListFiles(ctx context.Context, fileID int) ([]File, error) {
	path := fmt.Sprintf("/3/drive/%d/files/%d/files", c.driveID, fileID)

	var allFiles []File
	cursor := ""

	for {
		reqPath := path
		if cursor != "" {
			reqPath = fmt.Sprintf("%s?cursor=%s", path, cursor)
		}

		data, err := c.doRequest(ctx, "GET", reqPath, nil)
		if err != nil {
			return nil, err
		}

		var resp ListFilesResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("JSON parse error: %w", err)
		}

		if resp.Result != "success" {
			return nil, fmt.Errorf("API error: %s", resp.Result)
		}

		allFiles = append(allFiles, resp.Data...)

		if !resp.HasMore {
			break
		}
		cursor = resp.Cursor
	}

	return allFiles, nil
}

// FindFileByPath searches for a file/directory by path from the root
func (c *Client) FindFileByPath(ctx context.Context, filePath string) (*File, error) {
	filePath = strings.Trim(filePath, "/")
	if filePath == "" {
		return c.GetFile(ctx, 1)
	}

	parts := strings.Split(filePath, "/")
	currentID := 1

	for _, part := range parts {
		files, err := c.ListFiles(ctx, currentID)
		if err != nil {
			return nil, err
		}

		found := false
		partLower := strings.ToLower(part)
		for _, f := range files {
			if strings.ToLower(f.Name) == partLower {
				currentID = f.ID
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("path not found: %s", part)
		}
	}

	return c.GetFile(ctx, currentID)
}

// ProgressCallback is called during recursive operations to report progress
type ProgressCallback func(dirName string, fileCount int)

// ListFilesRecursive lists all files in a directory and its subdirectories
// Uses a worker pool for parallel directory traversal
func (c *Client) ListFilesRecursive(ctx context.Context, fileID int) ([]File, error) {
	return c.ListFilesRecursiveWithProgress(ctx, fileID, nil)
}

// ListFilesRecursiveWithProgress lists all files with progress callback
func (c *Client) ListFilesRecursiveWithProgress(ctx context.Context, fileID int, progress ProgressCallback) ([]File, error) {
	const numWorkers = 5

	type job struct {
		dirID   int
		dirName string
	}
	type result struct {
		files   []File
		dirName string
		err     error
	}

	jobs := make(chan job, 100)
	results := make(chan result, 100)

	// Start workers - they will exit when context is cancelled
	for i := 0; i < numWorkers; i++ {
		go func() {
			for j := range jobs {
				// Check context before making request
				if ctx.Err() != nil {
					return
				}
				files, err := c.ListFiles(ctx, j.dirID)
				// Check context after request - don't send if cancelled
				if ctx.Err() != nil {
					return
				}
				select {
				case results <- result{files: files, dirName: j.dirName, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Track pending jobs
	pending := 1
	jobs <- job{dirID: fileID, dirName: "root"}

	var allFiles []File
	var firstErr error

	for pending > 0 {
		select {
		case <-ctx.Done():
			// Don't close channels - let goroutines exit naturally via ctx check
			return nil, ctx.Err()
		case r := <-results:
			pending--

			if r.err != nil {
				if firstErr == nil {
					firstErr = r.err
				}
				continue
			}

			allFiles = append(allFiles, r.files...)

			if progress != nil {
				progress(r.dirName, len(allFiles))
			}

			// Queue subdirectories
			for _, f := range r.files {
				if f.Type == "dir" {
					pending++
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case jobs <- job{dirID: f.ID, dirName: f.Name}:
					}
				}
			}
		}
	}

	close(jobs)

	if firstErr != nil {
		return nil, firstErr
	}

	return allFiles, nil
}

type Category struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	IsPredefined bool   `json:"is_predefined"`
	CreatedBy    int    `json:"created_by"`
	CreatedAt    int64  `json:"created_at"`
}

func (c *Client) ListCategories(ctx context.Context) ([]Category, error) {
	path := fmt.Sprintf("/2/drive/%d/categories", c.driveID)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Category]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("API error: %s", resp.Result)
	}

	return resp.Data, nil
}

type CategoryResult struct {
	ID     int  `json:"id"`
	Result bool `json:"result"`
}

func (c *Client) modifyCategory(ctx context.Context, method string, categoryID int, fileIDs []int) ([]CategoryResult, error) {
	path := fmt.Sprintf("/2/drive/%d/files/categories/%d", c.driveID, categoryID)

	body := struct {
		FileIDs []int `json:"file_ids"`
	}{FileIDs: fileIDs}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("JSON encoding error: %w", err)
	}

	data, err := c.doRequest(ctx, method, path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]CategoryResult]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("API error: %s", resp.Result)
	}

	return resp.Data, nil
}

func (c *Client) AddCategoryToFiles(ctx context.Context, categoryID int, fileIDs []int) ([]CategoryResult, error) {
	return c.modifyCategory(ctx, http.MethodPost, categoryID, fileIDs)
}

func (c *Client) RemoveCategoryFromFiles(ctx context.Context, categoryID int, fileIDs []int) ([]CategoryResult, error) {
	return c.modifyCategory(ctx, http.MethodDelete, categoryID, fileIDs)
}
