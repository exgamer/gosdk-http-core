// Package publictext Признак HttpException «код ответа задал сервис явно» — между пакетами exception и response,
// без публичного API: поле исключения неэкспортируемое, читается через функцию, которую ставит пакет exception.
package publictext

// Explicit Код ответа исключения задан сервисом явно (публичный конструктор с кодом или функция response со статусом).
// Ставит пакет exception при инициализации; до неё — false.
var Explicit = func(any) bool { return false }
