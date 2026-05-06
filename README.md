# getcourser

CLI-утилита для скачивания HLS/M3U8-видео с параллельной загрузкой чанков. Поддерживает прямые M3U8-ссылки, пакетную загрузку и ссылки на видео с Яндекс Диска.

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

### Одно видео по M3U8-ссылке

```bash
grabber <playlist_url> <output.mpg>
```

### Видео с Яндекс Диска

```bash
grabber -yadisk=<share_url> [output.mpg]
```

Утилита скачивает страницу, находит все доступные качества и выбирает наилучшее. Чтобы указать конкретное качество:

```bash
grabber -yadisk=<share_url> -quality=720p [output.mpg]
```

### Пакетная загрузка из файла плейлиста

```bash
grabber -playlist=<file.playlist>
```

## Флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-threads` | `5` | Количество параллельных потоков загрузки |
| `-verbose` | `false` | Подробный вывод |
| `-playlist` | — | Путь к файлу плейлиста для пакетной загрузки |
| `-yadisk` | — | URL страницы Яндекс Диска с видео |
| `-quality` | лучшее | Желаемое качество видео: `240p`, `360p`, `480p`, `720p` и т.д. |

## Примеры

```bash
# Скачать одно видео
grabber https://example.com/playlist.m3u8 lecture.mpg

# Скачать с 10 потоками
grabber -threads=10 https://example.com/playlist.m3u8 lecture.mpg

# Скачать с Яндекс Диска (лучшее доступное качество)
grabber -yadisk=https://disk.yandex.ru/i/XxXxXxX lecture.mpg

# Скачать с Яндекс Диска в конкретном качестве
grabber -yadisk=https://disk.yandex.ru/i/XxXxXxX -quality=480p lecture.mpg

# Пакетная загрузка
grabber -playlist=courses.playlist
```

## Формат файла плейлиста

Каждая строка — одно видео:

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
cmd/grabber/main.go         — парсинг флагов, точка входа
internal/app/app.go         — основная логика (воркер-пул, конкатенация)
internal/m3u/m3u.go         — парсер M3U-формата
internal/yadisk/yadisk.go   — загрузка страницы Яндекс Диска и выбор качества
```

**Процесс загрузки:**
1. (Для `-yadisk`) Скачивает HTML-страницу, извлекает список потоков с качествами, выбирает нужный M3U8 URL
2. Скачивает M3U8-плейлист во временный файл
3. Парсит URL чанков (строки после `#EXTINF`)
4. Запускает пул горутин, которые скачивают чанки параллельно
5. Упорядочивает чанки по индексу
6. Конкатенирует в итоговый файл и удаляет временные файлы

## Тесты

```bash
go test ./...
```
