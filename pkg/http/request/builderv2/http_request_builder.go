package builderv2

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/constants"
	"github.com/exgamer/gosdk-http-core/pkg/debug"
	"github.com/exgamer/gosdk-http-core/pkg/helpers"
	"github.com/exgamer/gosdk-http-core/pkg/logger"
	"github.com/exgamer/gosdk-http-core/pkg/tracer"
	"github.com/gin-gonic/gin"
	"github.com/gookit/goutil/netutil/httpheader"
	"github.com/motemen/go-loghttp"
	"github.com/moul/http2curl"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewPostHttpRequestBuilder - Новый построитель rest запросов для POST
func NewPostHttpRequestBuilder[E interface{}](url string) *HttpRequestBuilder[E] {
	return &HttpRequestBuilder[E]{
		url:                      url,
		method:                   "POST",
		timeout:                  30 * time.Second,
		transport:                loghttp.Transport{},
		responseDataPresentation: constants.JSON,
		showLogs:                 true,
	}
}

// NewGetHttpRequestBuilder - Новый построитель rest запросов для GET
func NewGetHttpRequestBuilder[E interface{}](url string) *HttpRequestBuilder[E] {
	return &HttpRequestBuilder[E]{
		url:                      url,
		method:                   "GET",
		timeout:                  30 * time.Second,
		throwUnmarshalError:      false,
		transport:                loghttp.Transport{},
		responseDataPresentation: constants.JSON,
		showLogs:                 true,
	}
}

// NewPostHttpRequestBuilderWithCtx - Новый построитель rest запросов для POST. Требуется прокинуть контекст для трассировки
func NewPostHttpRequestBuilderWithCtx[E interface{}](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return &HttpRequestBuilder[E]{
		url:                      url,
		method:                   "POST",
		timeout:                  30 * time.Second,
		transport:                loghttp.Transport{},
		responseDataPresentation: constants.JSON,
		ctx:                      ctx,
		showLogs:                 true,
	}
}

// NewPutHttpRequestBuilderWithCtx - Новый построитель rest запросов для PUT. Требуется прокинуть контекст для трассировки
func NewPutHttpRequestBuilderWithCtx[E interface{}](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return &HttpRequestBuilder[E]{
		url:                      url,
		method:                   "PUT",
		timeout:                  30 * time.Second,
		transport:                loghttp.Transport{},
		responseDataPresentation: constants.JSON,
		ctx:                      ctx,
		showLogs:                 true,
	}
}

// NewPatchHttpRequestBuilderWithCtx - Новый построитель rest запросов для PATCH. Требуется прокинуть контекст для трассировки
func NewPatchHttpRequestBuilderWithCtx[E interface{}](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return &HttpRequestBuilder[E]{
		url:                      url,
		method:                   "PATCH",
		timeout:                  30 * time.Second,
		transport:                loghttp.Transport{},
		responseDataPresentation: constants.JSON,
		ctx:                      ctx,
		showLogs:                 true,
	}
}

// NewGetHttpRequestBuilderWithCtx - Новый построитель rest запросов для GET.  Требуется прокинуть контекст для трассировки
func NewGetHttpRequestBuilderWithCtx[E interface{}](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return &HttpRequestBuilder[E]{
		url:                      url,
		method:                   "GET",
		timeout:                  30 * time.Second,
		throwUnmarshalError:      false,
		transport:                loghttp.Transport{},
		responseDataPresentation: constants.JSON,
		ctx:                      ctx,
		showLogs:                 true,
	}
}

// HttpRequestBuilder - Построитель rest запросов
type HttpRequestBuilder[E interface{}] struct {
	url                      string
	method                   string
	headers                  map[string]string
	throwUnmarshalError      bool
	setStandardHeaders       bool
	showLogs                 bool
	queryParams              map[string]string
	body                     io.Reader
	rawBodyBytes             []byte
	appInfo                  *config.AppInfo
	timeout                  time.Duration
	transport                loghttp.Transport
	request                  *http.Request
	response                 *HttpResponse[E]
	responseDataPresentation string
	result                   Response[E]
	ctx                      context.Context
	token                    string
	execTime                 time.Duration
}

