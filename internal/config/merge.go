package config

import (
	"encoding/json"
	"fmt"
)

// Merge overlays src on top of dst. Maps are recursively merged; arrays and
// scalar values are replaced. It is deliberately deterministic and is used for
// global -> project configuration layering as well as tests and plugins.
func Merge(dst, src Config) Config {
	var left, right map[string]any
	_ = marshalMap(dst, &left)
	_ = marshalMap(src, &right)
	merged := mergeValue(left, right).(map[string]any)
	data, err := json.Marshal(merged)
	if err != nil { return dst }
	var out Config
	if err := json.Unmarshal(data, &out); err != nil { return dst }
	return out
}

func marshalMap(v Config, dst *map[string]any) error {
	b, err := json.Marshal(v); if err != nil { return err }
	return json.Unmarshal(b, dst)
}

func mergeValue(a, b any) any {
	am, aok := a.(map[string]any); bm, bok := b.(map[string]any)
	if !aok || !bok { return b }
	out := make(map[string]any, len(am)+len(bm))
	for k, v := range am { out[k] = v }
	for k, v := range bm {
		if old, ok := out[k]; ok { out[k] = mergeValue(old, v) } else { out[k] = v }
	}
	return out
}

// MergeJSON is useful to extensions that carry unknown configuration keys.
func MergeJSON(dst, src []byte) ([]byte, error) {
	var a, b map[string]any
	if err := json.Unmarshal(dst, &a); err != nil { return nil, fmt.Errorf("destination: %w", err) }
	if err := json.Unmarshal(src, &b); err != nil { return nil, fmt.Errorf("source: %w", err) }
	return json.MarshalIndent(mergeValue(a, b), "", "  ")
}
