package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gopherchai/contrib/lib/log"
	"github.com/gopherchai/contrib/lib/util"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	//grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"

	pkgerr "github.com/pkg/errors"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Server struct {
	r           *Register
	s           *grpc.Server
	serviceName string
	host        string
	port        int
}

func NewServer(serviceName string, port int, cfg clientv3.Config, d time.Duration, prefix string) (*Server, error) {
	etcdCli, err := clientv3.New(cfg)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "new ectd meet error:%+v  with cfg:%+v", err, cfg)
	}
	r := NewRegister(etcdCli, d, prefix)
	return NewServerWithRegister(serviceName, port, r), nil
}

func NewServerWithRegister(serviceName string, port int, r *Register) *Server {
	host := util.GetMacOrLinuxLocalIP()
	//TODO 添加拦截器
	s := grpc.NewServer(grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_opentracing.StreamServerInterceptor(),
		grpc_prometheus.StreamServerInterceptor,
		//grpc_zap.StreamServerInterceptor(zapLogger),
		//grpc_auth.StreamServerInterceptor(myAuthFunction),
		grpc_recovery.StreamServerInterceptor(),
	)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_opentracing.UnaryServerInterceptor(),
			grpc_prometheus.UnaryServerInterceptor,
			//grpc_zap.UnaryServerInterceptor(zapLogger),
			//grpc_auth.UnaryServerInterceptor(myAuthFunction),
			grpc_recovery.UnaryServerInterceptor(),
		)))

	return &Server{
		r:           r,
		s:           s,
		serviceName: serviceName,
		host:        host,
		port:        port,
	}
}

func (s *Server) GetRawServer() *grpc.Server {
	return s.s
}
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return pkgerr.Wrapf(err, "host:%s service :%s start meet error", s.host, s.serviceName)
	}
	hostIp := util.GetMacOrLinuxLocalIP()
	addr := hostIp + ":" + strconv.Itoa(s.port)
	err = s.r.Register(s.serviceName, addr)
	if err != nil {
		return pkgerr.Wrapf(err, "host:%s service :%s register meet error", s.host, s.serviceName)
	}
	err = s.s.Serve(lis)

	return pkgerr.Wrapf(err, "host:%s service :%s serve meet error", s.host, s.serviceName)
}

func (s *Server) Stop() {
	err := s.r.Deregister(context.TODO(), s.serviceName, fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		log.WarnX(context.TODO(), "register failed", zap.Error(err))
	}
	s.s.GracefulStop()

	return
}
