package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Error 业务错误，对应 Java 的各种异常类型
type Error struct {
	Type    string // 错误类型名（对应 Java 类名）
	Message string
	Status  int
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap 支持 errors.Is/As
func (e *Error) Unwrap() error { return nil }

// Java 全限定类名，对应 Response.failure 中 throwable.getClass().getName() 的取值。
const (
	ValidationExceptionType = "com.alibaba.higress.sdk.exception.ValidationException"
	AuthExceptionType       = "com.alibaba.higress.console.controller.exception.AuthException"
	NotFoundExceptionType   = "com.alibaba.higress.sdk.exception.NotFoundException"
	ConflictExceptionType   = "com.alibaba.higress.sdk.exception.ResourceConflictException"
	BusinessExceptionType   = "com.alibaba.higress.sdk.exception.BusinessException"
	InternalExceptionType   = "java.lang.IllegalStateException"
)

// 构造器
func newError(typ string, status int, msg string) *Error {
	return &Error{Type: typ, Message: msg, Status: status}
}

// Validation 对应 ValidationException（400）
func Validation(msg string) *Error {
	return newError(ValidationExceptionType, http.StatusBadRequest, msg)
}

// Auth 对应 AuthException（401）
func Auth(msg string) *Error {
	return newError(AuthExceptionType, http.StatusUnauthorized, msg)
}

// NotFound 对应 NotFoundException（502）
func NotFound(msg string) *Error {
	return newError(NotFoundExceptionType, http.StatusBadGateway, msg)
}

// Conflict 对应 ResourceConflictException（409）
func Conflict(msg string) *Error {
	return newError(ConflictExceptionType, http.StatusConflict, msg)
}

// Business 对应 BusinessException（500）
func Business(msg string) *Error {
	return newError(BusinessExceptionType, http.StatusInternalServerError, msg)
}

// Internal 内部错误（500）
func Internal(msg string) *Error {
	return newError(InternalExceptionType, http.StatusInternalServerError, msg)
}

// AsError 将任意 error 转换为 *Error；若不是 *Error 则包装为 Business
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Business(err.Error())
}

// IsNotFound 判断是否为 NotFound 错误
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusBadGateway
}

// IsConflict 判断是否为 Conflict 错误
func IsConflict(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusConflict
}
