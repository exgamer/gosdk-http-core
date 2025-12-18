## Инициализация приложения V2

<hr style="border: 1px solid orange;"/>

[Пример для HTTP приложения](HTTP_README.md)
[Пример для CONSUMER приложения](CONSUMER_README.md)


### PS

1) По примеру HTTP и CONSUMER определяются консольное приложение и Grpc (PrepareGrpcFunc, PrepareConsoleFunc) и запускаются RunConsole и RunGrpc

2) Для консольных приложений лучше создавать отдельный main.go

3) Для работы документации сваггера нужно указать переменную окружения SWAGGER_PREFIX=catalog

4) Для установки timezone в .ENV нужно указать переменную окружения TIMEZONE=Asia/Almaty. Далее при инициализации приложения в BaseConfig будет свойство Location, который можно использовать дальше