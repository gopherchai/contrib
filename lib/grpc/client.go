package grpc

import (
	"context"
	"time"

	"go.etcd.io/etcd/clientv3"
	//"go.etcd.io/etcd/clientv3/naming"
	pkgerr "github.com/pkg/errors"
	"google.golang.org/grpc"
)

func NewClientWithEcfg(ctx context.Context, prefix, target string, cfg clientv3.Config) (*grpc.ClientConn, error) {
	etcdCli, err := clientv3.New(cfg)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "new ectd meet error:%+v  with cfg:%+v", err, cfg)
	}
	return NewClient(ctx, target, etcdCli, prefix)

}

func NewClient(ctx context.Context, target string, etcdCli *clientv3.Client, prefix string) (*grpc.ClientConn, error) {

	r := &resolver{
		etcdClient:    etcdCli,
		ServicePrefix: prefix,
	}

	//todo 添加回退策略，拦截器
	options := []grpc.DialOption{
		grpc.WithBalancer(grpc.RoundRobin(r)),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		//grpc.WithAuthority()
		grpc.WithBackoffConfig(grpc.BackoffConfig{
			MaxDelay: time.Second,
		}),
	}

	cc, err := grpc.DialContext(ctx, target, options...)
	return cc, pkgerr.Wrapf(err, "new client with args:%+v meet error", []interface{}{target, etcdCli.Endpoints(), etcdCli.Username, etcdCli.Password})

}
