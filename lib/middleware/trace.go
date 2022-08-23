package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"go.uber.org/zap"

	"github.com/gopherchai/contrib/lib/ginext"
	"github.com/gopherchai/contrib/lib/log"
	"github.com/gopherchai/contrib/lib/metadata"
	"github.com/gopherchai/contrib/lib/trace"
	"github.com/gopherchai/contrib/lib/util"
)

var (
	//KeyCtx      = "KeyCtx"
	KeyRequest  = "KeyReq"
	KeyResponse = "KeyResponse"
	KeySpanCtx  = struct{}{}
)

func GetSpanCtx(c *gin.Context) context.Context {
	ctx, _ := c.Get(metadata.KeyContext)

	return ctx.(context.Context)

}
func GetTrace(c *gin.Context) opentracing.Tracer {
	ctx, _ := c.Get(metadata.KeyContext)
	t := ctx.(context.Context).Value(trace.TraceKey)
	return t.(opentracing.Tracer)
}

func MetaContext() func(c *gin.Context) {
	return func(c *gin.Context) {
		//获取用户id，ip,
	}
}

func Trace() func(c *gin.Context) {
	return func(c *gin.Context) {
		spanName := c.Request.Method + "_" + c.Request.RequestURI
		spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
		if err != nil {
			fmt.Println("error:", err)
			span := opentracing.StartSpan(spanName)
			defer span.Finish()
			spanCtx = span.Context()
		}

		span, ctx := opentracing.StartSpanFromContext(context.TODO(), spanName, opentracing.ChildOf(spanCtx))
		defer span.Finish()
		// oldCtx, ok := c.Get(KeyCtx)
		// if !ok {
		// 	oldCtx = ctx
		// }
		var oldCtx context.Context
		val, ok := c.Get(metadata.KeyContext)
		if !ok {
			oldCtx = ctx
		} else {
			oldCtx = val.(context.Context)
		}
		ctx = context.WithValue(oldCtx,
			KeySpanCtx, ctx)

		c.Set(metadata.KeyContext, ctx)

		//添加nginx的request id和span的traceId合并添加
		c.Writer.Header().Set("trace_id", span.Context().(jaeger.SpanContext).TraceID().String())
		c.Next()
		span.SetTag("req_id", span.Context().(jaeger.SpanContext).TraceID().String())
		span.SetOperationName(spanName)
		span.LogKV("a", "b", "c", "d")
		span.SetBaggageItem("bk1", "bv1")

	}
}

func LogV2() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		reqId := requestID(c.Request)
		c.Writer.Header().Set("request_id", reqId)
		// Process request
		c.Next()

		// Log only when path is not being skipped

		if raw != "" {
			path = path + "?" + raw
		}

		fields := []zap.Field{
			zap.String("requestID", reqId),
			zap.Time("time", time.Now()),
			zap.String("latency", time.Now().Sub(start).String()),
			zap.String("clientIp", c.ClientIP()),
			zap.String("method", c.Request.Method),
			zap.Int("status", c.Writer.Status()),
			zap.String("errMsg", c.Errors.ByType(gin.ErrorTypePrivate).String()),

			zap.Int("bodySize", c.Writer.Size()),
			zap.String("path", path),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errMsg0", fmt.Sprintf("%+v", c.Errors[0].Err)))
		}
		lf := log.InfoX
		if c.Writer.Status() >= 500 {
			lf = log.ErrorX
		} else if c.Writer.Status() >= 400 {
			lf = log.WarnX
		}

		res := ginext.GetRes(c)
		response, ok := ginext.GetResponse(c).(ginext.Response)
		if ok {
			if response.Code < 0 {
				lf = log.ErrorX
			}
		} else {
			lf = log.ErrorX
		}
		req := ginext.GetReq(c)

		fields = append(fields, zap.String("res", res), zap.String("req", req))

		lf(ginext.GetCtx(c), "middleware log", fields...)
	}
}

func AddContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		//ip,app,requestId
		md := metadata.MD{
			metadata.RemoteIP:  remoteIP(c.Request),
			metadata.RequestID: requestID(c.Request),
		}
		ctx := metadata.NewContext(context.TODO(), md)
		c.Set(metadata.KeyContext, ctx)

	}
}

func requestID(req *http.Request) string {
	id := req.Header.Get("request_id")
	if id != "" {
		return id
	}

	id, err := util.UUID()
	if err != nil {
		panic(err)
	}
	req.Header.Set("request_id", id)
	return id
}
func remoteIP(req *http.Request) (remote string) {
	var xff = req.Header.Get("X-Forwarded-For")
	if idx := strings.IndexByte(xff, ','); idx > -1 {
		if remote = strings.TrimSpace(xff[:idx]); remote != "" {
			return
		}
	}
	if remote = req.Header.Get("X-Real-IP"); remote != "" {
		return
	}
	remote = req.RemoteAddr[:strings.Index(req.RemoteAddr, ":")]
	return
}

//更改render，将返回值在context中设置，便于打印返回值
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Next()
		req, _ := c.Get(KeyRequest)
		res, _ := c.Get(KeyResponse)
		statusCode := c.Writer.Status()
		lf := log.InfoXf
		if statusCode >= 500 {
			lf = log.ErrorXf
		} else if statusCode >= 400 {
			lf = log.WarnXf
		}
		ctx, _ := c.Get(metadata.KeyContext)
		data, _ := json.Marshal(res)
		lf(ctx.(context.Context), "get req:%+v,get Resp:%s", req, string(data))
	}
}

func Jaeger() gin.HandlerFunc {
	return func(c *gin.Context) {
		var parentSpan opentracing.Span
		spCtx, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(c.Request.Header))
		if err != nil {
			parentSpan = opentracing.StartSpan(c.Request.URL.Path)
			defer parentSpan.Finish()
		} else {
			parentSpan = opentracing.StartSpan(c.Request.URL.Path,
				opentracing.ChildOf(spCtx),
				opentracing.Tag{Key: "test", Value: "http"})
			defer parentSpan.Finish()
		}
		c.Set("tracer", opentracing.GlobalTracer())
		c.Set("ctx", opentracing.ContextWithSpan(context.Background(), parentSpan))
		c.Next()
	}
}
