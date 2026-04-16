package core

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type TaskHistory struct {
	// Map relative path -> Transfer Time
	Records map[string]time.Time `json:"records"`
	mu      sync.RWMutex
}

type HistoryManager struct {
	// TaskName -> History
	Tasks map[string]*TaskHistory `json:"tasks"`
	Path  string
	mu    sync.RWMutex
}

func NewHistoryManager(path string) *HistoryManager {
	return &HistoryManager{
		Tasks: make(map[string]*TaskHistory),
		Path:  path,
	}
}

func (hm *HistoryManager) Load() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	data, err := os.ReadFile(hm.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &hm.Tasks)
}

func (hm *HistoryManager) Save() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	data, err := json.MarshalIndent(hm.Tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(hm.Path, data, 0644)
}

func (hm *HistoryManager) GetTaskHistory(taskName string) *TaskHistory {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, ok := hm.Tasks[taskName]; !ok {
		hm.Tasks[taskName] = &TaskHistory{
			Records: make(map[string]time.Time),
		}
	}
	return hm.Tasks[taskName]
}

func (th *TaskHistory) Add(path string) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.Records[path] = time.Now()
}

func (th *TaskHistory) Has(path string) bool {
	th.mu.RLock()
	defer th.mu.RUnlock()
	_, ok := th.Records[path]
	return ok
}

func (th *TaskHistory) GetTransferTime(path string) (time.Time, bool) {
	th.mu.RLock()
	defer th.mu.RUnlock()
	t, ok := th.Records[path]
	return t, ok
}

func (th *TaskHistory) Remove(path string) {
	th.mu.Lock()
	defer th.mu.Unlock()
	delete(th.Records, path)
}

// MarshalJSON implements json.Marshaler for concurrent-safe JSON serialization.
// Without this, json.MarshalIndent in Save() would read Records without holding
// the lock, racing with Add()/Remove() in other goroutines.
func (th *TaskHistory) MarshalJSON() ([]byte, error) {
	th.mu.RLock()
	defer th.mu.RUnlock()
	return json.Marshal(struct {
		Records map[string]time.Time `json:"records"`
	}{
		Records: th.Records,
	})
}

// UnmarshalJSON implements json.Unmarshaler for concurrent-safe JSON deserialization.
func (th *TaskHistory) UnmarshalJSON(data []byte) error {
	th.mu.Lock()
	defer th.mu.Unlock()
	var aux struct {
		Records map[string]time.Time `json:"records"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	th.Records = aux.Records
	if th.Records == nil {
		th.Records = make(map[string]time.Time)
	}
	return nil
}
