package structures

// NotFoundErrorResponse Структура описывает ответ 404 400
type NotFoundErrorResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ServiceCode int               `json:"service_code"`
		Status      int               `json:"status"`
		RequestId   string            `json:"request_id"`
		Message     string            `json:"message"`
		Hostname    string            `json:"hostname"`
		Error       string            `json:"error"`
		Details     map[string]string `json:"details"`
	} `json:"data"`
}
