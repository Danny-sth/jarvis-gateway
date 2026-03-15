package rbac

// PermissionToTools maps permission names to MCP tool names
var PermissionToTools = map[string][]string{
	// Core tools
	"tool:weather":       {"weather"},
	"tool:cortex_search": {"cortex_search"},
	"tool:cortex_store":  {"cortex_store"},
	"tool:system_info":   {"system_info"},

	// Obsidian (read group)
	"tool:obsidian:read": {
		"obsidian_read", "obsidian_search", "obsidian_folders",
		"obsidian_tags", "obsidian_search_tag", "obsidian_backlinks", "obsidian_list",
	},
	// Obsidian (write group)
	"tool:obsidian:write": {
		"obsidian_create", "obsidian_update", "obsidian_append",
		"obsidian_delete", "obsidian_move",
	},

	// GWS Gmail
	"gws:gmail:read":  {"gws_gmail_list", "gws_gmail_read"},
	"gws:gmail:write": {"gws_gmail_send", "gws_gmail_mark_read"},

	// GWS Calendar
	"gws:calendar:read":  {"gws_calendar_list"},
	"gws:calendar:write": {"gws_calendar_create", "gws_calendar_update", "gws_calendar_delete"},

	// GWS Drive
	"gws:drive:read": {"gws_drive_list", "gws_drive_read"},

	// GWS Tasks
	"gws:tasks:read":  {"gws_tasks_list"},
	"gws:tasks:write": {"gws_tasks_create", "gws_tasks_update", "gws_tasks_delete"},

	// SDK tools (built into Claude agent SDK)
	"sdk:web_search":  {"WebSearch"},
	"sdk:bash":        {"Bash"},
	"sdk:text_editor": {"TextEditor"},
	"sdk:computer":    {"Computer"},
}

// GetToolsForPermissions returns a unique list of tools for given permissions
func GetToolsForPermissions(permissions []string) []string {
	toolSet := make(map[string]bool)
	for _, perm := range permissions {
		if tools, ok := PermissionToTools[perm]; ok {
			for _, tool := range tools {
				toolSet[tool] = true
			}
		}
	}

	result := make([]string, 0, len(toolSet))
	for tool := range toolSet {
		result = append(result, tool)
	}
	return result
}
