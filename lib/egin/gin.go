package egin

import (
	"bytes"
	"context"

	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gopherchai/contrib/lib/ginext"
	"github.com/gopherchai/contrib/lib/model"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

type Server struct {
	api      *gin.Engine
	inter    *gin.Engine
	svr      *http.Server
	interSvr *http.Server
	quitCh   chan os.Signal

	cfg ServerConfig
}

func initPrometheus(appName, subsystem string) {
	handlerCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: appName,
		Subsystem: subsystem,
		Name:      "http_request_counter",
		Help:      "",
	}, []string{"method", "uri", "code"})
	handlerObsVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: appName,
		Subsystem: subsystem,
		Name:      "obs",
		Help:      "",
		Buckets:   []float64{100, 200, 500, 1000, 1500, 2000, 3000, 5000, 10000, 20000},
	}, []string{"method", "uri", "code"})
	handlerGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: appName,
		Subsystem: subsystem,
		Name:      "request_gauge",
		Help:      "",
	}, []string{"method", "uri"})
	Aobs = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   appName,
		Subsystem:   subsystem,
		Name:        "aobs",
		Help:        "",
		ConstLabels: map[string]string{},
	})
	prometheus.MustRegister(handlerCounterVec, handlerObsVec, handlerGauge, Aobs)
}

var (
	handlerCounterVec *prometheus.CounterVec

	handlerObsVec *prometheus.HistogramVec

	handlerGauge *prometheus.GaugeVec
	Aobs         prometheus.Histogram
)

func getHttpHandler(r *gin.Engine, c *gin.Context) {
	r.ServeHTTP(c.Writer, c.Request)
}

type ServerConfig struct {
	Port        int
	PprofPort   int
	AppName     string
	SubSystem   string
	AuthKey     string
	ReadTimeout model.Duration

	ListenKeepAlive bool
	KeepAlivePeriod model.Duration

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers. The connection's read deadline is reset
	// after reading the headers and the Handler can decide what
	// is considered too slow for the body. If ReadHeaderTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	ReadHeaderTimeout model.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	WriteTimeout model.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	IdleTimeout model.Duration

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line. It does not limit the
	// size of the request body.
	// If zero, DefaultMaxHeaderBytes is used.
	MaxHeaderBytes int
}

