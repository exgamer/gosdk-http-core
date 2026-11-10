package structures

// RawErrorResponse Структура описывает тело ошибки без обёртки success/data — ответы response.*Raw
// (ErrorResponseRaw, ErrorResponseWithStatusRaw, BadRequestRaw и подобные), для Swagger
type RawErrorResponse struct {
	Status    int            `json:"status"`
	RequestId string         `json:"request_id"`
	Error     string         `json:"error"`
	Message   string         `json:"message"`
	Hostname  string         `json:"hostname"`
	Details   map[string]any `json:"details"`
}
