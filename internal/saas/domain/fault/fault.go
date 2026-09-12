package fault

import "errors"

type Error struct {
	Code    int32
	Message string
}

func (e *Error) Error() string             { return e.Message }
func New(code int32, message string) error { return &Error{code, message} }

var Unauthorized = New(401, "登录已失效，请重新登录")
var Forbidden = New(403, "无权执行此操作")
var NotFound = New(404, "记录不存在")

func Invalid(message string) error  { return New(400, message) }
func Conflict(message string) error { return New(409, message) }
func Code(err error) (int32, string) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, e.Message
	}
	return 500, "服务暂时不可用"
}
