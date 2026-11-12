package domain

import "errors"

type Error struct {
	Status  int    `json:"-"`
	Type    string `json:"type"`
	Code    string `json:"code"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

var ErrNotFound = errors.New("not found")

func Invalid(code, param, msg string) *Error {
	return &Error{Status: 400, Type: "invalid_request_error", Code: code, Param: param, Message: msg}
}

func NotFound(what string) *Error {
	return &Error{Status: 404, Type: "invalid_request_error", Code: "resource_missing", Message: what + " não existe neste modo."}
}

func Forbidden(code, msg string) *Error {
	return &Error{Status: 403, Type: "permission_error", Code: code, Message: msg}
}

func Conflict(code, msg string) *Error {
	return &Error{Status: 409, Type: "invalid_request_error", Code: code, Message: msg}
}

func Unauthorized(code, msg string) *Error {
	return &Error{Status: 401, Type: "authentication_error", Code: code, Message: msg}
}

var (
	ErrBadCredentials = Unauthorized("invalid_credentials", "Email ou palavra-passe incorretos.")
	ErrLoginLocked    = &Error{Status: 429, Type: "rate_limit_error", Code: "login_locked", Message: "Demasiadas tentativas erradas. Tente de novo daqui a 15 minutos, ou recupere a palavra-passe."}
	ErrLoginCode      = Unauthorized("login_code_required", "Enviámos um código para o seu email. Escreva-o para entrar neste dispositivo.")
	ErrLoginCodeWrong = Unauthorized("login_code_invalid", "O código não está certo ou expirou. Peça outro.")
	ErrBadKey         = Unauthorized("api_key_invalid", "Chave API inválida ou revogada.")
)
