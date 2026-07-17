package platform

import (
	"encoding/json"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

// ---- mergeConsecutiveUserMessages ----

func TestMergeConsecutiveUserMessages(t *testing.T) {
	m := NewManager(&types.Config{})

	tests := []struct {
		name     string
		messages []types.ChatMessage
		want     []types.ChatMessage
	}{
		{
			name:     "Empty list",
			messages: []types.ChatMessage{},
			want:     []types.ChatMessage{},
		},
		{
			name:     "Single message",
			messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			want:     []types.ChatMessage{{Role: "user", Content: "Hello"}},
		},
		{
			name: "Two consecutive user messages",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "user", Content: "World"},
			},
			want: []types.ChatMessage{{Role: "user", Content: "Hello\n\nWorld"}},
		},
		{
			name: "User then assistant",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi"},
			},
			want: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi"},
			},
		},
		{
			name: "Mixed consecutive user and assistant messages",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "user", Content: "Are you there?"},
				{Role: "assistant", Content: "Yes."},
				{Role: "user", Content: "Great!"},
				{Role: "user", Content: "How's the weather?"},
			},
			want: []types.ChatMessage{
				{Role: "user", Content: "Hello\n\nAre you there?"},
				{Role: "assistant", Content: "Yes."},
				{Role: "user", Content: "Great!\n\nHow's the weather?"},
			},
		},
		{
			name: "Only assistant messages (no merging needed)",
			messages: []types.ChatMessage{
				{Role: "assistant", Content: "A"},
				{Role: "assistant", Content: "B"},
			},
			want: []types.ChatMessage{
				{Role: "assistant", Content: "A"},
				{Role: "assistant", Content: "B"},
			},
		},
		{
			name: "Trailing user message is flushed",
			messages: []types.ChatMessage{
				{Role: "assistant", Content: "Hi"},
				{Role: "user", Content: "Q1"},
				{Role: "user", Content: "Q2"},
			},
			want: []types.ChatMessage{
				{Role: "assistant", Content: "Hi"},
				{Role: "user", Content: "Q1\n\nQ2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.mergeConsecutiveUserMessages(tt.messages)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got) = %d, want %d; got=%v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i].Role != tt.want[i].Role {
					t.Errorf("[%d] Role = %q, want %q", i, got[i].Role, tt.want[i].Role)
				}
				if got[i].Content != tt.want[i].Content {
					t.Errorf("[%d] Content = %q, want %q", i, got[i].Content, tt.want[i].Content)
				}
			}
		})
	}
}

