package rbac

import (
	"testing"
)

func TestFilterOutObsidianTools(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty list",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "no obsidian tools",
			input:    []string{"web_search", "gmail_list", "calendar_events"},
			expected: []string{"web_search", "gmail_list", "calendar_events"},
		},
		{
			name:     "only obsidian tools",
			input:    []string{"obsidian_read", "obsidian_list", "obsidian_create"},
			expected: []string{},
		},
		{
			name:     "mixed tools - filters obsidian",
			input:    []string{"web_search", "obsidian_read", "gmail_list", "obsidian_create"},
			expected: []string{"web_search", "gmail_list"},
		},
		{
			name:     "keeps connect_obsidian",
			input:    []string{"obsidian_read", "connect_obsidian", "obsidian_list"},
			expected: []string{"connect_obsidian"},
		},
		{
			name:     "all obsidian tools plus connect",
			input:    []string{"obsidian_read", "obsidian_create", "obsidian_update", "obsidian_delete", "obsidian_list", "connect_obsidian"},
			expected: []string{"connect_obsidian"},
		},
		{
			name:     "short tool names not affected",
			input:    []string{"obs", "obsidian", "web_search"},
			expected: []string{"obs", "obsidian", "web_search"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterOutObsidianTools(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d tools, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, tool := range result {
				if tool != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], tool)
				}
			}
		})
	}
}
