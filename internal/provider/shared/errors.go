package shared

import (
	"errors"
	"net/http"

	"github.com/cloudflare/cloudflare-go/v7"
)

// HTTPStatusCoder is implemented by errors that carry an HTTP status code.
// Custom API clients (e.g. those not based on the cloudflare-go SDK) can
// implement this interface so their errors are recognized by
// IsNotFoundError alongside *cloudflare.Error.
type HTTPStatusCoder interface {
	HTTPStatusCode() int
}

// IsNotFoundError reports whether err is an API error with a 404 Not Found
// status code. It recognizes both *cloudflare.Error (from the cloudflare-go
// SDK) and any error implementing HTTPStatusCoder. It is used to detect
// resources that were deleted outside of Terraform.
func IsNotFoundError(err error) bool {
	var apierr *cloudflare.Error
	if errors.As(err, &apierr) && apierr.StatusCode == http.StatusNotFound {
		return true
	}
	var coder HTTPStatusCoder
	if errors.As(err, &coder) && coder.HTTPStatusCode() == http.StatusNotFound {
		return true
	}
	return false
}
