package structures

// PayloadTooLargeErrorResponse Структура описывает ответ для 413 (тело запроса больше допустимого)
type PayloadTooLargeErrorResponse struct {
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
