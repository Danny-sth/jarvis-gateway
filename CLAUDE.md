# ⛔⛔⛔ ЗАПРЕТ ЛОКАЛЬНОГО ТЕСТИРОВАНИЯ ⛔⛔⛔

## КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО:
- Запускать docker контейнеры локально
- Создавать .venv локально
- Запускать pytest/go test локально
- Создавать локальные БД
- Запускать сервисы локально
- ЛЮБОЕ локальное тестирование

## ⛔⛔⛔ ВСЕ ПРАВКИ ЧЕРЕЗ GIT ⛔⛔⛔

```
НИКАКОГО RSYNC! НИКАКОГО SCP!
ВСЕ ИЗМЕНЕНИЯ КОДА ТОЛЬКО ЧЕРЕЗ GIT!

1. Локально: git commit && git push
2. На VPS: git pull
3. Docker rebuild

НАРУШЕНИЕ = ПОТЕРЯ ВРЕМЕНИ НА ДЕБАГ НЕСИНХРОНИЗИРОВАННОГО КОДА!
```

## НИКОГДА НЕ ИСПОЛЬЗУЙ:
- `rsync` для деплоя кода
- `scp` для копирования файлов на VPS
- `sleep` в командах
- Плейсхолдеры или заглушки

## ПЕРЕД СЛОВОМ "ГОТОВО":
1. `grep -r "удаляемый_паттерн"` по ВСЕМУ проекту
2. Компиляция ДОЛЖНА пройти
3. Сервис ДОЛЖЕН запуститься на VPS

## ВСЕ ТЕСТИРОВАНИЕ ТОЛЬКО НА VPS

### Сервер:
```
IP: 90.156.230.49
SSH: ssh root@90.156.230.49
```

### Структура на VPS:
```
/opt/duq-dev/          ← КОД (git repos)
├── duq/
├── duq-gateway/
├── duq-admin/
└── duq-tracing/

/opt/duq-deploy/       ← КОНФИГИ (docker-compose, .env)
├── docker-compose.yml
├── .env
└── scripts/
```

### Docker Сервисы:
```
cd /opt/duq-deploy && ./deploy.sh deploy

duq-core         -> :8081
duq-gateway      -> :8082
duq-admin        -> :5000
duq-postgres     -> :5432
duq-redis        -> :6379
duq-keycloak     -> :8180
```

## ДЕПЛОЙ GATEWAY (ПРАВИЛЬНО):
```bash
# 1. Локально: коммит и пуш
git add -A && git commit -m "description" && git push

# 2. На VPS: pull и rebuild
ssh root@90.156.230.49 "cd /opt/duq-dev/duq-gateway && git pull && cd /opt/duq-deploy && docker compose build duq-gateway && docker compose up -d duq-gateway"

# 3. Проверить логи
ssh root@90.156.230.49 "docker logs duq-gateway --tail=30"
```

## ДЕПЛОЙ DUQ (Python):
```bash
# 1. Локально: коммит и пуш
git add -A && git commit -m "description" && git push

# 2. На VPS: pull и rebuild
ssh root@90.156.230.49 "cd /opt/duq-dev/duq && git pull && cd /opt/duq-deploy && docker compose build duq && docker compose up -d duq"
```

## НАРУШЕНИЕ = НЕМЕДЛЕННОЕ ПРЕКРАЩЕНИЕ РАБОТЫ
