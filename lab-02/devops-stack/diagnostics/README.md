# Воспроизведение обязательных поломок

Используйте отдельное имя проекта, чтобы не затронуть работающий стенд:

```bash
# Неправильный hostname БД
docker compose -p lab02-localhost -f docker-compose.yml -f diagnostics/localhost.yml up -d db backend
docker compose -p lab02-localhost -f docker-compose.yml -f diagnostics/localhost.yml ps -a
docker compose -p lab02-localhost -f docker-compose.yml -f diagnostics/localhost.yml logs backend
# В compose exec выводим состояние, а не пароль/DSN.
docker compose -p lab02-localhost -f docker-compose.yml -f diagnostics/localhost.yml exec backend wget -S -O - http://127.0.0.1:8000/health
# Снятие overlay возвращает DNS-имя db.
docker compose -p lab02-localhost -f docker-compose.yml up -d --wait db backend
docker compose -p lab02-localhost -f docker-compose.yml down

# Нет readiness-зависимости, PostgreSQL искусственно задержан на 12 секунд
docker compose -p lab02-race -f docker-compose.yml -f diagnostics/race.yml up -d db backend
docker compose -p lab02-race -f docker-compose.yml -f diagnostics/race.yml logs --timestamps
docker compose -p lab02-race -f docker-compose.yml -f diagnostics/race.yml ps -a
# В первые секунды /health = 503; позднее 200. Вернуть штатную конфигурацию:
docker compose -p lab02-race -f docker-compose.yml up -d --wait db backend
docker compose -p lab02-race -f docker-compose.yml down

# 64 MiB + контролируемое выделение памяти
docker compose -p lab02-oom -f docker-compose.yml -f diagnostics/oom.yml up -d --build db backend
docker compose -p lab02-oom -f docker-compose.yml -f diagnostics/oom.yml ps -a
docker compose -p lab02-oom -f docker-compose.yml -f diagnostics/oom.yml logs backend
docker inspect $(docker compose -p lab02-oom -f docker-compose.yml -f diagnostics/oom.yml ps -a -q backend) --format '{{json .State}}'
docker events --since 5m --until 0s --filter label=com.docker.compose.project=lab02-oom
docker compose -p lab02-oom -f docker-compose.yml -f diagnostics/oom.yml down
```

Сам по себе лимит 64 MiB не гарантирует OOM у небольшого Go API. Поэтому
в диагностическом target добавлен отдельный бинарник, последовательно
выделяющий и заполняющий память. В обычный runtime-образ он не попадает.
Доказательство OOM: `ExitCode=137` **и** `OOMKilled=true` / событие `oom`.
Один код 137 возможен и при обычном SIGKILL.

Диагностировать в порядке: `ps -a` → `logs` → `inspect` → `events` для
завершившегося процесса или `stats/top` для живого → `exec` для проверки сети.
Команда `down` сохраняет тома; повторный запуск использует те же данные.
