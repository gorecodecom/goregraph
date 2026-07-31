package agentmetrics

import (
	"strings"
	"testing"
)

func TestParseTokenUsageDerivesProspectiveMetrics(t *testing.T) {
	raw := []byte(`{
		"input_tokens": 182151,
		"cached_input_tokens": 146944,
		"output_tokens": 6160,
		"reasoning_output_tokens": 3195
	}`)
	got, err := ParseTokenUsage(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := TokenUsage{
		InputTokens:           182151,
		CachedInputTokens:     146944,
		UncachedInputTokens:   35207,
		OutputTokens:          6160,
		ReasoningOutputTokens: 3195,
		TotalTokens:           188311,
		EffectiveTokens:       41367,
	}
	if got != want {
		t.Fatalf("usage = %#v, want %#v", got, want)
	}
	if parsed, err := ParseTokenUsageRow(got.TSV()); err != nil || parsed != want {
		t.Fatalf("TSV round trip = %#v, %v", parsed, err)
	}
}

func TestParseTokenUsageRejectsInvalidCounters(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "missing input", raw: `{"output_tokens":1}`, want: "input_tokens is required"},
		{name: "missing output", raw: `{"input_tokens":1}`, want: "output_tokens is required"},
		{name: "negative cache", raw: `{"input_tokens":1,"cached_input_tokens":-1,"output_tokens":1}`, want: "cached_input_tokens must not be negative"},
		{name: "cache exceeds input", raw: `{"input_tokens":1,"cached_input_tokens":2,"output_tokens":1}`, want: "cached_input_tokens exceeds input_tokens"},
		{name: "reasoning exceeds output", raw: `{"input_tokens":1,"output_tokens":1,"reasoning_output_tokens":2}`, want: "reasoning_output_tokens exceeds output_tokens"},
		{name: "empty total", raw: `{"input_tokens":0,"output_tokens":0}`, want: "token usage is empty"},
		{name: "overflow", raw: `{"input_tokens":9223372036854775807,"output_tokens":1}`, want: "overflows int64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTokenUsage([]byte(test.raw))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseTokenUsageRowRejectsInvalidRows(t *testing.T) {
	for _, row := range []string{
		"1\t2",
		"10\t5\t4\t1\t0\t11\t6",
		"10\t5\t5\t1\t2\t11\t6",
	} {
		if _, err := ParseTokenUsageRow(row); err == nil {
			t.Errorf("invalid usage row passed: %q", row)
		}
	}
}