func (builder *HttpRequestBuilder[E]) SetStandardHeaders(set bool) *HttpRequestBuilder[E] {
	builder.setStandardHeaders = set

	return builder
}

func (builder *HttpRequestBuilder[E]) SetResponseDataPresentation(dataPresentation string) *HttpRequestBuilder[E] {
	builder.responseDataPresentation = dataPresentation

	return builder
}

func (builder *HttpRequestBuilder[E]) SetHttpRequest(request *http.Request) *HttpRequestBuilder[E] {
	builder.request = request

	return builder
}

// SetRequestData - установить Доп данные для запроса (используется для логирования)
func (builder *HttpRequestBuilder[E]) SetRequestData(appInfo *config.AppInfo) *HttpRequestBuilder[E] {
	builder.appInfo = appInfo

	return builder
}

// SetRequestHeaders - установить заголовки запроса
func (builder *HttpRequestBuilder[E]) SetRequestHeaders(headers map[string]string) *HttpRequestBuilder[E] {
	builder.headers = headers

	return builder
}

// SetQueryParams - установить гет параметры запроса
func (builder *HttpRequestBuilder[E]) SetQueryParams(queryParams map[string]string) *HttpRequestBuilder[E] {
	builder.queryParams = queryParams

	return builder
}

// SetRequestBody - установить тело запроса
func (builder *HttpRequestBuilder[E]) SetRequestBody(body io.Reader) *HttpRequestBuilder[E] {
	if body == nil {
		return builder
	}

	data, err := io.ReadAll(body)

	if err != nil {
		logger.LogError(err) // базовое логирование

		return builder
	}

	builder.rawBodyBytes = data
	builder.body = bytes.NewReader(data) // безопаснее, чем через string

	return builder
}

// SetRequestTimeout - установить таймаут запроса
func (builder *HttpRequestBuilder[E]) SetRequestTimeout(timeout time.Duration) *HttpRequestBuilder[E] {
	builder.timeout = timeout

	return builder
}

// SetRequestTransport - установить параметры запроса
func (builder *HttpRequestBuilder[E]) SetRequestTransport(transport loghttp.Transport) *HttpRequestBuilder[E] {
	builder.transport = transport

	return builder
}

// WithoutLogs - без логов
func (builder *HttpRequestBuilder[E]) WithoutLogs(showLogs bool) *HttpRequestBuilder[E] {
	builder.showLogs = showLogs

	return builder
}

func (builder *HttpRequestBuilder[E]) SetToken(token string) *HttpRequestBuilder[E] {
	builder.token = token

	return builder
}

