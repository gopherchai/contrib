package errors

type CodeExt struct {
	Codes
	Msg string //用于同一个错误码展示不同的错误信息
}

func (cet CodeExt) Message() string {
	return cet.Msg
}

//对于同样的错误码需要定制不一样的错误信息的场景，用这种来覆盖 例如数据校验需要提示用户第n条数据有问题的场景
func NewErrorWithMessage(code Codes, msg string) error {
	return CodeExt{
		Codes: code,
		Msg:   msg,
	}
}
