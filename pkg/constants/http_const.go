package constants

// HttpInfoKey Прежний строковый ключ HttpInfo в контексте.
//
// Deprecated: ключом контекста не служит с v1.2.0; HttpInfo читать через gin.GetHttpInfoFromContext.
const HttpInfoKey string = "http_info"
