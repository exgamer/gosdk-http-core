package builderv2

type Response[E interface{}] struct {
	Success bool `json:"success"`
	Data    E    `json:"data"`
}
