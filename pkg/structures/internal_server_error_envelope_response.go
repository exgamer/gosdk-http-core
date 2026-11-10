package structures

// InternalServerErrorResponse Структура описывает ответ для 500 (ошибка обработчика в общей обёртке; InternalServerResponse — тело ответа на панику)
type InternalServerErrorResponse struct {
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
