# getcourser

CLI-утилита для скачивания HLS/M3U8-видео с параллельной загрузкой чанков.

## Установка

```bash
git clone https://github.com/your-user/getcourser
cd getcourser
make build
```

Или напрямую через Go:

```bash
go build -o grabber ./cmd/grabber
```

## Использование

### Одно видео

```bash
grabber <playlist_url> <output.mpg>
```

### Пакетная загрузка из файла плейлиста

```bash
grabber -playlist=<file.playlist> <url>
```

### Флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-threads` | `5` | Количество параллельных потоков загрузки |
| `-verbose` | `false` | Подробный вывод |
| `-playlist` | — | Путь к файлу плейлиста для пакетной загрузки |

### Примеры

```bash
# Скачать одно видео
grabber https://example.com/playlist.m3u8 lecture.mpg

# Скачать с 10 потоками
grabber -threads=10 https://example.com/playlist.m3u8 lecture.mpg

# Пакетная загрузка
grabber -playlist=courses.playlist https://example.com
```

## Формат файла плейлиста

Каждая строка — одно видео. Формат строки:

```
filename.mpg https://example.com/playlist.m3u8
```

Если указать только URL без имени файла, имя генерируется автоматически (`video_1`, `video_2`, ...):

```
lesson1.mpg https://example.com/lesson1.m3u8
https://example.com/lesson2.m3u8
lesson3.mpg https://example.com/lesson3.m3u8
```

## Архитектура

```
cmd/grabber/main.go       — парсинг флагов, точка входа
internal/app/app.go       — основная логика (воркер-пул, конкатенация)
internal/m3u/m3u.go       — парсер M3U-формата
```

**Процесс загрузки:**
1. Скачивает M3U8-плейлист во временный файл
2. Парсит URL чанков (строки после `#EXTINF`)
3. Запускает пул горутин, которые скачивают чанки параллельно
4. Упорядочивает чанки по индексу
5. Конкатенирует в итоговый файл и удаляет временные файлы

## Тесты

```bash
go test ./...
```
