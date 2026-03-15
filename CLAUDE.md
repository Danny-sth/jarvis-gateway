# JARVIS Gateway

HTTP gateway для webhooks и голосового API. Go сервис.

## Архитектура

```
┌─────────────────────────────────────────────────────────────────┐
│                         ИНТЕРНЕТ                                │
└─────────────────────────────────────────────────────────────────┘
        │              │              │              │
        ▼              ▼              ▼              ▼
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │ Telegram │  │  GitHub  │  │ Calendar │  │  Mobile  │
  │  Voice   │  │ Webhooks │  │  Gmail   │  │   App    │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘
        │              │              │              │
        └──────────────┴──────────────┴──────────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │   nginx :443        │
                    │ on-za-menya.online  │
                    └─────────────────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │  jarvis-gateway     │
                    │      :8082          │
                    └─────────────────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
        ▼                      ▼                      ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│  STT/TTS      │    │  PostgreSQL   │    │   Vtoroy      │
│ whisper/edge  │    │   :5433       │    │   :8081       │
└───────────────┘    └───────────────┘    └───────────────┘
```

## Endpoints

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| GET | /health | - | Healthcheck |
| GET | /docs | BasicAuth | Документация |
| POST | /api/telegram/webhook | - | Telegram (text + voice) |
| POST | /api/github | HMAC | GitHub webhooks |
| POST | /api/calendar | Token | Calendar webhooks |
| POST | /api/gmail | Token | Gmail webhooks |
| POST | /api/auth/qr/generate | Token | QR для мобилки |
| POST | /api/auth/qr/verify | - | Верификация QR |
| POST | /api/voice | Mobile Token | Голосовой endpoint |

## Telegram Voice Flow

```
Voice message (OGA)
       │
       ▼
Download via Bot API
       │
       ▼
FFmpeg: OGA → WAV (16kHz mono)
       │
       ▼
whisper-stt → текст
       │
       ▼
Vtoroy /api/chat → ответ
       │
       ▼
edge-tts → MP3
       │
       ▼
FFmpeg: MP3 → OGG Opus
       │
       ▼
Telegram sendVoice:
  - <= 1024 chars: voice + caption (одно сообщение)
  - > 1024 chars: text + voice (два сообщения)
```

## Mobile API Flow

```
QR scan (/link в Telegram)
       │
       ▼
/api/auth/qr/verify → mob_token
       │
       ▼
/api/voice + Bearer mob_token
       │
       ▼
WAV → STT → Vtoroy → TTS → OGG (base64)
```

## Структура проекта

```
internal/
├── config/       # Конфигурация (env + json)
├── db/           # PostgreSQL (sessions, qr_codes)
├── handlers/     # HTTP handlers
│   ├── telegram.go   # Telegram + STT/TTS
│   ├── voice.go      # Mobile voice
│   ├── auth.go       # QR auth
│   ├── github.go     # GitHub webhooks
│   └── ...
├── middleware/   # Auth middleware
├── vtoroy/       # Vtoroy HTTP client
└── voice/        # STT/TTS wrappers
```

## Конфигурация

Environment:
- `TELEGRAM_BOT_TOKEN` - для скачивания voice
- `JARVIS_DB_*` - PostgreSQL
- `JARVIS_TOKEN_*` - токены для webhooks
- `VTOROY_URL` - URL vtoroy API (default: http://localhost:8081)

## База данных

PostgreSQL `jarvis@localhost:5433/jarvis`:
- `mobile_sessions` - сессии мобилок (30 дней)
- `qr_auth_codes` - QR коды (5 минут)

## VPS

- Локация: Алматы, Казахстан
- Провайдер: Timeweb
- IP: 90.156.230.49

## Деплой

```bash
# Сборка
GOOS=linux GOARCH=amd64 go build -o jarvis-gateway-linux .

# Деплой
scp jarvis-gateway-linux root@90.156.230.49:/usr/local/bin/jarvis-gateway
ssh root@90.156.230.49 "systemctl restart jarvis-gateway"
```

## Связанные системы

- **Vtoroy** - AI agent (localhost:8081, /api/chat)
- **PostgreSQL** - сессии и QR коды (:5433)
- **Obsidian** - документация (/opt/obsidian-vault)
