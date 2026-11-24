package classifier

import (
	"net/http"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

// HTTPStatus classifies a trusted HTTP response code without reading model
// output or error bodies. A classification alone never authorizes a retry.
func HTTPStatus(status int) model.FailureKind {
	switch {
	case status >= 200 && status < 300:
		return model.FailureNone
	case status == http.StatusTooManyRequests:
		return model.FailureRateLimit
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return model.FailureAuth
	case status == http.StatusPaymentRequired:
		return model.FailureQuota
	case status >= 500 && status <= 599:
		return model.FailureProvider
	default:
		return model.FailureUnknown
	}
}
