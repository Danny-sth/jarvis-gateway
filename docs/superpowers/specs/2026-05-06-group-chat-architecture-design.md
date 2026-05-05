# Group Chat Architecture Design

**Date:** 2026-05-06
**Status:** Approved
**Author:** Claude + Danny

## Overview

Duq поддерживает групповые чаты и каналы Telegram с полным интеллектом (Full LLM Per Message). Каждое сообщение обрабатывается агентом, который решает: ответить, поставить реакцию или молча наблюдать.

## Goals

1. Поддержка каналов (автор = канал) и групп (автор = пользователь)
2. Duq видит все сообщения и сам решает когда реагировать
3. Факты из групп сохраняются в unified memory с access tags
4. Membership graph для связей между участниками

## Non-Goals

- Tiered classification (отложено, можно добавить позже)
- Embedding-based triggers
- Per-message cost optimization в MVP

---

## Data Model

### Таблица: channels

Каналы и группы как сущности в системе.

| Column | Type | Description |
|--------|------|-------------|
| id | SERIAL PK | Internal ID |
| telegram_id | BIGINT UNIQUE | Telegram chat ID (negative) |
| channel_type | TEXT | 'channel' / 'group' / 'supergroup' |
| title | TEXT | Display name |
| username | TEXT | @channel_name if public |
| created_at | TIMESTAMPTZ | When first seen |
| config | JSONB | Channel configuration |

### Таблица: channel_members

Membership graph — связи участников с каналами.

| Column | Type | Description |
|--------|------|-------------|
| id | SERIAL PK | Internal ID |
| channel_id | INT FK | Reference to channels |
| user_id | INT FK | Reference to users |
| role | TEXT | 'admin' / 'member' / 'left' |
| joined_at | TIMESTAMPTZ | When joined |
| left_at | TIMESTAMPTZ | When left (nullable) |

**Unique constraint:** (channel_id, user_id)

### Channel Config (JSONB)

```json
{
  "response_mode": "full_llm",
  "observe_all": true,
  "allowed_tools": ["weather", "calendar"],
  "personality_override": null
}
```

| Field | Type | Description |
|-------|------|-------------|
| response_mode | string | "full_llm" / "mention_only" / "disabled" |
| observe_all | bool | Save facts from all messages |
| allowed_tools | array | Restrict available tools (null = all) |
| personality_override | string | Custom system prompt section |

### Hindsight Access Tags

Facts from channels stored with metadata:

| Tag | Type | Description |
|-----|------|-------------|
| owner_user_id | int | Message author (NULL for channel posts) |
| visibility | string | "private" / "channel" / "public" |
| source_channel | bigint | Telegram chat ID where fact originated |
| participants | array | User IDs involved in conversation |

---

## Message Flow

### 1. Gateway Receives Message

```
Telegram Update
    │
    ▼
Gateway extracts:
├── chat_id (negative for groups/channels)
├── chat_type (group/supergroup/channel)
├── from.id (author, NULL for channels)
├── message_id (for reactions)
└── text
    │
    ▼
Upsert channel record
    │
    ▼
Add/update channel_member (if author known)
    │
    ▼
Forward to Duq Agent
```

### 2. Agent Processing

```
Agent receives:
├── user_id = author (or chat_id for channels)
├── chat_id = where to respond
├── chat_type = private/group/supergroup/channel
├── message_id = for reactions
└── channel_config = from DB
    │
    ▼
Call decide_response tool
    │
    ├── RESPOND → Generate reply, send to chat_id
    ├── REACT → Set emoji reaction on message
    └── OBSERVE → Silent (no response)
    │
    ▼
Always: Save facts to Hindsight with source_channel tag
```

### 3. Response Delivery

| Decision | Action |
|----------|--------|
| respond | sendMessage to chat_id |
| react | setMessageReaction on message_id |
| observe | No Telegram API call |

---

## Tools

### decide_response

Decides how to react to a message.

**Input:**
- message: string
- chat_type: string
- chat_context: object (recent messages, participants)

**Output:**
```json
{
  "action": "respond" | "react" | "observe",
  "reason": "User asked a direct question",
  "emoji": "👍"  // only if action=react
}
```

**Logic:**
- Direct mention (@duq_bot) → respond
- Question directed at bot → respond
- Interesting fact → observe (save to memory)
- Greeting/thanks → react
- Unrelated chatter → observe

### query_channel_memory

Query facts from group/channel context.

