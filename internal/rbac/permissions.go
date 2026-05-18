package rbac

// PermissionToTools maps permission names to MCP tool names
var PermissionToTools = map[string][]string{
	// Core tools (always available)
	"tool:core": {
		"get_time", "system_info", "weather",
	},

	// Delegation (A2A multi-agent orchestration)
	"tool:delegate": {"delegate"},

	// Task management
	"tool:tasks": {
		"task_create", "task_list", "task_get", "task_run",
		"task_schedule", "task_cancel", "task_preview_cron", "task_actions_list",
		"scheduled_tasks_list", "scheduled_task_cancel",
	},

	// Reflection & Learning
	"tool:reflection": {
		"reflection_search_learnings", "reflection_store_learning", "reflection_trigger",
	},

	// Advanced (subtasks, templates)
	"tool:advanced": {
		"subtask_spawn", "subtask_spawn_parallel", "subtask_list", "subtask_results",
		"template_list", "template_get", "template_create_workflow",
	},

	// Output channel selection
	"tool:output": {
		"select_output_channel", "set_response_mode",
	},

	// Telegram interaction
	"tool:telegram": {
		"set_reaction", "reply_message", "edit_message", "send_message",
		"get_chat_info", "get_chat_member", "pin_message", "unpin_message",
	},

	// Group chat behavior
	"tool:group_chat": {
		"decide_response", "query_channel_memory", "query_membership", "configure_channel",
	},

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
