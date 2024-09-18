# Демонстрация работы утилиты andy

Для запуска демо:
1. Установить Windows WSL (Ubuntu Linux), podman, mssql server 2019 linux container
    sudo apt install podman

1. Подготовить MSSQL сервер и базы данных
    1. Через скрипты

        1. запустить `sudo podman-compose up`
        1. инициализировать БД `./init_db.sh`

    1. или вручную

        1. Создать каталоги для файлов БД MSSQL Server

            sudo mkdir -p  /var/mssql/data
            sudo chmod 755 -R /var/mssql/data

        1. Установить образ MSSQL Server Linux

            podman pull mcr.microsoft.com/mssql/server:2022-latest

        1. Запустить MSSQL Server в контейнер в WSL (для подключения будет использован порт 1460)

            sudo podman run -d -e 'ACCEPT_EULA=Y' -e 'MSSQL_SA_PASSWORD=Passw0rd' --name MSSQL -p 1460:1433 -v /var/mssql/data:/var/mssql/data:Z mcr.microsoft.com/mssql/server:2022-latest

        1. Подлкючиться в контейнер MSSQL

            sudo podman exec -it MSSQL "bash"
            /opt/mssql-tools/bin/sqlcmd -S localhost -U SA -P "Passw0rd"

            или выполнить `sudo podman exec -it andy_db_1 /opt/mssql-tools/bin/sqlcmd -S localhost -U SA -P "Passw0rd" -Q "CREATE DATABASE testdb;"`

            Если база уже соаздана, то увидите сообщение `Database 'testdb' already exists.`

        1. Cоздать тестовую БД с таблицей, выполнить:

            -- создаем БД
            CREATE DATABASE testdb;
            go;
            -- проверяем ее наличие
            SELECT Name from sys.Databases;
            go;

            -- выход из sqlcmd
            quit

        1. Выйти из конейнера MSSQL

            exit

1. Выполнить в корневом каталоге исходников: `make prepare-demo`
1. Перети в каталог demo
1. Показать штатную работу инструмента:
    1. Запустить проверку

        ./test_good_scripts.sh

    1. Показать, что объекты появилсь в БД (использовать Visual Studio Code MSSQL Extension или другую IDE для DB)
    1. Изменить текст одного из файлов.
    1. Запустить проверку из шага 1
    1. Показать, что код в файле соответствует коду в БД

1. Показать работу инструмента при наличии синтаксической ошибки в одном из файлов скриптов:
    1. Запустить проверку

        ./test_bad_scripts.sh

    1. Показать, что инструмент остановил выполнение на скрипте с ошибкой
    1. Исправить ошибку и запустить проверку снова. Показать, что скрипты выполнились.
    1. Показать, что код исправления применен к объекту в БД (использовать Visual Studio Code MSSQL Extension или другую IDE для DB)

1. Показать работу иснтрумента в режиме прогона (dry-run), когда показывается какие скрипты будут исполнены, но самого исполнения не происходит:
    1. Запустить проверку

        ./test_dryrun_good_scripts.sh
