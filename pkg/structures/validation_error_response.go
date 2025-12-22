package structures

// ValidationErrorResponse Структура описывает ответ для 422
type ValidationErrorResponse struct {
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
