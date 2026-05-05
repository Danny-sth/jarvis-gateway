# Group Chat Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable Duq to handle Telegram groups and channels with intelligent response decisions (respond/react/observe) and proper memory access control.

**Architecture:** Full LLM per message — every group message goes to the agent which calls `decide_response` tool to choose action. Facts are saved with `source_channel` tag for access control. Membership graph tracks who can see what.

**Tech Stack:** Go (gateway), Python (duq), PostgreSQL, Alembic migrations, MCP tools

---

## File Structure

### Gateway (Go) - duq-gateway

| File | Responsibility |
|------|----------------|
| `internal/db/channels.go` | NEW: Channel CRUD operations |
| `internal/handlers/telegram.go` | MODIFY: Upsert channel on message |
| `internal/handlers/telegram_api.go` | MODIFY: Add channel config endpoints |

### Duq (Python) - duq

| File | Responsibility |
|------|----------------|
| `alembic/versions/007_channels_table.py` | NEW: Create channels table |
| `src/duq/mcp/tools/group_chat.py` | NEW: decide_response, query_channel_memory, query_membership, configure_channel |
| `src/duq/mcp/tools/__init__.py` | MODIFY: Import group_chat tools |
| `src/duq/services/channels.py` | NEW: Channel service (DB operations) |
| `src/duq/core/agent.py` | MODIFY: Handle decide_response for groups |

---

## Task 1: Database Migration — channels table

**Files:**
- Create: `duq/alembic/versions/007_channels_table.py`

- [ ] **Step 1: Create migration file**

```python
"""Create channels table for group/channel entities.

Revision ID: 007_channels_table
Revises: 006_unified_memory_architecture
Create Date: 2026-05-06
"""
from alembic import op


revision = "007_channels_table"
down_revision = "006_unified_memory_architecture"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute("""
        CREATE TABLE IF NOT EXISTS channels (
            id SERIAL PRIMARY KEY,
            telegram_id BIGINT UNIQUE NOT NULL,
            channel_type TEXT NOT NULL,
            title TEXT,
            username TEXT,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            config JSONB DEFAULT '{
                "response_mode": "full_llm",
                "observe_all": true,
                "allowed_tools": null,
                "personality_override": null
            }'::jsonb
        )
    """)
    op.execute("""
        CREATE INDEX IF NOT EXISTS idx_channels_telegram_id
        ON channels(telegram_id)
    """)
    op.execute("""
        CREATE INDEX IF NOT EXISTS idx_channels_type
        ON channels(channel_type)
    """)


def downgrade() -> None:
    op.execute("DROP TABLE IF EXISTS channels CASCADE")
```

- [ ] **Step 2: Verify migration syntax**

Run: `cd /home/danny/Documents/projects/duq && python -c "from alembic.versions import *; print('OK')"`
Expected: OK (no syntax errors)

- [ ] **Step 3: Commit**

```bash
git add alembic/versions/007_channels_table.py
git commit -m "feat(db): add channels table for group/channel entities"
```

---

## Task 2: Channel Service (Python)

**Files:**
- Create: `duq/src/duq/services/channels.py`

- [ ] **Step 1: Write test file**

```python
# tests/unit/services/test_channels.py
import pytest
from duq.services.channels import ChannelConfig, parse_channel_config


def test_parse_channel_config_default():
    config = parse_channel_config({})
    assert config.response_mode == "full_llm"
    assert config.observe_all is True
    assert config.allowed_tools is None


def test_parse_channel_config_custom():
    config = parse_channel_config({
        "response_mode": "mention_only",
        "observe_all": False,
        "allowed_tools": ["weather"],
    })
    assert config.response_mode == "mention_only"
    assert config.observe_all is False
    assert config.allowed_tools == ["weather"]
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/danny/Documents/projects/duq && python -m pytest tests/unit/services/test_channels.py -v`
Expected: FAIL with "No module named 'duq.services.channels'"

- [ ] **Step 3: Create channel service**

