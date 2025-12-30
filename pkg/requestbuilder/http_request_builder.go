package requestbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/config"
	"github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-core/pkg/helpers"
	"github.com/exgamer/gosdk-core/pkg/logger"
	config2 "github.com/exgamer/gosdk-http-core/pkg/config"
	"github.com/exgamer/gosdk-http-core/pkg/gin"
	logger2 "github.com/exgamer/gosdk-http-core/pkg/logger"
	gin2 "github.com/gin-gonic/gin"
	"github.com/gookit/goutil/netutil/httpheader"
	"github.com/motemen/go-loghttp"
	"github.com/moul/http2curl"
	"io"
	"net/http"
	"net/url"
	"time"
)

//
// ----------------------------------------------------
// Constructors
// ----------------------------------------------------
//

func NewPostHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodPost)
}

func NewPutHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodPut)
}

func NewPatchHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodPatch)
}

func NewGetHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodGet)
}

func newBuilder[E any](ctx context.Context, rawURL, method string) *HttpRequestBuilder[E] {
	base := http.DefaultTransport.(*http.Transport).Clone()

	return &HttpRequestBuilder[E]{
		url:                      rawURL,
		method:                   method,
		timeout:                  30 * time.Second,
		transport:                loghttp.Transport{Transport: base},
		responseDataPresentation: constants.JSON,
		ctx:                      ctx,
		showLogs:                 true,
		throwUnmarshalError:      true,
		maxLogBodyBytes:          16 * 1024,
	}
}

//
// ----------------------------------------------------
// Builder
// ----------------------------------------------------
//

type HttpRequestBuilder[E any] struct {
	url                      string
	method                   string
	headers                  map[string]string
	queryParams              map[string]string
	body                     io.Reader
	rawBodyBytes             []byte
	timeout                  time.Duration
	transport                loghttp.Transport
	request                  *http.Request
	response                 *HttpResponse[E]
	responseDataPresentation string
	ctx                      context.Context
	execTime                 time.Duration

	appInfo  *config.AppInfo
	httpInfo *config2.HttpInfo

	showLogs            bool
	throwUnmarshalError bool
	maxLogBodyBytes     int
}

//
// ----------------------------------------------------
// Configuration
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) SetRequestHeaders(h map[string]string) *HttpRequestBuilder[E] {
	b.headers = h
	return b
}

func (b *HttpRequestBuilder[E]) SetQueryParams(q map[string]string) *HttpRequestBuilder[E] {
	b.queryParams = q
	return b
}

func (b *HttpRequestBuilder[E]) SetRequestTimeout(t time.Duration) *HttpRequestBuilder[E] {
	b.timeout = t
	return b
}

func (b *HttpRequestBuilder[E]) SetLogsEnabled(v bool) *HttpRequestBuilder[E] {
	b.showLogs = v
	return b
}

func (b *HttpRequestBuilder[E]) SetThrowUnmarshalError(v bool) *HttpRequestBuilder[E] {
	b.throwUnmarshalError = v
	return b
}

//
// ----------------------------------------------------
// Body helpers
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) SetJSONBody(v any) *HttpRequestBuilder[E] {
	data, err := json.Marshal(v)
	if err != nil {
		logger.LogError(err)

		return b
	}

	b.rawBodyBytes = data
	b.body = bytes.NewReader(data)
	b.ensureHeaders()
	b.headers[httpheader.ContentType] = gin2.MIMEJSON

	return b
}

func (b *HttpRequestBuilder[E]) SetXMLBody(v any) *HttpRequestBuilder[E] {
	data, err := xml.Marshal(v)
	if err != nil {
		logger.LogError(err)

		return b
	}

	b.rawBodyBytes = data
	b.body = bytes.NewReader(data)
	b.ensureHeaders()
	b.headers[httpheader.ContentType] = gin2.MIMEXML

	return b
}

//
// ----------------------------------------------------
// Execution
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) Do() error {
	start := time.Now()
	err := b.do()
	b.execTime = time.Since(start)

	if err != nil {
		b.logWithError(err)

		return err
	}

	if b.showLogs && b.appInfo != nil {
		msg, _ := b.buildLogMessage()
		logger2.FormattedLogWithAppInfo(b.appInfo, b.httpInfo, msg)
	}

	return nil
}

func (b *HttpRequestBuilder[E]) do() error {
	b.ensureHeaders()

	client := &http.Client{
		Timeout:   b.timeout,
		Transport: &b.transport,
	}

	finalURL, err := b.buildURL()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(b.ctx, b.method, finalURL, b.body)
	if err != nil {
		return err
	}

	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	b.response = &HttpResponse[E]{Url: finalURL, Method: b.method}
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	b.response.Status = resp.Status
	b.response.StatusCode = resp.StatusCode

	body, _ := io.ReadAll(resp.Body)
	b.response.Body = body

	return nil
}

//
// ----------------------------------------------------
// Result
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) GetResult() (*HttpResponse[E], error) {
	b.appInfo = gin.GetAppInfoFromContext(b.ctx)
	b.httpInfo = gin.GetHttpInfoFromContext(b.ctx)

	if err := b.Do(); err != nil {
		return nil, err
	}

	var r Response[E]

	if err := json.Unmarshal(b.response.Body, &r); err != nil && b.throwUnmarshalError {
		return nil, err
	}

	b.response.Result = r
	b.setDebugInfo()

	if b.response.StatusCode >= 500 {
		return nil, fmt.Errorf("http %s %s -> %d", b.method, b.response.Url, b.response.StatusCode)
	}

	return b.response, nil
}

//
// ----------------------------------------------------
// Logging & Debug
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) setDebugInfo() {
	d := debug.GetDebugFromContext(b.ctx)
	if d == nil {
		return
	}

	st := HttpStatement{
		Time:        helpers.GetDurationAsString(b.execTime),
		Status:      b.response.StatusCode,
		Method:      b.method,
		Url:         b.response.Url,
		Headers:     b.headers,
		QueryParams: b.queryParams,
		Body:        string(b.rawBodyBytes),
		Duration:    b.execTime,
	}

	if curl, err := http2curl.GetCurlCommand(b.request); err == nil {
		st.Curl = curl.String()
	}

	d.Cat("http")
	d.AddStatement("http", b.execTime, []HttpStatement{st})
	d.CalculateTotalTime()
}

func (b *HttpRequestBuilder[E]) buildLogMessage() (string, error) {
	msg := fmt.Sprintf("HTTP %s %s (%s)", b.method, b.url, b.execTime)

	return msg, nil
}

func (b *HttpRequestBuilder[E]) logWithError(err error) {
	if b.appInfo == nil {
		return
	}

	msg := fmt.Sprintf("HTTP ERROR %s %s: %v", b.method, b.url, err)

	logger2.FormattedLogWithAppInfo(b.appInfo, b.httpInfo, msg)
}

//
// ----------------------------------------------------
// Utils
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) ensureHeaders() {
	if b.headers == nil {
		b.headers = map[string]string{}
	}
}

func (b *HttpRequestBuilder[E]) buildURL() (string, error) {
	if len(b.queryParams) == 0 {
		return b.url, nil
	}

	u, err := url.Parse(b.url)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, v := range b.queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