**Input:**
- query: string
- channel_id: bigint (telegram chat ID)

**Output:**
- Relevant facts where source_channel = channel_id
- Filtered by requester's access (must be member)

### query_membership

Get group members and their relationships.

**Input:**
- channel_id: bigint

**Output:**
```json
{
  "channel": {"title": "Dev Chat", "type": "supergroup"},
  "members": [
    {"user_id": 15, "name": "Danny", "role": "admin"},
    {"user_id": 42, "name": "Lera", "role": "member"}
  ],
  "relationships": [
    {"from": 15, "to": 42, "type": "colleague"}
  ]
}
```

### configure_channel

Update channel behavior settings.

**Input:**
- channel_id: bigint
- config_updates: object (partial config)

**Output:**
- Updated full config

**Example:**
```json
{"response_mode": "mention_only", "observe_all": false}
```

---

## Access Control

### Visibility Levels

| Level | Who Can See |
|-------|-------------|
| private | Only owner_user_id |
| channel | Members of source_channel |
| public | Everyone |

### Access Check Algorithm

```
can_access(fact, requester_user_id, requester_channels):

  if fact.visibility == "private":
    return fact.owner_user_id == requester_user_id

  if fact.visibility == "channel":
    return fact.source_channel in requester_channels

  if fact.visibility == "public":
    return True

  return False
```

### Automatic Sensitivity Detection

Patterns that auto-set visibility=private:
- 12-digit numbers (ИНН)
- 16-digit numbers (card numbers)
- Keywords: пароль, password, secret, token

### Security Layers

1. **Prompt rules** — Duq instructed not to reveal private facts
2. **Code check** — Access control in query function
3. **Audit log** — Log all memory access attempts

---

## Channel vs Group Differences

| Aspect | Channel | Group |
|--------|---------|-------|
| from field | NULL | user_id |
| Author | Channel entity | Individual user |
| Fact owner | NULL (channel) | Author user_id |
| Membership | Subscribers (hidden) | Visible members |
| Replies | Broadcast only | Conversations |

### Handling Channels

- Author = channel entity (created in channels table)
- Facts have owner_user_id = NULL
- visibility = "channel" (visible to subscribers)
- No individual user tracking for channel posts
- **decide_response limited:** respond not available (no user to reply to), only react/observe

### Handling Groups

- Author = actual user from `from.id`
- User added to users table if new
- Membership tracked in channel_members
- Facts have owner_user_id = author

---

## Migration Plan

### Phase 1: Schema

1. Create `channels` table
2. Create `channel_members` table
3. Add indexes

### Phase 2: Gateway

1. Extract chat metadata from updates
2. Upsert channel on first message
3. Track membership on user messages
4. Pass chat_type, chat_id, message_id to agent

### Phase 3: Agent

1. Add decide_response tool
2. Add query_channel_memory tool
3. Add query_membership tool
4. Add configure_channel tool
5. Update ingest to include source_channel metadata

### Phase 4: Access Control

1. Implement can_access function
2. Filter query results by access
3. Add sensitivity detection
4. Add audit logging

---

## Cost Considerations

**Full LLM Per Message** costs ~1500-2000 tokens per message.

| Activity | Daily Messages | Tokens/Day |
|----------|----------------|------------|
| Quiet group | 10 | 20k |
| Active group | 100 | 200k |
| Very active | 500 | 1M |

**Mitigation options (future):**
- Add tiered classification as config option
- Per-channel budget limits
- Batch processing for observe-only

---

## Testing

### Unit Tests

1. can_access returns correct results for all visibility levels
2. Channel config JSONB parsing
3. Membership tracking (join/leave)
4. Sensitivity pattern detection

### Integration Tests

1. Gateway correctly identifies channel vs group
2. Agent calls decide_response for group messages
3. Facts saved with correct source_channel
4. query_channel_memory respects access control

### E2E Tests

1. Send message in group → Duq responds appropriately
2. Send in channel → Duq observes (no author to respond to)
3. Query fact from group → visible to members only
4. configure_channel changes behavior

---

## Open Questions

None — all clarified during brainstorming.

---

## Appendix: Telegram Chat Types

| Type | chat.type | chat.id | from |
|------|-----------|---------|------|
| Private | "private" | positive | user |
| Group | "group" | negative | user |
| Supergroup | "supergroup" | negative (-100...) | user |
| Channel | "channel" | negative (-100...) | NULL |
