# Практика 2: Docker в production

Исходный текст задания: [практика 2 (2).pdf](практика%202%20(2).pdf).

Приложение заметок из задания:
Vite SPA → nginx HTTPS → Go REST API → PostgreSQL.
Исходники находятся в `devops-stack/`, фактические результаты — в [REPORT.md](REPORT.md),
протоколы команд — в `evidence/`.

## Запуск на Windows

Требуется запущенный Docker Desktop с Linux containers. Go, Node.js и OpenSSL
на Windows устанавливать не требуется: сборки выполняются в контейнерах.

```powershell
cd E:\DevOps_Lab\DevOps_Lab\lab-02\devops-stack
.\scripts\init.ps1
docker compose -p devops-lab02 config --quiet
docker compose -p devops-lab02 up -d --build --wait
.\scripts\smoke.ps1
```

Откройте <https://localhost:8443> и примите предупреждение о self-signed сертификате.
`http://localhost:8088` перенаправляет на HTTPS. Внешний HTTP-порт 8088 выбран,
чтобы не занимать системный порт 80; внутри контейнера используется непривилегированный
порт 8080, nginx работает от пользователя `nginx`.

`init.ps1` создаёт `.env` со случайным паролем и сертификат с SAN для localhost.
Повторный запуск сохраняет имеющиеся пароль и сертификат. `.env` и `nginx/ssl/`
исключены из Git и контекста Docker. Не меняйте пароль `.env` у существующего
PostgreSQL-тома без смены пароля роли в самой БД.

## Запуск на Linux / control node

Нужны Docker, Compose plugin, BuildKit, `openssl` и shell.

```bash
cd lab-02/devops-stack
sh scripts/init.sh
docker compose -p devops-lab02 up -d --build --wait
curl -k https://localhost:8443/health
curl -k https://localhost:8443/api/notes
curl -k -X POST https://localhost:8443/api/notes \
  -H 'Content-Type: application/json' -d '{"title":"first","body":"hello"}'
```

Сервисы опубликованы только на `127.0.0.1`. Для просмотра удалённого control node
используйте `ssh -L 8443:localhost:8443 user@control-node` и локальный браузер.
Адреса, inventory и SSH-доступ предыдущей работы в репозитории отсутствуют.

## API и эксплуатация

| Запрос | Ответ |
| --- | --- |
| `GET /health` | 200 `{"status":"ok","db":true}`; 503 при недоступной БД |
| `GET /api/notes` | JSON-массив заметок; пустая БД возвращает `[]` |
| `POST /api/notes` | JSON `title`/`body`, 201 и ID; title обязателен, максимум 200 символов |
| `GET /api/notes/{id}` | 200 и заметка; 404 для отсутствующего ID; 400 для неверного ID |

Запросы ограничены по времени и размеру. SQL параметризован. SPA использует
относительные `/api/notes` и `/health`, поэтому работает через HTTPS без mixed content;
пользовательский текст вставляется через `textContent`.

```bash
docker compose -p devops-lab02 ps -a
docker compose -p devops-lab02 logs --tail 50 backend
docker compose -p devops-lab02 stats --no-stream
docker compose -p devops-lab02 top backend
docker compose -p devops-lab02 stop backend
docker compose -p devops-lab02 logs backend
docker compose -p devops-lab02 up -d --wait
docker compose -p devops-lab02 --profile debug up -d adminer
docker compose -p devops-lab02 --profile debug stop adminer
docker compose -p devops-lab02 down
```

Adminer: `http://localhost:8081`, система PostgreSQL, сервер `db`, пользователь и БД
`notes`, пароль из локального `.env`. Остановка `down` сохраняет именованный том;
`down --volumes` удаляет данные и для обычного завершения работы не нужен.

## Сборка и проверки

```bash
# Unit-тесты без живой БД; go test ./... также запускает tests/
cd backend
go test ./... -v
go vet ./...
# Альтернатива без локальной установки Go
docker build --target test -t devops-lab02-api-test .
docker build --target runtime -t notes-api .
docker build --target runtime -t notes-api . # CACHED
docker build -f Dockerfile.bad -t notes-api-bad .
docker image inspect notes-api notes-api-bad --format '{{.RepoTags}} {{.Size}}'
docker image history notes-api
cd ../frontend
npm ci && npm run build
docker build -t notes-web .
cd ..
```

