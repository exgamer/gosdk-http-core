package config

import "github.com/google/uuid"

// HttpInfo Данные http
type HttpInfo struct {
	// CorrelationId Сквозной id корреляции запроса (X-Correlation-Id → X-Request-Id → Request-Id → новый UUID);
	// тот же id — в sdkctx.CorrelationID(ctx).
	CorrelationId string
	// RequestId Тот же id, что CorrelationId.
	//
	// Deprecated: используйте CorrelationId или sdkctx.CorrelationID(ctx).
	RequestId     string
	AuthToken     string
	RequestScheme string
	RequestHost   string
	RequestMethod string
	RequestUrl    string
	CacheControl  string
	RequestOrigin string // CORS
	LanguageCode  string
}

// GenerateRequestId Новый id корреляции в CorrelationId и RequestId.
//
// Deprecated: id корреляции выставляет RequestInfoMiddleware (GetInstanceHttpInfo).
func (s *HttpInfo) GenerateRequestId() {
	s.CorrelationId = uuid.New().String()
	s.RequestId = s.CorrelationId
}
