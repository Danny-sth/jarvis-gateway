# ⛔⛔⛔ ЗАПРЕТ ЛОКАЛЬНОГО ТЕСТИРОВАНИЯ ⛔⛔⛔

## КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО:
- Запускать docker контейнеры локально
- Создавать .venv локально  
- Запускать pytest/go test локально
- Создавать локальные БД
- Запускать сервисы локально
- pip install / go build локально
- ЛЮБОЕ локальное тестирование

## ВСЕ ТЕСТИРОВАНИЕ ТОЛЬКО НА VPS

### Сервер:
```
IP: 90.156.230.49
SSH: ssh root@90.156.230.49
```

### Сервисы на сервере:
```
duq.service      -> /opt/duq/current       -> :8081
duq-gateway      -> /opt/duq-gateway       -> :8082  
PostgreSQL       -> duq база данных
Redis            -> duq:* ключи
```

### Деплой:
```bash
ssh root@90.156.230.49
cd /opt/duq/current
git pull
source .venv/bin/activate
pip install -e .
systemctl restart duq
```

## НАРУШЕНИЕ = НЕМЕДЛЕННОЕ ПРЕКРАЩЕНИЕ РАБОТЫ
