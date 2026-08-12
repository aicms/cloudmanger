package model

import "time"

// BaseModel 通用基础字段
type BaseModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// 统一 API 响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 分页请求参数
type PageQuery struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Keyword  string `form:"keyword" json:"keyword"`
}

// 分页结果
type PageResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func (q *PageQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
}

func (q *PageQuery) Offset() int {
	return (q.Page - 1) * q.PageSize
}

func Ok(data interface{}) Response {
	return Response{Code: 0, Message: "ok", Data: data}
}

func Fail(message string) Response {
	return Response{Code: 1, Message: message}
}

func FailWith(code int, message string) Response {
	return Response{Code: code, Message: message}
}