```python
# src/duq/services/channels.py
"""Channel service for group/channel management.

Provides:
- ChannelConfig dataclass for typed config access
- get_channel / upsert_channel / update_channel_config
- get_channel_members
"""

from dataclasses import dataclass
from datetime import datetime
from typing import TYPE_CHECKING

from loguru import logger
from sqlalchemy import text

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession


@dataclass
class ChannelConfig:
    """Typed channel configuration."""
    response_mode: str = "full_llm"  # full_llm | mention_only | disabled
    observe_all: bool = True
    allowed_tools: list[str] | None = None
    personality_override: str | None = None


@dataclass
class Channel:
    """Channel entity."""
    id: int
    telegram_id: int
    channel_type: str  # channel | group | supergroup
    title: str | None
    username: str | None
    config: ChannelConfig
    created_at: datetime
    updated_at: datetime


def parse_channel_config(data: dict | None) -> ChannelConfig:
    """Parse JSONB config into typed ChannelConfig."""
    if not data:
        return ChannelConfig()
    return ChannelConfig(
        response_mode=data.get("response_mode", "full_llm"),
        observe_all=data.get("observe_all", True),
        allowed_tools=data.get("allowed_tools"),
        personality_override=data.get("personality_override"),
    )


async def get_channel(session: "AsyncSession", telegram_id: int) -> Channel | None:
    """Get channel by telegram_id."""
    result = await session.execute(
        text("SELECT * FROM channels WHERE telegram_id = :telegram_id"),
        {"telegram_id": telegram_id}
    )
    row = result.fetchone()
    if not row:
        return None
    return Channel(
        id=row.id,
        telegram_id=row.telegram_id,
        channel_type=row.channel_type,
        title=row.title,
        username=row.username,
        config=parse_channel_config(row.config),
        created_at=row.created_at,
        updated_at=row.updated_at,
    )


async def upsert_channel(
    session: "AsyncSession",
    telegram_id: int,
    channel_type: str,
    title: str | None = None,
    username: str | None = None,
) -> int:
    """Create or update channel, return channel.id."""
    result = await session.execute(
        text("""
            INSERT INTO channels (telegram_id, channel_type, title, username)
            VALUES (:telegram_id, :channel_type, :title, :username)
            ON CONFLICT (telegram_id) DO UPDATE SET
                channel_type = EXCLUDED.channel_type,
                title = COALESCE(EXCLUDED.title, channels.title),
                username = COALESCE(EXCLUDED.username, channels.username),
                updated_at = NOW()
            RETURNING id
        """),
        {
            "telegram_id": telegram_id,
            "channel_type": channel_type,
            "title": title,
            "username": username,
        }
    )
    await session.commit()
    row = result.fetchone()
    return row.id


async def update_channel_config(
    session: "AsyncSession",
    telegram_id: int,
    config_updates: dict,
) -> ChannelConfig:
    """Update channel config (partial update)."""
    # Merge with existing config
    result = await session.execute(
        text("""
            UPDATE channels
            SET config = config || :updates::jsonb,
                updated_at = NOW()
            WHERE telegram_id = :telegram_id
            RETURNING config
        """),
        {
            "telegram_id": telegram_id,
            "updates": config_updates,
        }
    )
    await session.commit()
    row = result.fetchone()
    if not row:
        raise ValueError(f"Channel not found: {telegram_id}")
    return parse_channel_config(row.config)


async def get_channel_members(
    session: "AsyncSession",
    channel_telegram_id: int,
) -> list[dict]:
    """Get members of a channel with their info."""
    result = await session.execute(
        text("""
            SELECT u.id, u.first_name, u.last_name, u.username,
                   cm.joined_at, cm.is_active
            FROM channel_memberships cm
            JOIN users u ON cm.user_id = u.id
            WHERE cm.channel_id = :channel_id AND cm.is_active = TRUE
        """),
        {"channel_id": channel_telegram_id}
    )
    rows = result.fetchall()
    return [
        {
            "user_id": row.id,
            "name": f"{row.first_name or ''} {row.last_name or ''}".strip() or row.username,
            "username": row.username,
            "joined_at": row.joined_at.isoformat() if row.joined_at else None,
        }
        for row in rows
    ]
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/danny/Documents/projects/duq && python -m pytest tests/unit/services/test_channels.py -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/duq/services/channels.py tests/unit/services/test_channels.py
git commit -m "feat(services): add channel service for group management"
```

---

## Task 3: Group Chat Tools — decide_response

**Files:**
- Create: `duq/src/duq/mcp/tools/group_chat.py`

- [ ] **Step 1: Write test for decide_response**

