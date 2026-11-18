package constants

const (
	// CorrelationIdHeaderName Заголовок сквозного id корреляции: принимается на входе (первым) и отдаётся в ответе.
	CorrelationIdHeaderName string = "X-Correlation-Id"
	// XRequestIdHeaderName Id запроса от ingress-nginx: на входе — второй после X-Correlation-Id.
	XRequestIdHeaderName string = "X-Request-Id"
	// RequestIdHeaderName Прежний заголовок id запроса: на входе — третий после X-Correlation-Id и X-Request-Id.
	//
	// Deprecated: используйте CorrelationIdHeaderName.
	RequestIdHeaderName        string = "Request-Id"
	LanguageHeaderName         string = "Accept-Language"
	CityHeaderName             string = "City-Id"
	UserHeaderName             string = "User-Id"
	AppsflyerHeaderName        string = "Appsflyer-Id"
	CurrentCompanyIdHeaderName string = "Current-Company-Id"
	CompanyIdHeaderName        string = "Company-Id"
	CompanyIdsHeaderName       string = "Company-Ids"
	IinHeaderName              string = "Iin"
	CacheControlHeaderName     string = "Cache-Control"
	OriginHeaderName           string = "Origin"
	AuthorizationHeaderName    string = "Authorization"
)