// ---- parseTimestamp ----

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int64
	}{
		{"Float64 seconds", float64(1686588896), 1686588896},
		{"Float64 milliseconds", float64(1686588896000), 1686588896},
		{"Float64 microseconds", float64(1686588896000000), 1686588896},
		{"Float64 nanoseconds", float64(1686588896000000000), 1686588896},
		{"String numeric seconds", "1686588896", 1686588896},
		{"String numeric milliseconds", "1686588896000", 1686588896},
		{"String numeric microseconds", "1686588896000000", 1686588896},
		{"String numeric nanoseconds", "1686588896000000000", 1686588896},
		{"RFC3339 string", "2023-06-12T16:54:56Z", 1686588896},
		{"RFC3339Nano string", "2023-06-12T16:54:56.123456789Z", 1686588896},
		{"Date-only string", "2023-06-12", 1686528000}, // midnight UTC
		{"Scientific notation string", "1.686588896e9", 1686588896},
		{"Invalid string", "invalid-date", 0},
		{"Empty string", "", 0},
		{"Negative numeric string", "-1", 0},
		{"Negative float", float64(-1), 0},
		{"Zero float", float64(0), 0},
		// Just under the milliseconds threshold (1e12) stays interpreted as seconds.
		{"Just under ms threshold stays seconds", float64(999999999999), 999999999999},
		// Exactly at the milliseconds threshold is divided by 1e3.
		{"At ms threshold", float64(1e12), 1e12 / 1e3},
		{"Unsupported type bool", true, 0},
		{"Unsupported type nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTimestamp(tt.input)
			if got != tt.want {
				t.Errorf("parseTimestamp(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---- sortModelsByTime ----

func TestSortModelsByTime(t *testing.T) {
	tests := []struct {
		name   string
		models []modelWithTime
		want   []string
	}{
		{
			name: "Descending by timestamp",
			models: []modelWithTime{
				{name: "old", created: 1000},
				{name: "new", created: 2000},
				{name: "mid", created: 1500},
			},
			want: []string{"new", "mid", "old"},
		},
		{
			name: "Same timestamp: alphabetical",
			models: []modelWithTime{
				{name: "model-B", created: 1000},
				{name: "model-A", created: 1000},
				{name: "model-C", created: 1000},
			},
			want: []string{"model-A", "model-B", "model-C"},
		},
		{
			name: "All zero timestamps: alphabetical fallback",
			models: []modelWithTime{
				{name: "B", created: 0},
				{name: "A", created: 0},
				{name: "C", created: 0},
			},
			want: []string{"A", "B", "C"},
		},
		{
			name:   "Empty list",
			models: []modelWithTime{},
			want:   []string{},
		},
		{
			name:   "Single model",
			models: []modelWithTime{{name: "only", created: 42}},
			want:   []string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortModelsByTime(tt.models)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] got=%q, want=%q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---- sortModelsGroupedByPlatform ----

func TestSortModelsGroupedByPlatform(t *testing.T) {
	models := []modelWithTime{
		{name: "openai|gpt-4-old", created: 1000},
		{name: "openai|gpt-4-new", created: 2000},
		{name: "groq|llama3-old", created: 1000},
		{name: "groq|llama3-new", created: 2000},
	}
	// Groq < openai alphabetically; within each platform newest first
	want := []string{
		"groq|llama3-new",
		"groq|llama3-old",
		"openai|gpt-4-new",
		"openai|gpt-4-old",
	}
	got := sortModelsGroupedByPlatform(models)
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got=%q, want=%q", i, got[i], want[i])
		}
	}
}

// ---- isSlowModel / IsReasoningModel ----

func TestIsSlowModel(t *testing.T) {
	cfg := &types.Config{
		SlowModelPatterns: []string{`^o\d+`, `^gpt-5$`},
	}
	m := NewManager(cfg)

	tests := []struct {
		model string
		want  bool
	}{
		{"o1", true},
		{"o1-mini", true},
		{"o3", true},
		{"gpt-5", true},
		{"gpt-5-turbo", false}, // does not match ^gpt-5$
		{"gpt-4o", false},
		{"claude-3", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := m.isSlowModel(tt.model); got != tt.want {
				t.Errorf("isSlowModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
			// IsReasoningModel is an alias
			if got := m.IsReasoningModel(tt.model); got != tt.want {
				t.Errorf("IsReasoningModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsSlowModel_NoPatterns(t *testing.T) {
	m := NewManager(&types.Config{SlowModelPatterns: nil})
	if m.isSlowModel("o1") {
		t.Error("expected false with no patterns configured")
	}
}

// ---- extractModelsWithTimeFromJSON ----

func TestExtractModelsWithTimeFromJSON(t *testing.T) {
	m := NewManager(&types.Config{})

	t.Run("simple data.id path with created timestamps", func(t *testing.T) {
		raw := `{
			"data": [
				{"id": "model-a", "created": 2000},
				{"id": "model-b", "created": 1000}
			]
		}`
		var data interface{}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatal(err)
		}
		got, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 models, got %d", len(got))
		}
		// Extraction preserves source order; assert both entries exactly.
		byName := map[string]int64{}
		for _, g := range got {
			byName[g.name] = g.created
		}
		if c, ok := byName["model-a"]; !ok || c != 2000 {
			t.Errorf("model-a created = %d (present=%v), want 2000", c, ok)
		}
		if c, ok := byName["model-b"]; !ok || c != 1000 {
			t.Errorf("model-b created = %d (present=%v), want 1000", c, ok)
		}
	})

	t.Run("models.name path (Ollama style)", func(t *testing.T) {
		raw := `{
			"models": [
				{"name": "llama3"},
				{"name": "mistral"}
			]
		}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		got, err := m.extractModelsWithTimeFromJSON(data, "models.name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 models, got %d", len(got))
		}
	})

	t.Run("created_at ISO string timestamp", func(t *testing.T) {
		raw := `{
			"data": [
				{"id": "m1", "created_at": "2023-06-12T16:54:56Z"}
			]
		}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		got, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 model, got %+v", got)
		}
		if got[0].created != 1686588896 {
			t.Errorf("created timestamp = %d, want 1686588896", got[0].created)
		}
	})

	t.Run("modified_at fallback timestamp", func(t *testing.T) {
		raw := `{
			"data": [
				{"id": "m1", "modified_at": 1686588896000}
			]
		}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		got, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 model, got %+v", got)
		}
		if got[0].created != 1686588896 {
			t.Errorf("created timestamp = %d, want 1686588896", got[0].created)
		}
	})

	t.Run("missing path returns error", func(t *testing.T) {
		raw := `{"other": []}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		_, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err == nil {
			t.Error("expected error for missing path")
		}
	})

	t.Run("empty array returns empty slice", func(t *testing.T) {
		raw := `{"data": []}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		got, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected 0 models, got %d", len(got))
		}
	})

	t.Run("non-string name field is skipped", func(t *testing.T) {
		// id is a number, not a string -> the entry must be skipped entirely.
		raw := `{"data": [{"id": 123}, {"id": "valid"}]}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		got, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].name != "valid" {
			t.Errorf("expected only the string-id model, got %+v", got)
		}
	})

	t.Run("created preferred over created_at, invalid timestamp -> 0", func(t *testing.T) {
		// First field in the priority list (created) wins when valid.
		// A model with only an unparseable timestamp must end up with created=0.
		raw := `{"data": [
			{"id": "m1", "created": 2000, "created_at": "2023-06-12T16:54:56Z"},
			{"id": "m2", "created": "not-a-date"}
		]}`
		var data interface{}
		json.Unmarshal([]byte(raw), &data)
		got, err := m.extractModelsWithTimeFromJSON(data, "data.id")
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]int64{}
		for _, g := range got {
			byName[g.name] = g.created
		}
		if byName["m1"] != 2000 {
			t.Errorf("m1 created = %d, want 2000 (created preferred over created_at)", byName["m1"])
		}
		if byName["m2"] != 0 {
			t.Errorf("m2 created = %d, want 0 (unparseable timestamp)", byName["m2"])
		}
	})
}

func TestExtractPlatformModelsWithTimeFromJSONFiltersTogetherServerlessChatModels(t *testing.T) {
	m := NewManager(&types.Config{})
	raw := `[
		{"id": "serverless-chat", "type": "chat", "created": 3000, "pricing": {"hourly": 0, "input": 0.3, "output": 0.3}},
		{"id": "dedicated-only", "type": "chat", "created": 2000, "pricing": {"hourly": 1.5, "input": 0, "output": 0}},
		{"id": "image-model", "type": "image", "created": 1000, "pricing": {"hourly": 0, "input": 0.1, "output": 0.1}},
		{"id": "no-token-pricing", "type": "chat", "created": 500, "pricing": {"hourly": 0, "input": 0, "output": 0}}
	]`
	var data interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}

	got, err := m.extractPlatformModelsWithTimeFromJSON(data, types.Platform{Name: "together"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 Together serverless chat model, got %+v", got)
	}
	if got[0].name != "serverless-chat" || got[0].created != 3000 {
		t.Fatalf("unexpected model: %+v", got[0])
	}
}
