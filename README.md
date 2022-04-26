# Поисковая система

## Функционал

- [x] Подборка релевантных документов
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
- [x] Поддержка англицизмов / профессионализмов / жаргонизмов / синонимов / антонимов / паронимов / омофонов
- [x] Реализация поддержки ключевых слов с использованием ранга по частотности внутри соответствующего списка
- [x] Реализация загрузки констант из .env
- [x] Реализация загрузки словарей их папки
- [x] Реализация загрузки словаря стоп-слов
- [x] Поддержка аргументов командной строки с поддержкой всех значений из .env
- [x] Логирование на уровне операционной системы
- [ ] Работа с сигналами для запуска приложение в качестве службы

## Терминология

**Хит** — документ, в котором найдено хотя бы одно вхождение слов поисковой фразы (по сути единица результатов обработки запроса к поисковой системы).

**Пересечение результатов (intersection)** — случай, когда нужно найти те документы, в которых обязательно встречаются все слова поискового запроса, перечисленные через знак `+`.

**Исключение из результатов (subtraction)** — случай, когда нужно исключить те документы, в которых встречаются слова поискового запроса со знаком `-` перед ними.

**Категория** — кластер корпуса документов (необходимо использовать поле `category`).

**Тег** — маркер документа, который используется для лучшей навигации и таксономизации корпуса текстов (необходимо использовать поле `tags`).

**Ключевые слова** — отдельные слова или фразы, при поиске по которым хит обретает наивысший ранг (частотность при этом определяется внутри списка ключевых слов, а не по тексту в целом).

## Настройка

### Использование `.env`

Параметры для работы сервиса:

- `SEARCH_CONTENT` — используется для определения пути к файлу с контентом (обязательный параметр)
- `STOP_WORDS` — используется для определения словаря стоп-слов
- `DICTS_DIR` — используется для определения папки с другими словарями преобразования
- `APP_NAME` — название приложения (фигурирует в названии файла логов наряду с текущим временем, значение по умолчанию `SEARCH-DB-LESS`)
- `APP_HOST` — используется для определения хоста веб-сервиса (значение по умолчанию `""`)
- `APP_PORT` — используется для определения порта веб-сервиса (обязательный параметр, значение по умолчанию `8080`)

Параметры для настройки отображения хитов:

- `MARKER` — тег для выделения поисковой фразы в хитах (значение по умолчанию `mark`)
- `DISTANCE_BETWEEN_WORDS` — максимальная количество символов между искомыми словами в тексте при пересечении (значение по умолчанию `20`)
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
- `-l`, `--app-log` — количество записей в логе, после которых данные сохраняются в файл (значение по умолчанию `1000`)

Параметры для настройки отображения хитов:

- `--words-marker-tag` — тег для выделения поисковой фразы в хитах (значение по умолчанию `mark`)
- `--words-distance-between` — максимальная количество символов между искомыми словами в тексте при пересечении (значение по умолчанию `20`)
- `--words-trimmer-placeholder` — строка, которая ставится на концах  обрезки (значение по умолчанию `...`)
- `--words-occurrences` — нижний порог встречаемости слова внутри параграфа (влияет на показ хитов) (значение по умолчанию `-1`)
- `--words-around-range` — количество символов до и после для понимания контекста использования искомого слова (значение по умолчанию `42`)
- `--words-distance-limit` — редакционное расстояние для основ слов в запросе и в поисковом индексе (значение по умолчанию `3`)
