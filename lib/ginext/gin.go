package ginext

import (
	"context"

	"encoding/json"
	"fmt"
	"net/http"
	"os"

	ecode "github.com/gopherchai/contrib/lib/errors"

	"github.com/gopherchai/contrib/lib/metadata"

	"github.com/gin-gonic/gin"
)

const (
	KeyResponse = "KeyResponse"

	KeyRequest = "KeyRequest"
)

func GetCtx(c *gin.Context) context.Context {
	ctx, ok := c.Get(metadata.KeyContext)
	if ok {
		return ctx.(context.Context)
	}
	return context.TODO()
}

func SetRequest(c *gin.Context, req interface{}) {
	c.Set(metadata.KeyRequest, req)
}

func SetResp(c *gin.Context, resp Response) {
	c.Set(metadata.KeyResponse, resp)
}

func GetResponse(c *gin.Context) interface{} {
	res, _ := c.Get(metadata.KeyResponse)
	return res
}

func GetReq(c *gin.Context) string {
	req, _ := c.Get(metadata.KeyRequest)
	data, _ := json.Marshal(req)
	return string(data)
}

func GetRes(c *gin.Context) string {
	res := GetResponse(c)
	if res != nil {
		data, _ := json.Marshal(res)
		return string(data)
	}
	return ""
}

func GetJSONParam(c *gin.Context, req interface{}) (context.Context, error) {
	err := c.ShouldBindJSON(req)
	if err != nil {
		BadParameter(c, err)
		return nil, err
	}

	SetRequest(c, req)
	ctx, ok := c.Get(metadata.KeyContext)
	if ok {
		return ctx.(context.Context), err
	}
	return context.TODO(), err

}

func BadParameter(c *gin.Context, err error) {

	resp := Response{
		Code: ecode.ErrParameter.Code(),
		Msg:  ecode.ErrParameter.Message(),
		Data: struct{}{},
	}

	if err != nil {

		e, ok := err.(ecode.Codes)
		if ok {
			resp.Code = e.Code()
			resp.Msg = e.Message()
		} else {
			c.Error(err)
		}
		resp = setRespStack(resp, err)

	}
	SetResp(c, resp)

	c.JSON(http.StatusOK, resp)
}

func setRespStack(resp Response, err error) Response {
	if os.Getenv("env") == "" && err != nil {
		resp.Stack = fmt.Sprintf("%+v", err)
		e, ok := err.(ecode.CodeExt)
		if ok {
			resp.Stack = fmt.Sprintf("%+v", e.RawErr)
		}
	}
	return resp

}
func RendResponse(c *gin.Context, err error, data interface{}) {
	if data == nil {
		data = struct{}{}
	}
	resp := Response{
		Code: ecode.ErrNil.Code(),
		Msg:  ecode.ErrNil.Message(),
		Data: data,
	}
	resp = setRespStack(resp, err)
	statusCode := http.StatusOK
	if err != nil {
		c.Error(err)
		e, ok := err.(ecode.Codes)
		if !ok {
			resp.Code = ecode.ErrSystem.Code()
			resp.Msg = ecode.ErrSystem.Message()
		} else {
			resp.Code = e.Code()
			resp.Msg = e.Message()
		}
	}
	if resp.Code < 0 {
		statusCode = http.StatusBadGateway
	}

	SetResp(c, resp)
	c.JSON(statusCode, resp)
}

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`

	Stack string `json:"stack"` //debug环境将错误堆栈信息返回给前端
}