func NewServer(sc ServerConfig) *Server {
	initPrometheus(sc.AppName, sc.SubSystem)
	r := gin.New()

	r.GET("/metrics", func(c *gin.Context) {
		h := promhttp.Handler()
		h.ServeHTTP(c.Writer, c.Request)
	})

	inter := gin.New()
	prefixRouter := inter.Group("/debug/pprof")
	{
		prefixRouter.GET("/", pprofHandler(pprof.Index))
		prefixRouter.GET("/cmdline", pprofHandler(pprof.Cmdline))
		prefixRouter.GET("/profile", pprofHandler(pprof.Profile))
		prefixRouter.POST("/symbol", pprofHandler(pprof.Symbol))
		prefixRouter.GET("/symbol", pprofHandler(pprof.Symbol))
		prefixRouter.GET("/trace", pprofHandler(pprof.Trace))
		prefixRouter.GET("/allocs", pprofHandler(pprof.Handler("allocs").ServeHTTP))
		prefixRouter.GET("/block", pprofHandler(pprof.Handler("block").ServeHTTP))
		prefixRouter.GET("/goroutine", pprofHandler(pprof.Handler("goroutine").ServeHTTP))
		prefixRouter.GET("/heap", pprofHandler(pprof.Handler("heap").ServeHTTP))
		prefixRouter.GET("/mutex", pprofHandler(pprof.Handler("mutex").ServeHTTP))
		prefixRouter.GET("/threadcreate", pprofHandler(pprof.Handler("threadcreate").ServeHTTP))

	}

	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	return &Server{
		api:   r,
		inter: inter,
		svr: &http.Server{
			Addr:              fmt.Sprintf(":%d", sc.Port),
			Handler:           r,
			ReadTimeout:       sc.ReadTimeout.Duration,
			ReadHeaderTimeout: sc.ReadHeaderTimeout.Duration,
			WriteTimeout:      sc.WriteTimeout.Duration,
		},
		interSvr: &http.Server{
			Addr:              fmt.Sprintf(":%d", sc.PprofPort),
			Handler:           inter,
			ReadTimeout:       sc.ReadTimeout.Duration,
			ReadHeaderTimeout: sc.ReadHeaderTimeout.Duration,
			WriteTimeout:      sc.WriteTimeout.Duration,
		},
		quitCh: q,
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

type PanicLogger interface {
	WarnXf(ctx context.Context, msg string, args ...interface{})
	ErrorXf(ctx context.Context, msg string, args ...interface{})
}

func Panic(logger PanicLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}
				ctx := ginext.GetCtx(c)
				stackCount := stack(3)
				if logger != nil {

					httpRequest, _ := httputil.DumpRequest(c.Request, false)
					headers := strings.Split(string(httpRequest), "\r\n")
					for idx, header := range headers {
						current := strings.Split(header, ":")
						if current[0] == "Authorization" {
							headers[idx] = current[0] + ": *"
						}
					}
					if brokenPipe {
						logger.ErrorXf(ctx, fmt.Sprintf("%s\n%s%s", err, string(httpRequest), reset))
					} else {
						logger.ErrorXf(ctx, fmt.Sprintf("[Recovery]  panic recovered:\n%s\n%s%s",
							err, stackCount, reset))
					}
				} else {
					fmt.Printf("[Recovery]  panic recovered:\n%+v\n%s%s", err, stackCount, reset)
				}

			}
		}()
		c.Next()
	}
}

func (s *Server) AddPrometheus() {
	s.api.Use(Prometheus())
}

func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		now := time.Now()

		c.Next()
		duration := time.Since(now)
		d := duration.Milliseconds()
		code := c.Writer.Status()
		path := strings.Split(req.RequestURI, "?")[0]
		handlerCounterVec.WithLabelValues(req.Method, path, strconv.Itoa(int(code))).Inc()
		handlerObsVec.WithLabelValues(req.Method, path, strconv.Itoa(int(code))).Observe(float64(d))
		handlerGauge.WithLabelValues(req.Method, path).Set(float64(duration / time.Millisecond))

		Aobs.Observe(float64(duration / time.Millisecond))

	}
}

func (s *Server) RegisterMiddlewares(h ...gin.HandlerFunc) {
	s.api.Use(h...)

}

func (s *Server) RegisterApi(f func(e *gin.Engine)) {
	f(s.api)
}

func (s *Server) Run() chan error {
	var errChan chan error
	fmt.Println("start run")
	if s.cfg.ListenKeepAlive {
		lc := net.ListenConfig{
			KeepAlive: s.cfg.KeepAlivePeriod.Duration,
		}
		l, err := lc.Listen(context.TODO(), "tcp", s.svr.Addr)
		if err != nil {
			errChan <- err
		}
		go func() {
			err = s.svr.Serve(l)
			if err != nil {
				errChan <- err
			}
		}()
	} else {
		go func() {
			err := s.svr.ListenAndServe()
			if err != nil {
				errChan <- err
			}
		}()

	}
	go func() {
		err := s.interSvr.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	return errChan
}

func pprofHandler(h http.HandlerFunc) gin.HandlerFunc {
	handler := http.HandlerFunc(h)
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Server) Stop(ctx context.Context) {
	fmt.Println("start to stop")
	s.svr.Shutdown(ctx)
	s.interSvr.Shutdown(ctx)
}

func stack(skip int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}

func timeFormat(t time.Time) string {
	var timeString = t.Format("2006/01/02 - 15:04:05")
	return timeString
}