func (builder *HttpRequestBuilder[E]) do() error {
	if builder.ctx != nil {
		if tracer.TraceClient != nil && tracer.TraceClient.IsEnabled && tracer.TraceClient.IsTraceFlagEnabled() {
			_, span := tracer.TraceClient.CreateSpan(builder.ctx, "["+builder.method+"] "+builder.url)
			defer span.End()
		}
	}

	client := http.Client{
		Timeout:   builder.timeout,
		Transport: &builder.transport,
	}

	// Создаём параметры запроса с экранированием
	queryParams := url.Values{}

	if builder.queryParams != nil {
		for n, v := range builder.queryParams {
			queryParams.Add(n, v)
		}

		builder.url = builder.url + "?" + queryParams.Encode()
	}

	if builder.request == nil {
		builder.request, _ = http.NewRequest(builder.method, builder.url, builder.body)
	}

	if builder.appInfo != nil && builder.setStandardHeaders {
		if builder.headers == nil {
			builder.headers = make(map[string]string, 6)
		}

		if builder.appInfo.CityId != 0 {
			builder.headers[constants.CityHeaderName] = strconv.Itoa(builder.appInfo.CityId)
		}

		if builder.appInfo.UserId != 0 {
			builder.headers[constants.UserHeaderName] = strconv.Itoa(builder.appInfo.UserId)
		}

		if builder.appInfo.AppsflyerId != "" {
			builder.headers[constants.AppsflyerHeaderName] = builder.appInfo.AppsflyerId
		}

		if builder.appInfo.LanguageCode != "" {
			builder.headers[constants.LanguageHeaderName] = builder.appInfo.LanguageCode
		}

		if builder.appInfo.RequestId != "" {
			builder.headers[constants.RequestIdHeaderName] = builder.appInfo.RequestId
		}

		if builder.appInfo.AuthToken != "" {
			token := builder.appInfo.AuthToken

			if !strings.Contains(builder.appInfo.AuthToken, "Bearer") {
				token = "Bearer " + builder.appInfo.AuthToken
			}

			builder.headers[constants.AuthorizationHeaderName] = token
		}

		if builder.token != "" {
			builder.headers[constants.AuthorizationHeaderName] = builder.token
		}
	}

	_, exists := builder.headers[httpheader.ContentType]

	if !exists { // если кнтент тайп не указан делаем json
		builder.request.Header.Set(httpheader.ContentType, gin.MIMEJSON)
	}

	for n, v := range builder.headers {
		builder.request.Header.Set(n, v)
	}

	if builder.appInfo != nil {
		builder.request.Header.Set("X-Client-NAME", builder.appInfo.ServiceName)
	}

	response, err := client.Do(builder.request)
	builder.response = &HttpResponse[E]{
		Url:    builder.url,
		Method: builder.method,
	}

	if err != nil {
		return err
	}

	defer response.Body.Close()
	builder.response.Status = response.Status
	builder.response.StatusCode = response.StatusCode

	if builder.response.Headers == nil {
		builder.response.Headers = make(map[string]string)
	}

	for n, v := range response.Header {
		if len(v) > 0 {
			builder.response.Headers[n] = v[0]
		}
	}

	rBody, bErr := io.ReadAll(response.Body)

	if bErr != nil {
		logger.LogError(bErr)

		return bErr
	}

	builder.response.Body = rBody

	return nil
}

// Do - выполнить запрос
func (builder *HttpRequestBuilder[E]) Do() error {
	start := time.Now()
	err := builder.do()
	execTime := time.Since(start)
	builder.execTime = execTime

	if err != nil {
		messageBuilder, lErr := builder.GetRequestMessageBuilderWithError(err, execTime)

		if lErr != nil {

			return lErr
		}

		if builder.appInfo != nil {
			logger.FormattedLogWithAppInfo(builder.appInfo, messageBuilder.String())
		}

		return err
	}

	if !builder.showLogs {
		return nil
	}

	if builder.appInfo != nil {
		messageBuilder, lErr := builder.GetRequestMessageBuilder(execTime)

		if lErr != nil {

			return lErr
		}

		logger.FormattedLogWithAppInfo(builder.appInfo, messageBuilder.String())

		return nil
	}

	return nil
}

// GetResult  Возвращает результат
func (builder *HttpRequestBuilder[E]) GetResult() (*HttpResponse[E], error) {
	appInfo := helpers.GetAppInfoFromContext(builder.ctx)

	if appInfo != nil {
		builder.appInfo = appInfo
	}

	err := builder.Do()

	if err != nil {
		return nil, err
	}

	r := Response[E]{}

	switch builder.responseDataPresentation {
	case constants.JSON:
		unMarshErr := json.Unmarshal(builder.response.Body, &r)

		if unMarshErr != nil && builder.throwUnmarshalError {
			return nil, unMarshErr
		}
	case constants.XML:
		unMarshErr := xml.Unmarshal(builder.response.Body, &r)

		if unMarshErr != nil && builder.throwUnmarshalError {
			return nil, unMarshErr
		}
	default:
		unMarshErr := json.Unmarshal(builder.response.Body, &r)

		if unMarshErr != nil && builder.throwUnmarshalError {
			return nil, unMarshErr
		}
	}

	builder.response.Result = r

	builder.setDebugInfo()

	if builder.response.StatusCode >= 500 {
		return nil, errors.New(string(builder.response.Body))
	}

	return builder.response, nil
}

