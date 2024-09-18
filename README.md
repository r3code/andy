# Andy

Инструмент для выполнения SQL-скриптов в определенном порядке.

Он выполняет сценарии по папкам в указанном порядке, внутри папки файлы сортируются по имени и выполняются последовательно.

Поддерживаемые СУБД: MS-SQL Server


## Требования к файлам скриптов

1. SQL скрипты для создания объектов в БД должны быть идемпотентны, т.е. создавать объект, если его нет в БД или заменять если есть (например, использовать CREATE OR ALTER).
2. Все файлы должны быть в кодировке UTF-8 без BOM (byte order mark)
3. Для MS SQL скриптов:
    1. Нельзя использовать команду GO (это функция только для IDE и клиента sqlcmd)
    2. Нельзя использовать команду USE (выбор БД осуществляется автоматически)


## Структура файлов

Чтобы утилита могла правильно работать каталог скриптов должен иметь следюущую структуру:

    <имя бд 1>
        <procs>
            <имя схемы>
                <файл 1>.sql
                <файл 1>.sql
        <funcs|views|иное>
            <имя схемы>
                <файл 1>.sql
                <файл 1>.sql
    <имя бд 2>
        <procs>
            <имя схемы>
                <файл 1>.sql
                <файл 1>.sql
        <funcs|views|иное>
            <имя схемы>
                <файл 1>.sql
                <файл 1>.sql

1. Первый уровень - имя БД. Должны быть уникальными и совпадать с именами БД в файле настроек программы (см. поле databases/database).
1. Второй уровень - имена групп объектов (представления, процедуры и т.п.), должны быть одинаковы во всех папках баз данных. Конкретные имена и порядок выполнения скриптов из групп можно задать в файле настроек программы (см. поле changes_order, по умолчанию [procs funcs views], можно добавить свои).
1. Третий уровень - имя схемы БД (напрмер, dbo). Должжно быть уникально внутри второго уровня.
1. Четверный уровень - имя файла скрипта. Должно иметь расширение .sql. Имя файла влияет на порядок выполнения. Порядок выполнения - по возрастанию (лексикографический).

*Если в структуре присутсвуют каталоги вторго уровня не совпадающие с списком changes_order в файле настроек программы, то при исполнении они игнорируются, о чем программа напишет. Большая вложенность каталогов не поддерживается, все скрипты должны быть расположены в каталоге соответствующией им схемы.*

**Внимание!**
Имя каталога 2 и 3 уровня предназначены только для удобства группировки файлов, это значит, что разработчик все также может указать неверное имя схемы внутри файла скрипта. Это в его зоне ответственности.

## Механизм работы

Программа получает на вход список имено файлов в формате `db_name/group_folder/schema/script.sql`, по одному на строке. Разбирает их на части, определяет имя базы данных, подбирает из файла настроек подходящие парамтеры соединения, затем подключается и выполняет скрипты. Это происходит для каждой базы даных.

Порядок выполнения по базам данным при наличии изменений в нескольких базах данных не гарантирован. Гарантируется исполнение измененных скриптов в контексте одной базы данных.

Запуск скриптов выполняется последовательно по базам данных.

Каждый секрипт запускается внутри транзакции.

### Остановка при ошибках

При обнаружении ошибок конфигурации или выполнении скрипта исполнение будет остановлено на текущем файле.


## Сборка

Собирать в среде Linux, так как предназначен для запруска в GitLab CI (в linux-контейнере).

Скачать исходники, установить go 1.20 и запустить в папке исходиков команду:

    go build andy.go

Исполняемый файл `./andy` для Linux систем.

### Сборка контейнера

    sudo podman build -t andy-sql:0.1 -f ./Dockerfile

Проверить, что появился:

    sudo podman images | grep andy-sql

Проверка сборки:

    sudo podman run --rm -it andy-sql ./andy --version

    // Output: Version: , VCS Revision: , Branch: , BuildDate:

Выполнение команды:

    sudo podman run --rm -it andy-sql-app ./andy --version

## Базовое использование

Доступные параметры:
- `-dir` - путь к корневой папке в которой расположены скрипты баз данных согласно поддерживаемой структуре
- `-i` - принимать список путей измененных файлов из STDIN потока. Удобно при использовании вывода других программ
- `-f` - принимать список путей измененных файлов из указанного файла
- `--dry-run` - выполнить пробный запуск без реального соединения с БД, выводит список скриптов запланированных к выполнению, в том порядке как будет их исполнять
- `--config` - файл настроек, содержит параметры подключения к используемым БД, и парметры порядка обхода папок групп (напрмер: view, procs, func)
- `-help` - показать помощь, описание доступных параметров
- `-version` - информация о версии

**Режим пробного запуска (dry-run)** – показывает, какие скрипты будут выполняться, но не выполняет их на сервере

    ./andy --f=changes.txt --dry-run --config=./andy_config.yml

**Реальный режим**

    ./andy --f=changes.txt --config=./andy_config.yml

**Использование с git**

Можно получить список изменных файлов между 2 тегами так (исключая удаленные файлы):

    git diff --name-status --no-renames tag-1 tag-42 | grep -v '^D' | awk '{print $2}' | grep "databases/"

Перенаправить выввод этой команды в andy и он выполнит их, при наличии в конфигурации параметров подключения к базам данных.

Для тестовых прогонов использовать (не создает подключения к БД):

    git diff --name-status --no-renames tag-1 tag-42 | grep -v '^D' | awk '{print $2}' | grep "databases/" | ./andy -i --dry-run --config=./andy_config.yml

Для применения изменений к БД:

    git diff --name-status --no-renames tag-1 tag-42 | grep -v '^D' | awk '{print $2}' | grep "databases/" | ./andy -i --config=./andy_config.yml


## Ручное тестирование

Запустите `./andy -i --config=./andy_config.yml --dry-run` в введите пути к файлам по одному на строке:

    testdb/views/schema_a/1.sql
    testdb/views/schema_a/2.sql
    testdb/propcs/schema_a/p1.sql

Нажмите `Ctrl+D``. Программа выполнит действия с указанными файлами.

Проверьте вывод.

## Автоматизированное натурное тестирование

См. [Demo/Readme](demo/README.md)

# Известные проблемы и решения  

1. Если в файле настроек нет данных для имени базы данных - исполнение перывается.  
_Решение: актуализировать файлы настроек._
1. Если каталоги имеют неподходящие имена - они будут проигнорированны, а выводе задачи будет указано какие файлы были проигнорированны.  
_Решение: исправить имена на подходящие_
1. Несколько скриптов sql выполняются, но на определенных падает с ошибкой, хотя при запуске в MS SQL Studio все отрабатывает корректно. Проблема: драйвер программы не понимает многострочные определения параметров в процедурах и функциях, или вы сохранили файл в кодировке UTF-8 с BOM.  
_Решение: написать определение функции и параметров в одну строку без переносов, например, `CREATE proc1 (@param1 varchar, @param2 varchar)`, или поменять кодировку файла на UTF-8 без BOM_

# Сопровождение программы

Синявский Дмитрий (@SinyavskijjDS)
