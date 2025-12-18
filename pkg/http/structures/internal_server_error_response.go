package structures

// InternalServerResponse Структура описывает ответ для 500
type InternalServerResponse struct {
	Success     bool     `json:"success"`
	ServiceCode int      `json:"service_code"`
	Message     string   `json:"message"`
	Details     []string `json:"details"`
}
