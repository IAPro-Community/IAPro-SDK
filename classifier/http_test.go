package classifier

import (
	"testing"

	"github.com/IAPro-Community/IAPro-SDK/model"
)

func TestHTTPStatus(t *testing.T) {
	for status, want := range map[int]model.FailureKind{
		200: model.FailureNone, 401: model.FailureAuth, 403: model.FailureAuth,
		402: model.FailureQuota, 429: model.FailureRateLimit, 503: model.FailureProvider,
		529: model.FailureProvider, 400: model.FailureUnknown,
	} {
		if got := HTTPStatus(status); got != want {
			t.Errorf("HTTP %d: got %s want %s", status, got, want)
		}
	}
}