```python
# tests/unit/mcp/tools/test_group_chat.py
import pytest
from duq.mcp.tools.group_chat import _should_respond


def test_should_respond_mention():
    """Direct mention should always respond."""
    assert _should_respond("@duq_bot what time is it?", "group") == "respond"


def test_should_respond_question():
    """Direct question should respond."""
    assert _should_respond("Duq, what's the weather?", "group") == "respond"


def test_should_respond_channel():
    """Channel posts default to observe (no user to reply to)."""
    assert _should_respond("Some announcement", "channel") == "observe"


def test_should_respond_greeting():
    """Greetings in groups should react."""
    result = _should_respond("Привет всем!", "group")
    assert result in ("react", "observe")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/danny/Documents/projects/duq && python -m pytest tests/unit/mcp/tools/test_group_chat.py -v`
Expected: FAIL with "No module named"

- [ ] **Step 3: Create group_chat tools**

```python
# src/duq/mcp/tools/group_chat.py
"""Group chat tools for Duq.

Tools for handling group/channel messages:
- decide_response: Choose respond/react/observe
- query_channel_memory: Get facts from channel context
- query_membership: Get channel members
- configure_channel: Update channel settings
"""

import re
from typing import Any

from loguru import logger

from duq.core.activity import get_current_request_metadata
from duq.mcp.registry import register_tool


# ==================== Helper Functions ====================

def _should_respond(message: str, chat_type: str) -> str:
    """Determine response action based on message content.

    Returns: "respond" | "react" | "observe"
    """
    message_lower = message.lower()

    # Channel posts: can only react or observe (no user to reply to)
    if chat_type == "channel":
        return "observe"

    # Direct mention
    if "@duq" in message_lower or "дюк" in message_lower:
        return "respond"

    # Question patterns
    question_patterns = [
        r"\?$",  # Ends with ?
        r"^(как|что|где|когда|почему|кто|сколько)",  # Russian question words
        r"^(how|what|where|when|why|who)",  # English question words
    ]
    for pattern in question_patterns:
        if re.search(pattern, message_lower):
            # Check if question seems directed at bot
            if any(word in message_lower for word in ["duq", "дюк", "бот", "bot"]):
                return "respond"

    # Greeting patterns (react with emoji)
    greeting_patterns = [
        r"^(привет|здравствуй|добр|hi|hello|hey)",
        r"(спасибо|thanks|thank you)",
    ]
    for pattern in greeting_patterns:
        if re.search(pattern, message_lower):
            return "react"

    # Default: observe
    return "observe"


def _get_react_emoji(message: str) -> str:
    """Choose appropriate reaction emoji."""
    message_lower = message.lower()

    if any(word in message_lower for word in ["спасибо", "thanks"]):
        return "❤️"
    if any(word in message_lower for word in ["привет", "hello", "hi"]):
        return "👋"
    if any(word in message_lower for word in ["круто", "класс", "awesome", "great"]):
        return "🔥"
    if "?" in message:
        return "🤔"

    return "👍"


# ==================== decide_response ====================

@register_tool(
    "decide_response",
    """Decide how to respond to a group/channel message.

Call this FIRST for every message in groups/channels.
Returns action: respond (full reply), react (emoji only), or observe (silent).

For channels: respond is not available (no user to reply to).
""",
    {
        "message": {
            "type": "string",
            "description": "The message text to analyze",
        },
        "chat_type": {
            "type": "string",
            "description": "Chat type: private, group, supergroup, channel",
        },
    },
)
async def decide_response(args: dict[str, Any], services: dict[str, Any]) -> dict[str, Any]:
    """Decide response action for group message."""
    message = args.get("message", "")
    chat_type = args.get("chat_type", "private")

    action = _should_respond(message, chat_type)

    result = {
        "action": action,
        "reason": f"Analyzed message in {chat_type}",
    }

    if action == "react":
        result["emoji"] = _get_react_emoji(message)

    logger.info(f"[decide_response] {chat_type}: {action}")
    return {"content": [{"type": "text", "text": str(result)}]}


# ==================== query_channel_memory ====================

@register_tool(
    "query_channel_memory",
    """Query facts from the current channel/group context.

Returns relevant facts that were shared in this channel.
Only returns facts you have access to see.
""",
    {
        "query": {
            "type": "string",
            "description": "What to search for in channel memory",
        },
    },
)
async def query_channel_memory(args: dict[str, Any], services: dict[str, Any]) -> dict[str, Any]:
    """Query channel-specific memory."""
    query = args.get("query", "")

    metadata = get_current_request_metadata()
    if not metadata:
        return {"content": [{"type": "text", "text": "Error: no request context"}]}

    chat_id = metadata.get("chat_id")
    if not chat_id:
        return {"content": [{"type": "text", "text": "Error: missing chat_id"}]}

    # Get memory provider from services
    memory = services.get("memory_provider")
    if not memory:
        return {"content": [{"type": "text", "text": "Memory not available"}]}

    # Query with channel filter
    try:
        # The memory provider should filter by source_channel
        facts = await memory.query(
            query=query,
            user_id=metadata.get("db_user_id"),
            source_channel=int(chat_id),
        )
        return {"content": [{"type": "text", "text": facts or "No relevant facts found"}]}
    except Exception as e:
        logger.error(f"query_channel_memory failed: {e}")
        return {"content": [{"type": "text", "text": f"Error: {e}"}]}


# ==================== query_membership ====================

@register_tool(
    "query_membership",
    """Get members of the current group/channel.

Returns list of members with their roles and relationships.
Only works in groups (not private chats).
""",
    {},
)
async def query_membership(args: dict[str, Any], services: dict[str, Any]) -> dict[str, Any]:
    """Get channel membership info."""
    metadata = get_current_request_metadata()
    if not metadata:
        return {"content": [{"type": "text", "text": "Error: no request context"}]}

    chat_id = metadata.get("chat_id")
    chat_type = metadata.get("chat_type", "private")

    if chat_type == "private":
        return {"content": [{"type": "text", "text": "This is a private chat, no membership to query"}]}

    # Get DB session from services
    db_session = services.get("db_session")
    if not db_session:
        return {"content": [{"type": "text", "text": "Database not available"}]}

    try:
        from duq.services.channels import get_channel, get_channel_members

        channel = await get_channel(db_session, int(chat_id))
        members = await get_channel_members(db_session, int(chat_id))

        result = {
            "channel": {
                "title": channel.title if channel else metadata.get("chat_title"),
                "type": chat_type,
            },
            "members": members,
            "member_count": len(members),
        }
        return {"content": [{"type": "text", "text": str(result)}]}
    except Exception as e:
        logger.error(f"query_membership failed: {e}")
        return {"content": [{"type": "text", "text": f"Error: {e}"}]}


# ==================== configure_channel ====================

@register_tool(
    "configure_channel",
    """Update channel/group behavior settings.

Available settings:
- response_mode: "full_llm" (always analyze), "mention_only" (only @mentions), "disabled"
- observe_all: true/false - save facts even when not responding
- allowed_tools: list of tool names to allow, or null for all
- personality_override: custom prompt section for this channel
""",
    {
        "response_mode": {
            "type": "string",
            "description": "How to respond: full_llm, mention_only, disabled",
        },
        "observe_all": {
            "type": "boolean",
            "description": "Save facts from all messages",
        },
        "allowed_tools": {
            "type": "array",
            "items": {"type": "string"},
            "description": "List of allowed tool names",
        },
    },
)
async def configure_channel(args: dict[str, Any], services: dict[str, Any]) -> dict[str, Any]:
    """Update channel configuration."""
    metadata = get_current_request_metadata()
    if not metadata:
        return {"content": [{"type": "text", "text": "Error: no request context"}]}

    chat_id = metadata.get("chat_id")
    chat_type = metadata.get("chat_type", "private")

    if chat_type == "private":
        return {"content": [{"type": "text", "text": "Cannot configure private chats"}]}

    # Build config updates
    config_updates = {}
    if "response_mode" in args:
        if args["response_mode"] not in ("full_llm", "mention_only", "disabled"):
            return {"content": [{"type": "text", "text": "Invalid response_mode"}]}
        config_updates["response_mode"] = args["response_mode"]
    if "observe_all" in args:
        config_updates["observe_all"] = bool(args["observe_all"])
    if "allowed_tools" in args:
        config_updates["allowed_tools"] = args["allowed_tools"]

    if not config_updates:
        return {"content": [{"type": "text", "text": "No settings to update"}]}

    db_session = services.get("db_session")
    if not db_session:
        return {"content": [{"type": "text", "text": "Database not available"}]}

    try:
        from duq.services.channels import update_channel_config

        new_config = await update_channel_config(db_session, int(chat_id), config_updates)
        return {"content": [{"type": "text", "text": f"Channel config updated: {new_config}"}]}
    except Exception as e:
        logger.error(f"configure_channel failed: {e}")
        return {"content": [{"type": "text", "text": f"Error: {e}"}]}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/danny/Documents/projects/duq && python -m pytest tests/unit/mcp/tools/test_group_chat.py -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/duq/mcp/tools/group_chat.py tests/unit/mcp/tools/test_group_chat.py
git commit -m "feat(tools): add group chat tools (decide_response, query_channel_memory, query_membership, configure_channel)"
```