В backend зависимости копируются и скачиваются до исходников; применяется
BuildKit cache для модулей и компиляции. Итоговый runtime содержит статический
Go-бинарник и отдельный healthcheck-скрипт. Во frontend `package-lock.json` +
`npm ci` обеспечивают воспроизводимый набор зависимостей, Node.js остаётся в builder.
Оба приложения и шлюз работают без root. Exec form `CMD` передаёт SIGTERM процессу.

Учебный `Dockerfile.bad` содержит минимум семь проблем: полный SDK в runtime,
нет multi-stage, root, жёстко заданные реквизиты БД, `localhost` вместо DNS,
`COPY . .` до зависимостей и shell form CMD. Он используется только для сравнения.

## Самопроверка и дополнительные задания

Поломки описаны в [diagnostics/README.md](devops-stack/diagnostics/README.md).
Каждая воспроизводится отдельным Compose-проектом; основной стенд сохраняется.
64 MiB недостаточно, чтобы автоматически вызвать OOM у небольшого Go API,
поэтому отдельный диагностический target выполняет контролируемое выделение памяти.

Для разработки без TLS и с прямым портом API:

```bash
docker compose -p devops-lab02-dev -f docker-compose.yml -f docker-compose.dev.yml up -d --build
# http://localhost:8088, API http://localhost:8000
```

Сначала остановите основной проект, если используете те же опубликованные порты.
Файл назван `docker-compose.dev.yml`, чтобы Compose не применял отладочные настройки
автоматически к штатному HTTPS-запуску. Включение явное через `-f`.

Пример BuildKit secret для release metadata:

```bash
mkdir -p frontend/secrets
printf 'lab02-release\n' > frontend/secrets/release.txt
docker build --build-arg BUILD_VARIANT=lab02-release \
  --secret id=release,src=frontend/secrets/release.txt -t notes-web-release frontend
```

Исходный secret-файл не попадает в контекст. Значение release намеренно отображается
в SPA, поэтому здесь допустимы только публичные метаданные, а не токены.
Смена содержимого BuildKit secret не сбрасывает cache автоматически:
при изменении release меняйте публичный `BUILD_VARIANT` либо пересоберите
builder с `--no-cache-filter builder`. Ключ cache не содержит токенов.

Полный набор обязательных проверок на Windows можно повторить командой
`.\scripts\verify.ps1`. Он использует основной проект `devops-lab02`, временно
перезапускает его для проверки данных и создаёт отдельные проекты для поломок.
Перед запуском сохраните свою работу в интерфейсе.

Для очистки места сначала выполните `docker system df -v`. Общая команда
`docker system prune` удаляет неиспользуемые ресурсы всех проектов, поэтому
на общем компьютере применяйте адресную очистку. Во время этой работы удалён
только неиспользуемый OOM-образ по label `com.docker.compose.project=lab02-oom`;
тома сохранены.

## Исправления примеров задания

- `pgxpool` — пакет внутри модуля `pgx/v5`, а не отдельный модуль в `go.mod`.
- INSERT теперь сканирует `RETURNING id` в `&id`; цикл SELECT проверяет `rows.Err()`.
- Исправлен приём сигнала: `<-ctx.Done()` вместо `<-ctx`.
- Unit-тесты импортируют `internal/api`; отдельная папка `tests/` не видит
  неэкспортированные функции пакета `main` автоматически.
- `/health` проксируется к backend; в исходной конфигурации путь уходил во frontend.
- Readiness возвращает 503 при отказе БД: Docker healthcheck действительно замечает отказ.
- Создан каталог для сертификатов; современный Compose передаёт `build.secrets`
  напрямую, ручная смена `build` на `image` не требуется.
- Копирование self-signed ключа из builder **сохраняет его в конечном образе**.
  BuildKit скрывает исходный mount, но не результат `COPY`; это учебное исключение
  из задания. Рабочие TLS-ключи следует подключать во время запуска.

Использованы Go 1.25, Node 22, nginx 1.28, PostgreSQL 16, Vite 6.4.3 вместо
устаревших/ошибочных фрагментов PDF. Возможности Compose сверены с
[документацией build secrets](https://docs.docker.com/compose/how-tos/use-secrets/),
а поведение mount — с [документацией BuildKit](https://docs.docker.com/build/building/secrets/).
