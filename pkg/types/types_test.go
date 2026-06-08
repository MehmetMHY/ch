package types

import (
	"encoding/json"
	"testing"
)

func TestBaseURLValue_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantS    string
		wantM    []string
		wantErr  bool
	}{
		{
			name:     "Single string base URL",
			jsonData: `"https://api.openai.com/v1"`,
			wantS:    "https://api.openai.com/v1",
			wantM:    nil,
			wantErr:  false,
		},
		{
			name:     "Multiple string base URLs",
			jsonData: `["https://api-1.com", "https://api-2.com"]`,
			wantS:    "",
			wantM:    []string{"https://api-1.com", "https://api-2.com"},
			wantErr:  false,
		},
		{
			name:     "Empty list base URLs",
			jsonData: `[]`,
			wantS:    "",
			wantM:    []string{},
			wantErr:  false,
		},
		{
			name:     "Invalid JSON types (object)",
			jsonData: `{"url": "https://api.com"}`,
			wantErr:  true,
		},
		{
			name:     "Invalid JSON types (number array)",
			jsonData: `[1, 2]`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b BaseURLValue
			err := json.Unmarshal([]byte(tt.jsonData), &b)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if b.Single != tt.wantS {
				t.Errorf("Single = %v, want %v", b.Single, tt.wantS)
			}
			if len(b.Multi) != len(tt.wantM) {
				t.Fatalf("Multi len = %v, want %v", len(b.Multi), len(tt.wantM))
			}
			for i := range b.Multi {
				if b.Multi[i] != tt.wantM[i] {
					t.Errorf("Multi[%d] = %v, want %v", i, b.Multi[i], tt.wantM[i])
				}
			}
		})
	}
}

func TestBaseURLValue_UnmarshalJSONMixedTypeArrayErrors(t *testing.T) {
	// A heterogeneous array (string + number) can't unmarshal into either
	// string or []string, so it must surface an error rather than silently
	// dropping the bad element.
	var b BaseURLValue
	if err := json.Unmarshal([]byte(`["https://api.com", 5]`), &b); err == nil {
		t.Errorf("expected error for mixed-type array, got Single=%q Multi=%v", b.Single, b.Multi)
	}
}

func TestBaseURLValue_EmptyArrayIsNotMulti(t *testing.T) {
	// An empty JSON array yields a non-nil but empty Multi slice; IsMulti must
	// report false and GetURLs must return an empty list (no phantom URLs).
	var b BaseURLValue
	if err := json.Unmarshal([]byte(`[]`), &b); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if b.IsMulti() {
		t.Error("IsMulti() should be false for an empty array")
	}
	if got := b.GetURLs(); len(got) != 0 {
		t.Errorf("GetURLs() = %v, want empty", got)
	}
}

func TestBaseURLValue_UnmarshalIntoStructField(t *testing.T) {
	// Exercises the unmarshaller as it is actually used: as a field inside a
	// larger config object, for both the string and array forms.
	var single struct {
		BaseURL BaseURLValue `json:"base_url"`
	}
	if err := json.Unmarshal([]byte(`{"base_url": "https://api.com"}`), &single); err != nil {
		t.Fatalf("unmarshal single: %v", err)
	}
	if single.BaseURL.Single != "https://api.com" || single.BaseURL.IsMulti() {
		t.Errorf("single field: got %+v", single.BaseURL)
	}

	var multi struct {
		BaseURL BaseURLValue `json:"base_url"`
	}
	if err := json.Unmarshal([]byte(`{"base_url": ["https://a.com", "https://b.com"]}`), &multi); err != nil {
		t.Fatalf("unmarshal multi: %v", err)
	}
	if !multi.BaseURL.IsMulti() || len(multi.BaseURL.GetURLs()) != 2 {
		t.Errorf("multi field: got %+v", multi.BaseURL)
	}
}

func TestBaseURLValue_UnmarshalJSONClearsPreviousMode(t *testing.T) {
	b := BaseURLValue{Multi: []string{"https://api-1.com"}}
	if err := json.Unmarshal([]byte(`"https://api-single.com"`), &b); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if b.Single != "https://api-single.com" {
		t.Errorf("Single = %q, want %q", b.Single, "https://api-single.com")
	}
	if b.Multi != nil {
		t.Errorf("Multi = %v, want nil after unmarshalling a single URL", b.Multi)
	}

	if err := json.Unmarshal([]byte(`["https://api-2.com"]`), &b); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if b.Single != "" {
		t.Errorf("Single = %q, want empty after unmarshalling multiple URLs", b.Single)
	}
	if len(b.Multi) != 1 || b.Multi[0] != "https://api-2.com" {
		t.Errorf("Multi = %v, want [https://api-2.com]", b.Multi)
	}
}

func TestBaseURLValue_IsMulti(t *testing.T) {
	tests := []struct {
		name string
		b    BaseURLValue
		want bool
	}{
		{
			name: "Single URL",
			b: BaseURLValue{
				Single: "https://api.com",
			},
			want: false,
		},
		{
			name: "Multiple URLs",
			b: BaseURLValue{
				Multi: []string{"https://api-1.com", "https://api-2.com"},
			},
			want: true,
		},
		{
			name: "Empty BaseURLValue",
			b:    BaseURLValue{},
			want: false,
		},
		{
			name: "Empty Multi slice",
			b: BaseURLValue{
				Multi: []string{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.IsMulti(); got != tt.want {
				t.Errorf("IsMulti() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBaseURLValue_GetURLs(t *testing.T) {
	tests := []struct {
		name string
		b    BaseURLValue
		want []string
	}{
		{
			name: "Single URL non-empty",
			b: BaseURLValue{
				Single: "https://api.com",
			},
			want: []string{"https://api.com"},
		},
		{
			name: "Multiple URLs",
			b: BaseURLValue{
				Multi: []string{"https://api-1.com", "https://api-2.com"},
			},
			want: []string{"https://api-1.com", "https://api-2.com"},
		},
		{
			name: "Single URL takes backseat if Multi is set",
			b: BaseURLValue{
				Single: "https://api-fallback.com",
				Multi:  []string{"https://api-1.com"},
			},
			want: []string{"https://api-1.com"},
		},
		{
			name: "Single URL used when Multi is empty",
			b: BaseURLValue{
				Single: "https://api.com",
				Multi:  []string{},
			},
			want: []string{"https://api.com"},
		},
		{
			name: "Empty BaseURLValue",
			b:    BaseURLValue{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.GetURLs()
			if len(got) != len(tt.want) {
				t.Fatalf("GetURLs() len = %v, want %v", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetURLs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
