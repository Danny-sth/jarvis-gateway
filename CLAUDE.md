# ⛔⛔⛔ ЗАПРЕТ ЛОКАЛЬНОГО ТЕСТИРОВАНИЯ ⛔⛔⛔

## КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО:
- Запускать docker контейнеры локально
- Создавать .venv локально
- Запускать pytest/go test локально
- Создавать локальные БД
- Запускать сервисы локально
- ЛЮБОЕ локальное тестирование

## НИКОГДА НЕ ИСПОЛЬЗУЙ:
- `sleep` в командах
- `git clone` на VPS
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

### Docker Сервисы (АКТУАЛЬНО):
```
cd /opt/duq-deploy && ./deploy.sh deploy

duq-core         -> :8081
duq-gateway      -> :8082
duq-admin        -> :5000
duq-postgres     -> :5432
duq-redis        -> :6379
duq-keycloak     -> :8180
```

### Legacy systemd (УСТАРЕЛО):
```
duq.service      -> /opt/duq/current       -> :8081  (ОТКЛЮЧЕН)
duq-gateway      -> /opt/duq-gateway       -> :8082  (ОТКЛЮЧЕН)
```

## ДЕПЛОЙ GATEWAY (Docker - АКТУАЛЬНО):
```bash
# 1. Синхронизировать код
rsync -avz --exclude='.git' /home/danny/Documents/projects/duq-gateway/ root@90.156.230.49:/opt/duq-deploy/duq-gateway/

# 2. Пересобрать и перезапустить
ssh root@90.156.230.49 "cd /opt/duq-deploy && docker compose build duq-gateway && docker compose up -d duq-gateway"

# 3. Проверить
ssh root@90.156.230.49 "docker compose -f /opt/duq-deploy/docker-compose.yml logs duq-gateway --tail=20"
```

## LEGACY ДЕПЛОЙ GATEWAY (Go - УСТАРЕЛО):
```bash
# 1. Локальный билд (ОБЯЗАТЕЛЬНО с флагами оптимизации!)
cd /home/danny/Documents/projects/duq-gateway
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o gateway-bin .

# 2. Остановить сервис
ssh root@90.156.230.49 "systemctl stop duq-gateway; rm -f /opt/duq-gateway/duq-gateway"

# 3. Скопировать бинарник
scp gateway-bin root@90.156.230.49:/opt/duq-gateway/duq-gateway

# 4. Запустить
ssh root@90.156.230.49 "chmod +x /opt/duq-gateway/duq-gateway && systemctl start duq-gateway && systemctl status duq-gateway --no-pager"
```

## ДЕПЛОЙ DUQ (Python):
```bash
ssh root@90.156.230.49 "cd /opt/duq/current && git pull && source .venv/bin/activate && pip install -e . && systemctl restart duq"
```

## НАРУШЕНИЕ = НЕМЕДЛЕННОЕ ПРЕКРАЩЕНИЕ РАБОТЫ
