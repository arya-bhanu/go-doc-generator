package conpool

import (
	"os"
	"strconv"
	"sync"
	"time"

	"google.golang.org/api/forms/v1"
)

func defaultPollInterval() time.Duration {
	if v := os.Getenv("POLL_INTERVAL_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return 5 * time.Second
}

var (
	storeFormID  map[string]any
	mu           sync.RWMutex
	formsSvc     *forms.Service
	pollInterval = defaultPollInterval()

	// responseHandler is called once per new (previously-unseen) form response.
	// Register it with SetResponseHandler before calling StartPooler.
	responseHandler func(formID string, qAndA []FormAnswer)

	// processedResponses tracks which response IDs have already been dispatched
	// to the handler, keyed by formID.  Guarded by mu.
	processedResponses map[string]map[string]struct{}
)

// Init must be called once at application start-up (before StartPooler) to
// inject the Google Forms service client that the watcher uses when polling
// for responses.  The service may be backed by either OAuth user credentials
// or a service-account – the pooler is agnostic to which one is used.
func Init(svc *forms.Service) {
	formsSvc = svc
	storeFormID = make(map[string]any)
	processedResponses = make(map[string]map[string]struct{})
}

// SetResponseHandler registers a callback that is invoked exactly once for
// every new form submission.  The callback is executed in its own goroutine,
// so it must be safe to call concurrently.  Call this before StartPooler.
func SetResponseHandler(fn func(formID string, qAndA []FormAnswer)) {
	responseHandler = fn
}

// StartPooler loads all existing form IDs from the database into the
// in-memory map and then spins up the background watcher goroutine.
func StartPooler() {
	FetchFormIDInit()
	go watchForms()
}
