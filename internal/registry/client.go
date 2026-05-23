package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const officialRegistryURL = "https://registry.terraform.io"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient() Client {
	return NewClientForBaseURL(officialRegistryURL)
}

func NewClientForBaseURL(baseURL string) Client {
	return Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c Client) doJSON(req *http.Request, target any) error {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return notFoundError{url: req.URL.String()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("registry request failed: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

type pagination struct {
	totalPages int
	itemCount  int
}

func (p pagination) isLast(pageNumber int, pageSize int) bool {
	if p.totalPages > 0 {
		return pageNumber >= p.totalPages
	}
	return p.itemCount < pageSize
}

type paginationMeta struct {
	Pagination struct {
		TotalPages int `json:"total-pages"`
	} `json:"pagination"`
}

type notFoundError struct {
	url string
}

func (e notFoundError) Error() string {
	return "registry resource not found: " + e.url
}

func isNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}
