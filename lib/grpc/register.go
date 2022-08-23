package grpc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	pkgerr "github.com/pkg/errors"
	"go.etcd.io/etcd/clientv3"
)

type Register struct {
	etcdClient    *clientv3.Client
	t             *time.Ticker
	servicePrefix string
}

func NewRegister(cli *clientv3.Client, d time.Duration, prefix string) *Register {
	return &Register{
		etcdClient:    cli,
		t:             time.NewTicker(d),
		servicePrefix: prefix,
	}
}

func getServiceKey(servicePrefix, serviceName string) string {
	servicePrefix = strings.TrimSuffix(servicePrefix, "/")
	return strings.Join([]string{servicePrefix, serviceName}, "/")
}

func getNodeServiceKey(servicePrefix, serviceName, address string) string {
	key := getServiceKey(servicePrefix, serviceName)
	nodeKey := address
	return strings.Join([]string{key, nodeKey}, "/")
}

func (r *Register) Deregister(ctx context.Context, serviceName, address string) error {
	r.t.Stop()
	key := getNodeServiceKey(r.servicePrefix, serviceName, address)

	_, err := r.etcdClient.Delete(ctx, key)

	return pkgerr.Wrapf(err, "Deregister service:%s meet error", key)
}

// type ServiceValue struct {
// 	Addr string
// 	//	Weight   int
// 	Disalbed bool
// }

func (r *Register) Register(serviceName, address string) error {

	key := getNodeServiceKey(r.servicePrefix, serviceName, address)
	sv := ServiceValue{Addr: address}
	go func() {
		for range r.t.C {
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*2)
			defer cancel()
			leaseResp, err := r.etcdClient.Grant(ctx, 10)
			if err != nil {
				continue
			}
			ctx, cancel = context.WithTimeout(context.TODO(), time.Second*1)
			defer cancel()
			val, _ := json.Marshal(sv)
			_, err = r.etcdClient.Put(ctx, key, string(val), clientv3.WithLease(leaseResp.ID))
			if err != nil {

			}
		}
	}()

	return nil
}
