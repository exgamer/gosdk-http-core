### Пример использования функционала для получения стандартных заголовков сервисов

## подключае мидлвейр
```go

	router.Use(httpMiddleware.RequestMiddleware(baseConfig))

	v1 := router.Group("/article/v1")
	{
	    v1.GET("/articles", handlersFactory.ArticleHandler.Index())
	}
```

## Вызываем функцию gin.GetAppInfo(c) для получения структуры с значениями заголовков
```go
		appInfo := gin.GetAppInfo(c)
        pagerRequest, err := paginator.GetPagerRequest(c)
        request := requests.ArticleIndexRequest{}
        c.BindQuery(&request)
        searchDto := h.searchDtoFromIndexRequest(request)
        searchDto.Page = pagerRequest.Page
        searchDto.PerPage = pagerRequest.PerPage
        paginated, err := h.articleService.PaginatedCashed(c.Request.Context(), searchDto)
        
        if err != nil {
            httpHelpers.ErrorResponse(c, http.StatusInternalServerError, err, nil)
            
            return
        }

        httpHelpers.SuccessResponse(c, h.paginatedResponse(paginated, appInfo.LanguageCode))


```


### Пример использования функционала для валидации HTTP реквеста. GIn использует эту библиотеку для валидации, все валидаторы в документации (https://github.com/go-playground/validator)

## Определить структуру request унаследовать от validation.Request и описать свойства а так же валидацию
## Для поля Custom приведенпример кастомной валидации. 
## Для этого в реквесте определена функция валидации customIntBig2Validator. 
## Далее нужно определить функцию CustomValidationRules и указать там все доп правила валидаии.
## Если нужно указать сообещия для ошибок к кастомным правилам нужно определить функцию CustomValidationMessage по примеру
```go
type ArticleCreateRequest struct {
    validation.Request
	Custom               int               `json:"custom" binding:"required,customIntBig2"`
    LocationId           int               `json:"location_id" binding:"required,email"`
    LocatioId            int               `json:"locatio_id" binding:"required"`
}

func (r *ArticleCreateRequest) CustomValidationMessage(fe validator.FieldError) string {
    switch fe.Tag() {
    case "customIntBig2":
    
    return "Must be greater than 2"
    }

    return fe.Tag()
}

func (r *ArticleCreateRequest) CustomValidationRules() map[string]validator.Func {
    return map[string]validator.Func{
        "customIntBig2": r.customIntBig2Validator(),
    }
}

func (r *ArticleCreateRequest) customIntBig2Validator() func(fl validator.FieldLevel) bool {
    return func(fl validator.FieldLevel) bool {
        i, ok := fl.Field().Interface().(int)
        
        if ok {
            if i < 2 {
                return false
            }
        }
    
        return true
    }
}
```

### Для вызова валидации тела запроса нужно в контроллере сделать следующее
```go 
    import ginHelper "github.com/exgamer/gosdk-http-core/pkg/gin"

    c := *gin.Context
    request := requests.ArticleCreateRequest{}
    valid := ginHelper.ValidateRequestBody(c, &request)
    
    if valid == false {
        return
    }
	
```

### Для вызова валидации query параметров запроса нужно в контроллере сделать следующее
```go 
    import ginHelper "github.com/exgamer/gosdk-http-core/pkg/gin"

    c := *gin.Context
    request := requests.ArticleIndexRequest{}
    valid := ginHelper.ValidateRequestQuery(c, request)
    
    if valid == false {
        return
    }
	
```