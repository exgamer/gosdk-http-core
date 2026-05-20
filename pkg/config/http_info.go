package config

import "github.com/google/uuid"

// HttpInfo Данные http
type HttpInfo struct {
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

func (s *HttpInfo) GenerateRequestId() {
	s.RequestId = uuid.New().String()
}
