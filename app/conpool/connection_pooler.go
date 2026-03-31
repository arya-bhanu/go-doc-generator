package conpool

import (
	"sync"
	"time"

	"google.golang.org/api/forms/v1"
)

const defaultPollInterval = 5 * time.Second

var (
	storeFormID  map[string]any
	mu           sync.RWMutex
	formsSvc     *forms.Service
	pollInterval = defaultPollInterval
)

// Init must be called once at application start-up (before StartPooler) to
// inject the Google Forms service client that the watcher uses when polling
// for responses.  The service may be backed by either OAuth user credentials
// or a service-account – the pooler is agnostic to which one is used.
func Init(svc *forms.Service) {
	formsSvc = svc
	storeFormID = make(map[string]any)
}

// StartPooler loads all existing form IDs from the database into the
// in-memory map and then spins up the background watcher goroutine.
func StartPooler() {
	FetchFormIDInit()
	go watchForms()
}
