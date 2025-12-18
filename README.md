SDK Tool for fast REST Go Application create

Example Application https://github.com/exgamer/gosdk-http-core-rest-template


Gin rate limiter
https://github.com/JGLTechnologies/gin-rate-limit

<hr style="border: 1px solid orange;"/>

### При повышении версии gosdk, начиная с версии где добавлен функционал GRPC обязательно нужно выполнить команды указанные ниже:

go get google.golang.org/genproto@latest

go mod tidy



### Как подключать локально?
1) В терминале на локальной машине прописать
<code>go env -w GOPRIVATE='gitlab.almanit.kz/jmart/*'</code>


2) В гитлабе https://gitlab.almanit.kz/-/user_settings/personal_access_tokens сгенерировать себе токены доступа:
   1)  <b>Expiration Date</b>  можно удалить, и тогда токен будет с сроком жизни на год
   
   2) <b>Select scopes</b>  дать права на чтение как минимум, а остальное на свое усмотрение
   3)  Нажать на кнопку  <code>Create personal access token</code>. Не забудьте сохранить себе этот токен куда нибудь, в случае необходимости
   

3)  В терминале на локальной машине прописать <code>git config --global --add url."https://gitlab-ci-token:ACCESS_TOKEN@gitlab.almanit.kz/".insteadOf "https://gitlab.almanit.kz/"
</code> Необходимо вместо ACCESS_TOKEN прописать тот токен который сгенерировался ранее на шаге 2

4) Пробовать команду  <code>go get -u github.com/exgamer/gosdk-http-core</code>


<hr style="border: 1px solid orange;"/>

### Как работает в  Docker?

1)  Можно взять Docker файл отсюда - https://gitlab.almanit.kz/jmart/banner-service
2) Или самим прописать:
   1) <code>ENV GOPRIVATE=gitlab.almanit.kz/jmart/*</code>
   2) <code>RUN git config --global url."https://gitlab-ci-token:${READ_JMART_PROJECT_TOKEN}@gitlab.almanit.kz/".insteadOf "https://gitlab.almanit.kz/"
      </code>   Сделать запрос команде Devops о том, чтобы они добавили переменную READ_JMART_PROJECT_TOKEN в команду создания образа