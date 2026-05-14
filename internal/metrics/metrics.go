package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/your-org/monitor-agent/pkg/api"
)

// CalculateChecksum returns a SHA256 hex digest of the serialised events slice
func CalculateChecksum(events []*api.Event) string {
	h := sha256.New()
	for _, e := range events {
		data, _ := json.Marshal(e)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// FilterByMetricType returns only events of the given MetricType
func FilterByMetricType(events []*api.Event, mt api.MetricType) []*api.Event {
	out := make([]*api.Event, 0, len(events))
	for _, e := range events {
		if e.MetricType == mt {
			out = append(out, e)
		}
	}
	return out
}

// FilterByTag returns events that have the given tag key/value pair
func FilterByTag(events []*api.Event, key, value string) []*api.Event {
	out := make([]*api.Event, 0, len(events))
	for _, e := range events {
		if v, ok := e.Tags[key]; ok && v == value {
			out = append(out, e)
		}
	}
	return out
}

// EventCounts returns a map of type/level → count
func EventCounts(events []*api.Event) map[string]int {
	counts := make(map[string]int)
	for _, e := range events {
		if e.Type == "metric" {
			counts[string(e.MetricType)]++
		} else {
			counts[e.LogLevel]++
		}
	}
	return counts
}

// EstimateSize returns the approximate JSON-serialised size in bytes
func EstimateSize(events []*api.Event) int64 {
	var total int64
	for _, e := range events {
		data, _ := json.Marshal(e)
		total += int64(len(data))
	}
	return total
}
