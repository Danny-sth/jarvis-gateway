package rbac

// PermissionToTools maps permission names to MCP tool names
var PermissionToTools = map[string][]string{
	// Core tools
	"tool:weather":       {"weather"},
	"tool:cortex_search": {"cortex_search"},
	"tool:cortex_store":  {"cortex_store"},
	"tool:system_info":   {"system_info"},

	// Obsidian (connect - allows users to connect their vault)
	"tool:obsidian:connect": {"connect_obsidian"},
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

	// GWS OAuth (connect Google account)
	"gws:oauth": {"google_oauth_connect"},

	// GWS Gmail (actual MCP tool names from gws.py)
	"gws:gmail:read":  {"gmail_get_unread"},
	"gws:gmail:write": {"gmail_send", "gmail_mark_read"},

	// GWS Calendar (actual MCP tool names from gws.py)
	"gws:calendar:read":  {"calendar_list_events"},
	"gws:calendar:write": {"calendar_create_event", "calendar_update_event", "calendar_delete_event"},

	// GWS Drive (actual MCP tool names from gws.py)
	"gws:drive:read": {"drive_list_files"},

	// GWS Tasks (actual MCP tool names from gws.py)
	"gws:tasks:read":  {"tasks_list"},
	"gws:tasks:write": {"tasks_create", "tasks_update", "tasks_delete", "tasks_complete"},

	// SDK tools (built into Claude agent SDK)
	"sdk:web_search":  {"web_search"},  // Custom DuckDuckGo search tool (not Anthropic built-in)
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