---

## Task 4: Register Group Chat Tools

**Files:**
- Modify: `duq/src/duq/mcp/tools/__init__.py`

- [ ] **Step 1: Read current file**

Run: `cat /home/danny/Documents/projects/duq/src/duq/mcp/tools/__init__.py`

- [ ] **Step 2: Add import for group_chat**

Add after other imports:
```python
from . import group_chat  # noqa: F401
```

- [ ] **Step 3: Verify import works**

Run: `cd /home/danny/Documents/projects/duq && python -c "from duq.mcp.tools import group_chat; print('OK')"`
Expected: OK

- [ ] **Step 4: Commit**

```bash
git add src/duq/mcp/tools/__init__.py
git commit -m "feat(tools): register group_chat tools"
```

---

## Task 5: Gateway — Channel DB Operations

**Files:**
- Create: `duq-gateway/internal/db/channels.go`

- [ ] **Step 1: Create channels.go**

```go
// internal/db/channels.go
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ChannelConfig represents channel configuration
type ChannelConfig struct {
	ResponseMode        string   `json:"response_mode"`
	ObserveAll          bool     `json:"observe_all"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	PersonalityOverride string   `json:"personality_override,omitempty"`
}

// Channel represents a group/channel entity
type Channel struct {
	ID          int64
	TelegramID  int64
	ChannelType string
	Title       string
	Username    string
	Config      ChannelConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UpsertChannel creates or updates a channel record
func (c *Client) UpsertChannel(telegramID int64, channelType, title, username string) (int64, error) {
	var id int64
	err := c.db.QueryRow(`
		INSERT INTO channels (telegram_id, channel_type, title, username)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE SET
			channel_type = EXCLUDED.channel_type,
			title = COALESCE(EXCLUDED.title, channels.title),
			username = COALESCE(EXCLUDED.username, channels.username),
			updated_at = NOW()
		RETURNING id
	`, telegramID, channelType, title, username).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("upsert channel failed: %w", err)
	}

	log.Printf("[db] Upserted channel: telegram_id=%d type=%s id=%d", telegramID, channelType, id)
	return id, nil
}

