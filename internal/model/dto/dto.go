package dto

import "console/internal/model"

// Response 对应 Java 的 Response<T>
type Response[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data"`
}

// NewSuccess 创建成功响应
func NewSuccess[T any](data T) *Response[T] {
	return &Response[T]{Success: true, Data: data}
}

// NewFailure 创建失败响应
func NewFailure[T any](message string) *Response[T] {
	return &Response[T]{Success: false, Message: message}
}

// PaginatedResponse 对应 Java 的 PaginatedResponse<T>
type PaginatedResponse[T any] struct {
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	Data     []T    `json:"data"`
	Total    int    `json:"total"`
	PageNum  *int   `json:"pageNum,omitempty"`
	PageSize *int   `json:"pageSize,omitempty"`
}

// LoginRequest 对应 Java 的 LoginRequest
type LoginRequest struct {
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	AutoLogin *bool   `json:"autoLogin,omitempty"`
}

// SystemInitRequest 对应 Java 的 SystemInitRequest
type SystemInitRequest struct {
	AdminUser *model.User    `json:"adminUser,omitempty"`
	Configs   map[string]any `json:"configs,omitempty"`
}

// ChangePasswordRequest 对应 Java 的 ChangePasswordRequest
type ChangePasswordRequest struct {
	OldPassword *string `json:"oldPassword,omitempty"`
	NewPassword *string `json:"newPassword,omitempty"`
}

// UpdateHigressConfigRequest 对应 Java 的 UpdateHigressConfigRequest
type UpdateHigressConfigRequest struct {
	Config *string `json:"config,omitempty"`
}
