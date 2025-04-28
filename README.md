# go-highload-balancer
Test task for Cloud.ru golang developer


## Алгоритмы балансировки
- `round-robin` (по умолчанию)
- `least-connections`

## Переменные окружения
| Переменная       | Описание                     |
|------------------|------------------------------|
| STRATEGY         | Алгоритм балансировки        |
| HEALTH_CHECK_INT | Интервал проверок (секунды)  |

## API Endpoints
POST /clients
```json
{
    "client_id": "client1",
    "capacity": 100,
    "rate_per_sec": 10
}