package structures

// ForbiddenErrorResponse Структура описывает ответ для 403
type ForbiddenErrorResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status    int    `json:"status"`
		RequestId string `json:"request_id"`
		Error     string `json:"error"`
		Message   string `json:"message"`
		Hostname  string `json:"hostname"`
	} `json:"data"`
}
