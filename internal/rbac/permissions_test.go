package rbac

import (
	"sort"
	"testing"
)

// TestPermissionToToolsMapping tests that permission mappings exist
func TestPermissionToToolsMapping(t *testing.T) {
	// Test core tool permissions
	corePermissions := []string{
		"tool:weather",
		"tool:cortex_search",
		"tool:cortex_store",
		"tool:system_info",
	}

	for _, perm := range corePermissions {
		if tools, ok := PermissionToTools[perm]; !ok || len(tools) == 0 {
			t.Errorf("Permission %q should have at least one tool mapped", perm)
		}
	}
}

// TestPermissionToToolsObsidian tests Obsidian permission groups
func TestPermissionToToolsObsidian(t *testing.T) {
	// Test obsidian read
	readTools := PermissionToTools["tool:obsidian:read"]
	if len(readTools) == 0 {
		t.Error("tool:obsidian:read should have tools")
	}

	expectedReadTools := []string{
		"obsidian_read", "obsidian_search", "obsidian_folders",
		"obsidian_tags", "obsidian_search_tag", "obsidian_backlinks", "obsidian_list",
	}
	for _, tool := range expectedReadTools {
		found := false
		for _, t := range readTools {
			if t == tool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool:obsidian:read should include %q", tool)
		}
	}

	// Test obsidian write
	writeTools := PermissionToTools["tool:obsidian:write"]
	if len(writeTools) == 0 {
		t.Error("tool:obsidian:write should have tools")
	}

	expectedWriteTools := []string{
		"obsidian_create", "obsidian_update", "obsidian_append",
		"obsidian_delete", "obsidian_move",
	}
	for _, tool := range expectedWriteTools {
		found := false
		for _, t := range writeTools {
			if t == tool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool:obsidian:write should include %q", tool)
		}
	}
}

// TestPermissionToToolsGWS tests Google Workspace permission groups
func TestPermissionToToolsGWS(t *testing.T) {
	gwsPermissions := map[string]int{
		"gws:oauth":          1, // google_oauth_connect
		"gws:gmail:read":     1, // gmail_get_unread
		"gws:gmail:write":    2, // gmail_send, gmail_mark_read
		"gws:calendar:read":  1, // calendar_list_events
		"gws:calendar:write": 3, // calendar_create_event, calendar_update_event, calendar_delete_event
		"gws:drive:read":     1, // drive_list_files
		"gws:tasks:read":     1, // tasks_list
		"gws:tasks:write":    4, // tasks_create, tasks_update, tasks_delete, tasks_complete
	}

	for perm, expectedCount := range gwsPermissions {
		tools, ok := PermissionToTools[perm]
		if !ok {
			t.Errorf("Permission %q not found", perm)
			continue
		}
		if len(tools) != expectedCount {
			t.Errorf("Permission %q has %d tools, want %d", perm, len(tools), expectedCount)
		}
	}
}

// TestPermissionToToolsSDK tests SDK permission groups
func TestPermissionToToolsSDK(t *testing.T) {
	sdkPermissions := map[string]string{
		"sdk:web_search":  "web_search",
		"sdk:bash":        "Bash",
		"sdk:text_editor": "TextEditor",
		"sdk:computer":    "Computer",
	}

	for perm, expectedTool := range sdkPermissions {
		tools, ok := PermissionToTools[perm]
		if !ok {
			t.Errorf("Permission %q not found", perm)
			continue
		}
		if len(tools) != 1 || tools[0] != expectedTool {
			t.Errorf("Permission %q should map to [%q], got %v", perm, expectedTool, tools)
		}
	}
}

