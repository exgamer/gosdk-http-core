Пример http запроса

## Get запрос
```go
package app

func (repo *UserRepository) GetUserByTokenAndRealm(token, userRealm string) (*structures2.User, *exception.AppException) {
	builder := httpRequestBuilder.NewGetHttpRequestBuilder[structures2.User](repo.restConfig.UserServiceHost + "/user/profile/by-token")
	builder.SetRequestHeaders(map[string]string{
		constants.HeaderUserRouteAuthorization: token,
		constants.HeaderUserRouteRealm:         userRealm,
	})
	builder.SetRequestData(repo.appInfo)
	// если нужно автоматически пробросить станартные заголовки такие как Request-Id, Accepted-Language, City-Id, "User-Id, Appsflyer-Id
	// нужно просто выставить SetStandardHeaders(true), но при этом нужно обязательно передать SetRequestData чтобы получить заголовки
	httpBuilder.SetStandardHeaders(true) 
	
	result, err := builder.GetResult()

	if err != nil {
		return nil, exception.NewAppException(http.StatusInternalServerError, err, nil)
	}

	if result.StatusCode != 200 {
		return nil, exception.NewAppException(result.StatusCode, errors.New(builder.GetErrorByKey("message")), nil)
	}

	return &result.Result, nil
}

```



## Post запрос
```go
package app

func (c *CatalogRepository) GetCategoryPost(ctx context.Context, appInfo *config.AppInfo) (*[]catalog.Category, *exception.AppException) {
	httpBuilder := rest.NewPostHttpRequestBuilderWithCtx[response.Response[catalog.Category]](ctx, c.restConfig.CatalogService+"/catalog-go/v1/categories/by-ids?page=1")

	httpBuilder.SetRequestHeaders(map[string]string{
		httpheader.ContentType: gin.MIMEJSON,
	})

	httpBuilder.SetRequestData(appInfo)
	a := make([]int, 0)
	a = append(a, 2207)
	a = append(a, 2879)
	requestBody, err := json.Marshal(map[string]interface{}{
		"ids": a,
	})

	httpBuilder.SetRequestBody(bytes.NewBuffer(requestBody))

	httpBuilder.SetStandardHeaders(true)
	result, err := httpBuilder.GetResult()

	if err != nil {
		return nil, exception.NewAppException(http.StatusInternalServerError, err, nil)
	}

	return &result.Result.Data, nil
}

```