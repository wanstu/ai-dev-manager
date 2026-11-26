package memory

import (
	"ai-dev-manager-v2/internal/model"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

func validity(m model.MemoryMetadata, now time.Time) string {
	switch m.Status {
	case "obsolete", "archived":
		return m.Status
	}
	if m.ValidUntil != nil && !now.Before(*m.ValidUntil) {
		return "expired"
	}
	if m.ReviewAfter != nil && !now.Before(*m.ReviewAfter) {
		return "review_due"
	}
	return "active"
}
func recallable(status string) bool { return status == "active" || status == "review_due" }

// ReviewQueue exposes retained records requiring review, not deleted keys.
// Its output is bounded and does not contain full memory values.
type ReviewItem struct {
	Key      string                `json:"key"`
	Revision int64                 `json:"revision"`
	Validity string                `json:"validity"`
	Metadata *model.MemoryMetadata `json:"metadata,omitempty"`
}
type ReviewResult struct {
	Items   []ReviewItem `json:"items"`
	Total   int          `json:"total_matches"`
	Omitted int          `json:"omitted"`
}

func (s *Service) ReviewQueue(id string, global bool, status string, limit int) (ReviewResult, error) {
	switch status {
	case "", "review_due", "expired", "obsolete", "archived", "active":
	default:
		return ReviewResult{}, fmt.Errorf("invalid validity filter")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return ReviewResult{}, fmt.Errorf("review limit must be 1 to 100")
	}
	entries, err := s.list(id, global)
	if err != nil {
		return ReviewResult{}, err
	}
	items := []ReviewItem{}
	for _, e := range entries {
		if (status == "" && e.Validity != "active") || e.Validity == status {
			items = append(items, ReviewItem{e.Key, e.Revision, e.Validity, e.Metadata})
		}
	}
	order := map[string]int{"expired": 0, "review_due": 1, "obsolete": 2, "archived": 3, "active": 4}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Validity != items[j].Validity {
			return order[items[i].Validity] < order[items[j].Validity]
		}
		return items[i].Key < items[j].Key
	})
	out := ReviewResult{Items: []ReviewItem{}, Total: len(items), Omitted: len(items)}
	for _, item := range items {
		if len(out.Items) >= limit {
			break
		}
		out.Items = append(out.Items, item)
		out.Omitted--
		b, _ := json.Marshal(out)
		if len(b) > 65536 {
			out.Items = out.Items[:len(out.Items)-1]
			out.Omitted++
		}
	}
	return out, nil
}
