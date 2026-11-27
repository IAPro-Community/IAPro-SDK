package providers

import (
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type RegistrationState string

const (
	InstalledUnregistered   RegistrationState = "INSTALLED_UNREGISTERED"
	RegistrationUnknown     RegistrationState = "REGISTRATION_UNKNOWN"
	PendingAuth             RegistrationState = "PENDING_AUTH"
	RegisteredAuthenticated RegistrationState = "REGISTERED_AUTHENTICATED"
	RegisteredAuthExpired   RegistrationState = "REGISTERED_AUTH_EXPIRED"
	UnmanagedEphemeral      RegistrationState = "UNMANAGED_EPHEMERAL"
)

type InstallationRecord struct {
	Provider   string            `json:"provider"`
	Binary     string            `json:"binary"`
	Version    string            `json:"version,omitempty"`
	Path       string            `json:"path,omitempty"`
	Installed  bool              `json:"installed"`
	DetectedAt time.Time         `json:"detected_at"`
	State      RegistrationState `json:"state"`
}

// InstallationStore is a host-owned persistence port. Save must be atomic.
// No installation or authentication is performed by the registry.
type InstallationStore interface {
	Load() ([]InstallationRecord, error)
	Save([]InstallationRecord) error
}

type InstallationRegistry struct {
	mu      sync.Mutex
	records map[string]InstallationRecord
	store   InstallationStore
	now     func() time.Time
}

func NewInstallationRegistry(store InstallationStore, now func() time.Time) (*InstallationRegistry, error) {
	if store == nil || now == nil {
		return nil, errors.New("installation store and clock required")
	}
	records, err := store.Load()
	if err != nil {
		return nil, err
	}
	r := &InstallationRegistry{store: store, now: now, records: make(map[string]InstallationRecord)}
	for _, rec := range records {
		r.records[rec.Provider] = rec
	}
	return r, nil
}

func listInstallations(records map[string]InstallationRecord) []InstallationRecord {
	out := make([]InstallationRecord, 0, len(records))
	for _, rec := range records {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

func (r *InstallationRegistry) List() []InstallationRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return listInstallations(r.records)
}

func (r *InstallationRegistry) Apply(detections map[string]Detection) ([]InstallationRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := make(map[string]InstallationRecord, len(r.records)+len(detections))
	for id, rec := range r.records {
		next[id] = rec
	}
	for id, det := range detections {
		id = NormalizeID(id)
		if id == "" {
			return nil, errors.New("provider ID required")
		}
		rec := next[id]
		rec.Provider, rec.DetectedAt, rec.Installed = id, r.now().UTC(), det.Installed
		if det.Installed {
			rec.Path, rec.Version = det.BinaryPath, det.Version
			rec.Binary = filepath.Base(det.BinaryPath)
			if det.BinaryPath == "" {
				rec.Binary = ""
			}
			if rec.State == "" {
				rec.State = InstalledUnregistered
			}
		} else {
			rec.Path, rec.Version, rec.Binary = "", "", ""
		}
		next[id] = rec
	}
	out := listInstallations(next)
	if err := r.store.Save(out); err != nil {
		return nil, err
	}
	r.records = next
	return out, nil
}
