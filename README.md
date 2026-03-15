# JARVIS Webhook Gateway

HTTP gateway для приёма webhooks и голосового API. Отправляет уведомления через Vtoroy.

## Endpoints

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| GET | /health | - | Healthcheck |
| GET | /docs | BasicAuth | Документация (Obsidian → HTML) |
| GET | /docs/{name} | BasicAuth | Конкретный документ |
| POST | /api/telegram/webhook | - | Telegram (text + voice) |
| POST | /api/calendar | Token | Google Calendar events |
| POST | /api/gmail | Token | Gmail notifications |
| POST | /api/github | HMAC | GitHub webhooks |
| POST | /api/auth/qr/generate | Token | QR для мобилки |
| POST | /api/auth/qr/verify | - | Верификация QR |
| POST | /api/voice | Mobile Token | Голосовой endpoint |

## Сборка

```bash
go build -o jarvis-gateway .
```

## Конфигурация

Создайте `config.json` из примера:

```bash
cp config.example.json config.json
```

Переменные окружения:
- `JARVIS_PORT` - порт (default: 8082)
- `JARVIS_TELEGRAM_CHAT_ID` - Telegram chat ID
- `VTOROY_URL` - URL Vtoroy API (default: http://localhost:8081)
- `JARVIS_TOKEN_CALENDAR` - токен для calendar webhook
- `JARVIS_TOKEN_GMAIL` - токен для gmail webhook
- `JARVIS_TOKEN_GITHUB` - токен для github webhook

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

### Voice API

```bash
curl -X POST https://on-za-menya.online/api/voice \
  -H "Authorization: Bearer MOB_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"audio": "base64_wav_data"}'
```

## Systemd

```bash
sudo cp jarvis-gateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable jarvis-gateway
sudo systemctl start jarvis-gateway
```

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

## Documentation

Endpoint `/docs` отдаёт документацию из Obsidian vault в HTML формате.
Файлы читаются динамически при каждом запросе.
Защищён BasicAuth.

## Связанные системы

- **Vtoroy** - AI agent (localhost:8081)
- **PostgreSQL** - сессии и QR коды
