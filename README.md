# JARVIS Webhook Gateway

HTTP gateway для приёма webhooks и отправки уведомлений в Telegram через OpenClaw.

## Endpoints

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| GET | /health | - | Healthcheck |
| GET | /docs | BasicAuth | Документация (Obsidian → HTML) |
| GET | /docs/{name} | BasicAuth | Конкретный документ |
| POST | /api/calendar | Token | Google Calendar events |
| POST | /api/gmail | Token | Gmail notifications |
| POST | /api/github | Token | GitHub webhooks |
| POST | /api/custom | Token | Custom webhooks |

## Сборка

```bash
go build -o jarvis-gateway .
```

## Конфигурация

Создайте `config.json` из примера:

```bash
cp config.example.json config.json
```

Или используйте переменные окружения:
- `JARVIS_PORT` - порт (default: 8082)
- `JARVIS_TELEGRAM_CHAT_ID` - Telegram chat ID
- `JARVIS_OPENCLAW_BIN` - путь к openclaw binary
- `JARVIS_TOKEN_CALENDAR` - токен для calendar webhook
- `JARVIS_TOKEN_GMAIL` - токен для gmail webhook
- `JARVIS_TOKEN_GITHUB` - токен для github webhook
- `JARVIS_TOKEN_CUSTOM` - токен для custom webhook

## Запуск

```bash
./jarvis-gateway
```

## API

### Calendar Webhook

```bash
curl -X POST https://on-za-menya.online/api/calendar \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "reminder",
    "event": {
      "title": "Meeting",
      "start_time": "2026-03-09T10:00:00Z",
      "meet_link": "https://meet.google.com/xxx"
    },
    "minutes_before": 15
  }'
```

### Custom Webhook

```bash
curl -X POST https://on-za-menya.online/api/custom \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Hello from webhook!",
    "source": "my-service"
  }'
```

## Systemd

```bash
sudo cp jarvis-gateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable jarvis-gateway
sudo systemctl start jarvis-gateway
```

## Google Apps Script

Для автоматических напоминаний о событиях календаря используйте скрипт:
`scripts/calendar-reminder.gs`

См. [scripts/README.md](scripts/README.md) для инструкции по настройке.

## Authentication

### BasicAuth (для /docs)

Защита документации логином и паролем:

```json
{
  "basic_auth": {
    "username": "user",
    "password": "secret"
  }
}
```

Доступ: `curl -u user:secret https://example.com/docs`

### Token Auth (для /api/*)

Webhook endpoints требуют токен в заголовке:

```
Authorization: Bearer YOUR_TOKEN
```

или

```
X-Webhook-Token: YOUR_TOKEN
```

## Documentation

Endpoint `/docs` отдаёт документацию из Obsidian vault в HTML формате.
Файлы читаются динамически при каждом запросе (изменения отражаются сразу).
Защищён BasicAuth.

Конфиг:
```json
{
  "docs_path": "/opt/obsidian-vault/Coding/OpenClaw"
}
```
