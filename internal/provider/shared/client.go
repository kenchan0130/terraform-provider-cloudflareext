package shared

import (
	cloudflare "github.com/cloudflare/cloudflare-go/v4"
)

// CloudflareClient wraps the official cloudflare-go SDK client with the account ID.
type CloudflareClient struct {
	Client    *cloudflare.Client
	AccountID string
}

// CloudflareResponse represents the standard Cloudflare API response envelope.
// Used in tests to construct mock HTTP responses.
type CloudflareResponse[T any] struct {
	Success    bool              `json:"success"`
	Errors     []CloudflareError `json:"errors"`
	Messages   []any             `json:"messages"`
	Result     T                 `json:"result"`
	ResultInfo *ResultInfo       `json:"result_info,omitempty"`
}

// CloudflareError represents an error in a Cloudflare API response.
type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ResultInfo represents pagination metadata in a Cloudflare API response.
type ResultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
}