// TestGetToolsForPermissions tests permission to tools conversion
func TestGetToolsForPermissions(t *testing.T) {
	t.Run("empty permissions", func(t *testing.T) {
		tools := GetToolsForPermissions([]string{})
		if len(tools) != 0 {
			t.Errorf("Empty permissions should return empty tools, got %v", tools)
		}
	})

	t.Run("single permission", func(t *testing.T) {
		tools := GetToolsForPermissions([]string{"tool:weather"})
		if len(tools) != 1 || tools[0] != "weather" {
			t.Errorf("Expected [weather], got %v", tools)
		}
	})

	t.Run("multiple permissions", func(t *testing.T) {
		tools := GetToolsForPermissions([]string{
			"tool:weather",
			"tool:cortex_search",
		})
		if len(tools) != 2 {
			t.Errorf("Expected 2 tools, got %d: %v", len(tools), tools)
		}
	})

	t.Run("permission with multiple tools", func(t *testing.T) {
		tools := GetToolsForPermissions([]string{"tool:obsidian:read"})
		// obsidian:read has 7 tools
		if len(tools) != 7 {
			t.Errorf("Expected 7 tools for obsidian:read, got %d: %v", len(tools), tools)
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		// If same permission is passed twice, tools should be unique
		tools := GetToolsForPermissions([]string{
			"tool:weather",
			"tool:weather",
		})
		if len(tools) != 1 {
			t.Errorf("Duplicate permissions should dedupe tools, got %v", tools)
		}
	})

	t.Run("unknown permission", func(t *testing.T) {
		tools := GetToolsForPermissions([]string{"unknown:permission"})
		if len(tools) != 0 {
			t.Errorf("Unknown permission should return empty tools, got %v", tools)
		}
	})

	t.Run("mixed known and unknown", func(t *testing.T) {
		tools := GetToolsForPermissions([]string{
			"tool:weather",
			"unknown:permission",
			"tool:cortex_search",
		})
		if len(tools) != 2 {
			t.Errorf("Expected 2 tools, got %d: %v", len(tools), tools)
		}
	})
}

// TestGetToolsForPermissionsComprehensive tests all permissions return valid tools
func TestGetToolsForPermissionsComprehensive(t *testing.T) {
	allPermissions := make([]string, 0)
	for perm := range PermissionToTools {
		allPermissions = append(allPermissions, perm)
	}

	tools := GetToolsForPermissions(allPermissions)

	// Should have many unique tools
	if len(tools) < 30 {
		t.Errorf("Expected at least 30 unique tools from all permissions, got %d", len(tools))
	}

	// Verify no empty tool names
	for _, tool := range tools {
		if tool == "" {
			t.Error("Found empty tool name in results")
		}
	}
}

// TestPermissionToToolsImmutability tests that returned slices are independent
func TestPermissionToToolsImmutability(t *testing.T) {
	// Get tools and modify the returned slice
	tools1 := GetToolsForPermissions([]string{"tool:weather"})
	if len(tools1) > 0 {
		tools1[0] = "modified"
	}

	// Get same tools again
	tools2 := GetToolsForPermissions([]string{"tool:weather"})

	// Should still have original value
	if len(tools2) > 0 && tools2[0] == "modified" {
		t.Error("GetToolsForPermissions returned mutable reference")
	}
}

// TestPermissionKeys verifies all expected permissions exist
func TestPermissionKeys(t *testing.T) {
	expectedPermissions := []string{
		"tool:weather",
		"tool:cortex_search",
		"tool:cortex_store",
		"tool:system_info",
		"tool:obsidian:read",
		"tool:obsidian:write",
		"gws:oauth",
		"gws:gmail:read",
		"gws:gmail:write",
		"gws:calendar:read",
		"gws:calendar:write",
		"gws:drive:read",
		"gws:tasks:read",
		"gws:tasks:write",
		"sdk:web_search",
		"sdk:bash",
		"sdk:text_editor",
		"sdk:computer",
	}

	for _, perm := range expectedPermissions {
		if _, ok := PermissionToTools[perm]; !ok {
			t.Errorf("Expected permission %q not found in PermissionToTools", perm)
		}
	}
}

// TestToolNamesValid verifies all tool names are valid identifiers
func TestToolNamesValid(t *testing.T) {
	for perm, tools := range PermissionToTools {
		for _, tool := range tools {
			if tool == "" {
				t.Errorf("Permission %q has empty tool name", perm)
			}
			// Tool names should be snake_case or PascalCase
			for _, c := range tool {
				if !((c >= 'a' && c <= 'z') ||
					(c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9') ||
					c == '_') {
					t.Errorf("Permission %q has tool %q with invalid character %q", perm, tool, string(c))
				}
			}
		}
	}
}

// TestGetToolsForPermissionsOrder tests that results are deterministic
func TestGetToolsForPermissionsOrder(t *testing.T) {
	perms := []string{"tool:obsidian:read", "tool:weather"}

	// Run multiple times and sort results to compare
	var results [][]string
	for i := 0; i < 3; i++ {
		tools := GetToolsForPermissions(perms)
		sort.Strings(tools)
		results = append(results, tools)
	}

	// All results should have same length
	for i := 1; i < len(results); i++ {
		if len(results[i]) != len(results[0]) {
			t.Errorf("Run %d returned %d tools, run 0 returned %d", i, len(results[i]), len(results[0]))
		}
		// After sorting, content should match
		for j := range results[0] {
			if results[i][j] != results[0][j] {
				t.Errorf("Results differ at index %d: %s vs %s", j, results[i][j], results[0][j])
			}
		}
	}
}
