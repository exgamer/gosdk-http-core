package structures

// UnsupportedMediaTypeErrorResponse Структура описывает ответ для 415 (неподдерживаемый Content-Type)
type UnsupportedMediaTypeErrorResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status    int            `json:"status"`
		RequestId string         `json:"request_id"`
		Error     string         `json:"error"`
		Message   string         `json:"message"`
		Hostname  string         `json:"hostname"`
		Details   map[string]any `json:"details"`
	} `json:"data"`
}
