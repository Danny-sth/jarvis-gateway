# ТЗ: Voice Endpoint для jarvis-gateway

## Что нужно Android-приложению

### Request
```
POST /api/voice
Authorization: Bearer <token>
Content-Type: multipart/form-data

Body: audio=<WAV file, 16kHz, mono, 16-bit PCM>
```

### Response (Success)
```
HTTP 200 OK
Content-Type: audio/ogg

<binary OGG Vorbis audio>
```

### Response (Error)
```
HTTP 4xx/5xx
Content-Type: application/json

{"error": "описание"}
```

## Логика
1. Принять WAV
2. STT + OpenClaw + TTS (детали на усмотрение бэкенда)
3. Вернуть OGG с голосовым ответом

## Токен
Добавить в config.json:
```json
"tokens": {
  "voice": "voice-xxx"
}
```
