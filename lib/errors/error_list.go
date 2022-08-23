package errors

var (
	ErrNil                     = add(0)
	ErrIDNotExistInDataBase    = New(1)
	ErrQualifiedRecordNotFound = New(100)
	ErrParameter               = New(400)

	ErrSystem = add(-1) //小于0的错误定义为系统错误，大于0的定义为业务错误
)
