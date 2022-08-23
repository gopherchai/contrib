package trace

import (
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"

	"github.com/gopherchai/contrib/lib/log"
)

var (
	TraceKey = struct{}{}
)

func Init() io.Closer {
	cfg := &config.Configuration{
		ServiceName: "serviceName",
		Disabled:    false,
		RPCMetrics:  false,
		Tags:        nil,
		Sampler: &config.SamplerConfig{
			Type:                     jaeger.SamplerTypeConst,
			Param:                    1,
			SamplingServerURL:        "",
			SamplingRefreshInterval:  0,
			MaxOperations:            0,
			OperationNameLateBinding: false,
			Options:                  nil,
		},
		Reporter: &config.ReporterConfig{
			QueueSize:                  0,
			BufferFlushInterval:        0,
			LogSpans:                   true,
			LocalAgentHostPort:         "127.0.0.1:5778",
			DisableAttemptReconnecting: false,
			AttemptReconnectInterval:   0,
			CollectorEndpoint:          "http://127.0.0.1:14268/api/traces",
			User:                       "",
			Password:                   "",
			HTTPHeaders:                map[string]string{},
		},
		Headers: &jaeger.HeadersConfig{
			JaegerDebugHeader:        "",
			JaegerBaggageHeader:      "",
			TraceContextHeaderName:   "",
			TraceBaggageHeaderPrefix: "",
		},
		BaggageRestrictions: &config.BaggageRestrictionsConfig{
			DenyBaggageOnInitializationFailure: false,
			HostPort:                           "",
			RefreshInterval:                    0,
		},
		Throttler: &config.ThrottlerConfig{
			HostPort:                  "",
			RefreshInterval:           0,
			SynchronousInitialization: false,
		},
	}
	// cfg, err := config.FromEnv()
	// if err != nil {
	// 	panic(err)
	// }

	cfg.ServiceName = "test"

	t, closer, err := cfg.NewTracer(config.Logger(log.GetDefaultLogger()))
	if err != nil {
		panic(err)
	}

	opentracing.SetGlobalTracer(t)
	return closer
}
