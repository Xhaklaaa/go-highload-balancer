# Go Highload Balancer

HTTP-балансировщик нагрузки с поддержкой rate-limiting и health checks

## Особенности
- 🌀 Поддержка алгоритмов балансировки: Round Robin и Least Connections
- 🚦 Rate Limiting на основе алгоритма Token Bucket
- 🩺 Регулярные health checks бэкендов
- 📦 Конфигурация через YAML-файл или переменные окружения
- 🐳 Готовые Docker-образы и docker-compose конфигурация
- 📡 API для управления клиентами и их лимитами
- 🔒 Graceful shutdown

## Быстрый старт

### Требования
- Go 1.23
- PostgreSQL 14+ (для персистентного хранилища)
- Docker 20.10+

### Сборка и запуск
```bash
# Клонировать репозиторий
git clone https://github.com/yourusername/go-highload-balancer.git
cd go-highload-balancer
# Запустить контейнеры
docker-compose up -d
# С помощью postman или curl  проверить работу балансировщика
curl http://localhost:8080
```

### API Endpoints
# POST /api/v1/clients
```
curl -X POST http://localhost:8080/api/v1/clients \
  -H "Content-Type: application/json" \
  -d '{"client_id": "testuser1", "capacity": 5, "rate_per_sec": 1}'
```

{
    "client_id": "client1",
    "capacity": 100,
    "rate_per_sec": 10
}

# GET /api/v1/clients/{client_id}
```
curl -X GET http://localhost:8080/api/v1/clients/testuser1
```
{
    "client_id": "client1",
    "capacity": 100,
    "rate_per_sec": 10
}

# PUT /api/v1/clients/{client_id}
```
curl -X PUT http://localhost:8080/api/v1/clients/testuser1 \
  -H "Content-Type: application/json" \
  -d '{
        "capacity": 10,
        "rate_per_sec": 2
      }'
```
{
    "client_id": "testuser1",
    "capacity": 10,
    "rate_per_sec": 2
}
# DELETE /api/v1/clients/{client_id}
```
curl -X DELETE http://localhost:8080/api/v1/clients/testuser1
```
{
    "code": 204, "message": "No content"
}
