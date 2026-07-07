package shared

import (
	"errors"
	"net/http"

	"github.com/cloudflare/cloudflare-go/v7"
)

// IsNotFoundError reports whether err is a Cloudflare API error with a
// 404 Not Found status code. It is used to detect resources that were
// deleted outside of Terraform.
func IsNotFoundError(err error) bool {
	var apierr *cloudflare.Error
	return errors.As(err, &apierr) && apierr.StatusCode == http.StatusNotFound
}
