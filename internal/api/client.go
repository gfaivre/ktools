package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gfaivre/ktools/internal/config"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	driveID    int
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    cfg.BaseURL,
		token:      cfg.APIToken,
		driveID:    cfg.DriveID,
	}
}

func (c *Client) doRequest(method, path string, body io.Reader) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("erreur création requête: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lecture réponse: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("erreur API (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
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

func (c *Client) GetFile(fileID int) (*File, error) {
	path := fmt.Sprintf("/3/drive/%d/files/%d", c.driveID, fileID)

	data, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[File]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erreur parsing JSON: %w", err)
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

func (c *Client) ListFiles(fileID int) ([]File, error) {
	path := fmt.Sprintf("/3/drive/%d/files/%d/files", c.driveID, fileID)

	var allFiles []File
	cursor := ""

	for {
		reqPath := path
		if cursor != "" {
			reqPath = fmt.Sprintf("%s?cursor=%s", path, cursor)
		}

		data, err := c.doRequest("GET", reqPath, nil)
		if err != nil {
			return nil, err
		}

		var resp ListFilesResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("erreur parsing JSON: %w", err)
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

// ListFilesRecursive liste tous les fichiers d'un répertoire et ses sous-répertoires
func (c *Client) ListFilesRecursive(fileID int) ([]File, error) {
	files, err := c.ListFiles(fileID)
	if err != nil {
		return nil, err
	}

	var allFiles []File
	for _, f := range files {
		allFiles = append(allFiles, f)
		if f.Type == "dir" {
			children, err := c.ListFilesRecursive(f.ID)
			if err != nil {
				return nil, err
			}
			allFiles = append(allFiles, children...)
		}
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

func (c *Client) ListCategories() ([]Category, error) {
	path := fmt.Sprintf("/2/drive/%d/categories", c.driveID)

	data, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Category]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erreur parsing JSON: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("API error: %s", resp.Result)
	}

	return resp.Data, nil
}

type AddCategoryResult struct {
	ID     int  `json:"id"`
	Result bool `json:"result"`
}

func (c *Client) AddCategoryToFiles(categoryID int, fileIDs []int) ([]AddCategoryResult, error) {
	path := fmt.Sprintf("/2/drive/%d/files/categories/%d", c.driveID, categoryID)

	body := struct {
		FileIDs []int `json:"file_ids"`
	}{FileIDs: fileIDs}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("erreur encoding JSON: %w", err)
	}

	data, err := c.doRequest("POST", path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]AddCategoryResult]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erreur parsing JSON: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("API error: %s", resp.Result)
	}

	return resp.Data, nil
}

func (c *Client) RemoveCategoryFromFiles(categoryID int, fileIDs []int) ([]AddCategoryResult, error) {
	path := fmt.Sprintf("/2/drive/%d/files/categories/%d", c.driveID, categoryID)

	body := struct {
		FileIDs []int `json:"file_ids"`
	}{FileIDs: fileIDs}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("erreur encoding JSON: %w", err)
	}

	data, err := c.doRequest("DELETE", path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]AddCategoryResult]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erreur parsing JSON: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("API error: %s", resp.Result)
	}

	return resp.Data, nil
}