// GetErrorByKey Возвращает ключ из ошибки
func (builder *HttpRequestBuilder[E]) GetErrorByKey(key string) string {
	message, found := builder.response.ErrorsMap[key]

	if found {
		return message.(string)
	}

	return "unknown"
}

func (builder *HttpRequestBuilder[E]) GetRequestMessageBuilder(execTime time.Duration) (*strings.Builder, error) {
	messageBuilder := strings.Builder{}
	status := builder.response.Status

	if status == "" {
		status = "? UNKNOWN"
	}

	messageBuilder.WriteString("Url: " + builder.response.Method + " " + status + " " + builder.response.Url)
	flatHeaders := make(map[string]string)
	//проходим циклом по массиву заголовков и считываем в map flatHeaders
	for key, values := range builder.request.Header {
		if len(values) > 0 {
			flatHeaders[key] = values[0]
		}
	}

	jsonHeadersData, fhErr := json.Marshal(flatHeaders)

	if fhErr != nil {
		return &messageBuilder, fhErr
	}

	messageBuilder.WriteString(", Headers: " + string(jsonHeadersData))
	//логируем тело запроса
	if builder.rawBodyBytes != nil {
		messageBuilder.WriteString(", Request Body: " + string(builder.rawBodyBytes))
	}

	messageBuilder.WriteString(", Exec time:" + execTime.String())

	if builder.response.StatusCode >= 400 {
		messageBuilder.WriteString(", Response:" + string(builder.response.Body))

		if json.Valid(builder.response.Body) {
			unmarshallErr := json.Unmarshal(builder.response.Body, &builder.response.ErrorsMap)

			if unmarshallErr != nil {
				return nil, unmarshallErr
			}
		}
	}

	return &messageBuilder, nil
}

func (builder *HttpRequestBuilder[E]) GetRequestMessageBuilderWithError(error error, execTime time.Duration) (*strings.Builder, error) {
	messageBuilder, err := builder.GetRequestMessageBuilder(execTime)

	if err != nil {
		return nil, err
	}

	messageBuilder.WriteString(", Error:" + error.Error())

	return messageBuilder, nil
}

func (builder *HttpRequestBuilder[E]) setDebugInfo() {
	if builder.ctx == nil {
		return
	}

	debugCollector := debug.GetDebugFromContext(builder.ctx)

	if debugCollector == nil {

		return
	}

	statement := debug.HttpStatement{}
	statement.Headers = builder.headers
	statement.QueryParams = builder.queryParams
	statement.Body = string(builder.rawBodyBytes)
	statement.Time = helpers.GetDurationAsString(builder.execTime)
	statement.Duration = builder.execTime
	statement.Timeout = helpers.GetDurationAsString(builder.timeout)
	statement.Method = builder.method
	statement.Status = builder.response.StatusCode
	statement.Url = builder.response.Url

	command, err := http2curl.GetCurlCommand(builder.request)

	if err != nil {
		log.Println(err)
	} else {
		statement.Curl = command.String()
	}

	if builder.response.StatusCode >= 400 {
		if json.Valid(builder.response.Body) {
			json.Unmarshal(builder.response.Body, &builder.response.ErrorsMap)

			statement.Response = builder.response.ErrorsMap
		}
	}

	debugCollector.HttpQueries.Statements = append(debugCollector.HttpQueries.Statements, statement)
	debugCollector.CalculateHttpTotalTime()
}