// GetChannelConfig returns channel configuration
func (c *Client) GetChannelConfig(telegramID int64) (*ChannelConfig, error) {
	var configJSON []byte
	err := c.db.QueryRow(`
		SELECT config FROM channels WHERE telegram_id = $1
	`, telegramID).Scan(&configJSON)

	if err == sql.ErrNoRows {
		// Return default config if channel not found
		return &ChannelConfig{
			ResponseMode: "full_llm",
			ObserveAll:   true,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel config failed: %w", err)
	}

	var config ChannelConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("parse channel config failed: %w", err)
	}

	return &config, nil
}

// UpdateChannelMembership tracks user membership in channel
func (c *Client) UpdateChannelMembership(userID int64, channelID int64, isActive bool) error {
	if isActive {
		_, err := c.db.Exec(`
			INSERT INTO channel_memberships (user_id, channel_id, is_active)
			VALUES ($1, $2, TRUE)
			ON CONFLICT (user_id, channel_id)
			DO UPDATE SET is_active = TRUE, left_at = NULL
		`, userID, channelID)
		return err
	}

	_, err := c.db.Exec(`
		UPDATE channel_memberships
		SET is_active = FALSE, left_at = NOW()
		WHERE user_id = $1 AND channel_id = $2
	`, userID, channelID)
	return err
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/danny/Documents/projects/duq-gateway && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/db/channels.go
git commit -m "feat(db): add channel CRUD operations"
```

---

## Task 6: Gateway — Upsert Channel on Message

**Files:**
- Modify: `duq-gateway/internal/handlers/telegram.go`

- [ ] **Step 1: Find the section after extracting chatID**

Look for line ~129: `chatID := msg.Chat.ID`

- [ ] **Step 2: Add channel upsert after chatID extraction**

Add after line ~135 (after `log.Printf("[telegram] Message from..."`):

```go
// Upsert channel for group/channel messages
if msg.Chat.Type != "private" && deps.DBClient != nil {
	_, err := deps.DBClient.UpsertChannel(
		chatID,
		msg.Chat.Type,
		msg.Chat.Title,
		msg.Chat.Username,
	)
	if err != nil {
		log.Printf("[telegram] Failed to upsert channel: %v", err)
	}

	// Track membership if we know the user
	if msg.From != nil && dbUserID > 0 {
		if err := deps.DBClient.UpdateChannelMembership(dbUserID, chatID, true); err != nil {
			log.Printf("[telegram] Failed to update membership: %v", err)
		}
	}
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/danny/Documents/projects/duq-gateway && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/telegram.go
git commit -m "feat(telegram): upsert channel and track membership on messages"
```

---

## Task 7: Update Agent — Pass Channel Context

**Files:**
- Modify: `duq/src/duq/core/agent.py`

- [ ] **Step 1: Find process_message method**

Look for the section where `request_metadata` is used (~line 306)

- [ ] **Step 2: Add channel config to request metadata**

In `process_message`, after getting `request_metadata`, add channel config loading:

```python
# After line ~307: chat_type = request_metadata.get("chat_type", "private")

# Load channel config for groups/channels
channel_config = None
if chat_type in ("group", "supergroup", "channel") and db_user_id:
    try:
        from duq.services.channels import get_channel
        async with self.conversation_service.get_session() as session:
            channel = await get_channel(session, int(chat_id))
            if channel:
                channel_config = channel.config
    except Exception as e:
        logger.warning(f"Failed to load channel config: {e}")
```

- [ ] **Step 3: Include channel config in enhanced message**

Update the call to `_enhance_message` to include channel_config (modify signature if needed).

- [ ] **Step 4: Verify syntax**

Run: `cd /home/danny/Documents/projects/duq && python -c "from duq.core.agent import DuqAgent; print('OK')"`
Expected: OK

- [ ] **Step 5: Commit**

```bash
git add src/duq/core/agent.py
git commit -m "feat(agent): load and pass channel config for groups"
```

---

## Task 8: Deploy and Test

**Files:** None (deployment task)

- [ ] **Step 1: Push all changes**

```bash
cd /home/danny/Documents/projects/duq
git push

cd /home/danny/Documents/projects/duq-gateway
git push
```

- [ ] **Step 2: Deploy migration**

```bash
ssh root@90.156.230.49 "cd /opt/duq-dev/duq && git pull && docker exec duq-core alembic upgrade head"
```

- [ ] **Step 3: Deploy duq**

```bash
ssh root@90.156.230.49 "cd /opt/duq-dev/duq && git pull && cd /opt/duq-deploy && docker compose build duq && docker compose up -d duq"
```

- [ ] **Step 4: Deploy gateway**

```bash
ssh root@90.156.230.49 "cd /opt/duq-dev/duq-gateway && git pull && cd /opt/duq-deploy && docker compose build duq-gateway && docker compose up -d duq-gateway"
```

- [ ] **Step 5: Verify services are running**

```bash
ssh root@90.156.230.49 "docker logs duq-core --tail 20"
ssh root@90.156.230.49 "docker logs duq-gateway --tail 20"
```

- [ ] **Step 6: Test in Telegram group**

1. Send message in test group
2. Check logs for channel upsert
3. Check that decide_response tool is available
4. Verify facts are saved with source_channel

---

## Summary

| Task | Description | Est. Steps |
|------|-------------|------------|
| 1 | Database migration (channels table) | 3 |
| 2 | Channel service (Python) | 5 |
| 3 | Group chat tools | 5 |
| 4 | Register tools | 4 |
| 5 | Gateway channel DB | 3 |
| 6 | Gateway upsert on message | 4 |
| 7 | Agent channel context | 5 |
| 8 | Deploy and test | 6 |

**Total:** 8 tasks, ~35 steps
