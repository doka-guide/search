# Поисковая система

## Функционал

- [x] Подборка релевантных документов по содержанию и заголовку
- [x] Пересечение результатов (intersection) при запросе по нескольким словам через `+`
- [x] Исключение из результатов (subtraction) при запросе по нескольким словам через `-`
- [x] Фильтрация результатов по категориям документов
- [x] Фильтрация результатов по тегам документов
- [x] Использование фильтра стоп-слов
- [x] Подсветка слов в результатах (расстояние между словами в абзаце не более установленного)
- [x] Поддержка возможности ошибок в слове
- [x] Поддержка ошибок при использовании неправильной раскладки клавиатуры
- [x] Поддержка ошибок при использовании неправильной раскладки клавиатуры
- [x] Маркировка результатов для всех форм слова (выделение основы слова)
- [x] Расчёт времени поиска
- [x] Исправление ошибок в выводе результатов (повторений столько, сколько результатов)
- [x] Поддержка словарей трансформации
- [x] Реализация поддержки ключевых слов с использованием ранга по частотности внутри соответствующего списка
- [x] Реализация загрузки констант из .env
- [x] Реализация загрузки словарей трансформации из папки
- [x] Реализация загрузки словаря стоп-слов
- [x] Поддержка аргументов командной строки с поддержкой всех значений из .env
- [x] Логирование на уровне файлов операционной системы
- [x] Обработка сигналов операционных систем из семейства Unix
- [x] Сборка и работа приложения внутри контейнера

## Терминология

**Хит** — документ, в котором найдено хотя бы одно вхождение слов поисковой фразы (по сути единица результатов обработки запроса к поисковой системы).

**Пересечение результатов (intersection)** — случай, когда нужно найти те документы, в которых обязательно встречаются все слова поискового запроса, перечисленные через знак `+`.

**Исключение из результатов (subtraction)** — случай, когда нужно исключить те документы, в которых встречаются слова поискового запроса со знаком `-` перед ними.

**Категория** — кластер корпуса документов (необходимо использовать поле `category`).

**Тег** — маркер документа, который используется для лучшей навигации и таксономизации корпуса текстов (необходимо использовать поле `tags`).

**Ключевые слова** — отдельные слова или фразы, при поиске по которым хит обретает наивысший ранг (частотность при этом определяется внутри списка ключевых слов, а не по тексту в целом).

## Запуск

### Сборка и запуск в обычном окружении Go

Пример команды для запуска приложения при наличии файла с переменными окружения `.env`:

```bash
go build main.go && ./main
```

Пример команды для запуска приложения с помощью аргументов командной строки:

```bash
go build main.go && ./main --search-content search-content.json --stop-words stop-search.json --dicts-dir dics --app-port 8080
```

### Сборка и запуск внутри контейнера Docker

Пример команды для сборки образа необходимо выполнить команду:

```bash
docker build -t search .
```

Пример команды для запуска приложения в контейнере при наличии файла с переменными окружения `.env`:

```bash
docker run -ti --rm -p 8080:8080 --name search --mount type=bind,source="$(pwd)",target=/app/data search
```

Пример команды для приложения в контейнере с помощью аргументов командной строки:

```bash
docker run -ti --rm -p 8080:8080 --name search --mount type=bind,source="$(pwd)",target=/app/data search --search-content data/search-content.json --stop-words data/stop-search.json --dicts-dir data/dics --app-port 8080
```

## Настройка

### Использование `.env`

Параметры для работы сервиса:

- `SEARCH_CONTENT` — используется для определения пути к файлу с контентом (обязательный параметр)
- `STOP_WORDS` — используется для определения словаря стоп-слов
- `DICTS_DIR` — используется для определения папки с другими словарями преобразования
- `APP_NAME` — название приложения (фигурирует в названии файла логов наряду с текущим временем, значение по умолчанию `SEARCH-DB-LESS`)
- `APP_HOST` — используется для определения хоста веб-сервиса (значение по умолчанию `""`)
- `APP_PORT` — используется для определения порта веб-сервиса (обязательный параметр, значение по умолчанию `8080`)
- `APP_LOG_LIMIT` — количество записей в логе, после которых данные сохраняются в файл (значение по умолчанию `100`)

Параметры для настройки отображения хитов:

- `WORDS_MARKER_TAG` — тег для выделения поисковой фразы в хитах (значение по умолчанию `mark`)
- `WORDS_DISTANCE_BETWEEN` — максимальная количество символов между искомыми словами в тексте при пересечении (значение по умолчанию `20`)
- `WORDS_TRIMMER_PLACEHOLDER` — строка, которая ставится на концах  обрезки (значение по умолчанию `...`)
- `WORDS_OCCURRENCES` — нижний порог встречаемости слова внутри параграфа (влияет на показ хитов) (значение по умолчанию `-1`)
- `WORDS_AROUND_RANGE` — количество символов до и после для понимания контекста использования искомого слова (значение по умолчанию `42`)
- `WORDS_DISTANCE_LIMIT` — редакционное расстояние для основ слов в запросе и в поисковом индексе (значение по умолчанию `3`)

### Использование аргументов командной строки

Параметры для работы сервиса:

- `-c`, `--search-content` — используется для определения пути к файлу с контентом (обязательный параметр)
- `-w`, `--stop-words` — используется для определения словаря стоп-слов
- `-d`, `--dicts-dir` — используется для определения папки с другими словарями преобразования
- `-n`, `--app-name` — название приложения (фигурирует в названии файла логов наряду с текущим временем, значение по умолчанию `SEARCH-DB-LESS`)
- `-h`, `--app-host` — используется для определения хоста веб-сервиса (значение по умолчанию `""`)
- `-p`, `--app-port` — используется для определения порта веб-сервиса (обязательный параметр, значение по умолчанию `8080`)
- `-l`, `--app-log` — количество записей в логе, после которых данные сохраняются в файл (значение по умолчанию `100`)

Параметры для настройки отображения хитов:

- `--words-marker-tag` — тег для выделения поисковой фразы в хитах (значение по умолчанию `mark`)
- `--words-distance-between` — максимальная количество символов между искомыми словами в тексте при пересечении (значение по умолчанию `20`)
- `--words-trimmer-placeholder` — строка, которая ставится на концах  обрезки (значение по умолчанию `...`)
- `--words-occurrences` — нижний порог встречаемости слова внутри параграфа (влияет на показ хитов) (значение по умолчанию `-1`)
- `--words-around-range` — количество символов до и после для понимания контекста использования искомого слова (значение по умолчанию `42`)
- `--words-distance-limit` — редакционное расстояние для основ слов в запросе и в поисковом индексе (значение по умолчанию `3`)

## Формирование поискового запроса

Поисковый запрос реализуется методом GET к серверу. Используются следующие поля:

- `search` — для поисковой фразы;
- `category` — фильтрация хитов по категориям материалов;
- `tags` — (массив значений) фильтрация хитов по тегам.

## Формат вывода результатов

Ответ на поисковый запрос возвращается в формате JSON, в виде массива хитов, каждый из которых представлен следующей JSON-схемой:

```javascript
[
  {
    // Заголовок материала
    "title": ""
    // Ссылка на материал
    "link": ""
    // Список фрагментов контента, в которых встречается поисковая фраза
    "fragments": [ "" ]
    // Теги найденного материала
    "tags": [ "" ]
    // Категория найденного материала
    "category": ""
  }
]
```
