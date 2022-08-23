package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/naming"
)

const (
//servicePrefix = "/tal/codemonkey/"
)

type resolver struct {
	etcdClient    *clientv3.Client
	ServicePrefix string
}

func NewResolver(client *clientv3.Client) *resolver {

	return &resolver{
		etcdClient: client,
	}
}
func (r *resolver) Resolve(target string) (naming.Watcher, error) {
	return r.Watch(context.TODO(), target)
}

func (r *resolver) Watch(ctx context.Context, serviceName string) (naming.Watcher, error) {

	w := newWatcher(r.etcdClient, r.ServicePrefix, serviceName)
	return w, nil
}

// func newWatcher(prefix string, cli *clientv3.Client) *watcher {
// 	w := &watcher{
// 		watchKeyPrefix: prefix,
// 		etcdClient:     cli,
// 		//updates:        make(chan []*naming.Update),
// 	}
// 	ctx := context.TODO()
// 	resp, err := w.etcdClient.Get(ctx, w.watchKeyPrefix, clientv3.WithPrefix())
// 	if err != nil {
// 		panic(err)
// 	}
// 	w.updates = getUpdates(resp)
// 	w.initialized = true
// 	w.rev = resp.Header.Revision
// 	fmt.Printf("%+v\n", w.updates[0])
// 	go w.watchLoop(ctx)

// 	return w
// }

// // func (w *watcher) init() error {

// // 	ctx := context.TODO()

// // 	resp, err := w.etcdClient.Get(ctx, w.watchKeyPrefix, clientv3.WithPrefix())
// // 	if err != nil {
// // 		return pkgerr.Wrapf(err, "watcher key prefix:%s meet error", w.watchKeyPrefix)
// // 	}
// // 	for _, kv := range resp.Kvs {
// // 		kv.Value
// // 	}
// // }

// func getUpdates(resp *clientv3.GetResponse) []*naming.Update {
// 	updates := make([]*naming.Update, 0, 0)
// 	for _, kv := range resp.Kvs {
// 		var sk ServiceValue
// 		err := json.Unmarshal(kv.Value, &sk)
// 		if err != nil {

// 			continue
// 		}

// 		updates = append(updates, &naming.Update{
// 			Op:       naming.Add,
// 			Addr:     sk.Addr,
// 			Metadata: sk.MetaData,
// 		})
// 	}
// 	return updates
// }
// func (w *watcher) watchLoop(ctx context.Context) {

// 	respChan := w.etcdClient.Watch(ctx, w.watchKeyPrefix, clientv3.WithRev(w.rev), clientv3.WithPrefix(), clientv3.WithPrevKV())
// 	for resp := range respChan {
// 		w.err = resp.Err()
// 		if w.err != nil {
// 			log.Printf("%+v\n", w.err)
// 			continue
// 		}
// 		if w.rev == resp.Header.Revision {
// 			continue
// 		}
// 		//TODO 支持将以下划线开头的作为配置中心对服务实例下线的标记
// 		updates := make([]*naming.Update, 0, 0)
// 		for _, event := range resp.Events {
// 			var kv *mvccpb.KeyValue
// 			var op naming.Operation
// 			switch event.Type {
// 			case mvccpb.DELETE:
// 				op = naming.Delete
// 				kv = event.PrevKv
// 			case mvccpb.PUT:
// 				op = naming.Add
// 				kv = event.Kv
// 			}

// 			var sk ServiceValue
// 			err := json.Unmarshal(kv.Value, &sk)
// 			if err != nil {

// 				continue
// 			}

// 			updates = append(updates, &naming.Update{
// 				Op:       op,
// 				Addr:     sk.Addr,
// 				Metadata: sk.MetaData,
// 			})

// 		}

// 		if len(updates) > 0 {

// 			w.updates = updates
// 		}
// 	}
// }

// func (w *watcher) Next() ([]*naming.Update, error) {
// 	return w.updates, w.err

// }

// func (w *watcher) Close() {
// 	w.etcdClient.Close()
// 	return
// }

// type watcher struct {
// 	watchKeyPrefix string

// 	etcdClient  *clientv3.Client
// 	updates     []*naming.Update
// 	err         error
// 	initialized bool
// 	rev         int64
// }

//先对watcher初始化

type ServiceValue struct {
	Addr     string //like 192.168.1.2:80
	MetaData interface{}
}

type watcher struct {
	client        *clientv3.Client
	closed        bool
	serviceName   string
	servicePrefix string
	isInitialized bool
	updates       chan []*naming.Update
	revision      int64
}

func newWatcher(client *clientv3.Client, prefix, serviceName string) *watcher {
	w := &watcher{
		client:        client,
		serviceName:   serviceName,
		servicePrefix: prefix,
		updates:       make(chan []*naming.Update),
	}
	go func() {
		count := 1
		for {
			err := w.watch()
			if err != nil {
				fmt.Printf("%+v\n", err)
			}
			if w.closed {
				return
			}
			time.Sleep(time.Second * time.Duration(count))
			if count < 10 {
				count++
			}
		}
	}()
	return w
}

func (w *watcher) Close() {
	w.closed = true
	w.client.Close()
}

func (w *watcher) watch() error {
	key := getServiceKey(w.servicePrefix, w.serviceName)

	if !w.isInitialized {
		ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := w.client.Get(ctx, key, clientv3.WithPrefix())
		if err != nil {
			return err
		}

		if resp != nil && len(resp.Kvs) > 0 {
			updates := []*naming.Update{}
			for _, kv := range resp.Kvs {
				if v := kv.Value; v != nil {
					var sk ServiceValue
					err := json.Unmarshal(v, &sk)
					if err != nil {
						continue
					}

					updates = append(updates, &naming.Update{Op: naming.Add, Addr: sk.Addr})

				}
			}
			if len(updates) > 0 {
				w.updates <- updates
				w.isInitialized = true
			}
		}
	}
	opts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithPrevKV(),
	}
	if w.revision > 0 {
		opts = append(opts, clientv3.WithRev(w.revision))
	}
	rch := w.client.Watch(context.Background(), key, opts...)
	for wresp := range rch {
		//logkit.Debugf(("[D][shaco] grpc etcd get watcher resp:%s", wresp.Header)
		w.revision = wresp.Header.Revision
		updates := []*naming.Update{}
		for _, ev := range wresp.Events {
			var op naming.Operation
			var kv *mvccpb.KeyValue
			switch ev.Type {
			case mvccpb.PUT:
				op = naming.Add
				kv = ev.Kv
			case mvccpb.DELETE:
				op = naming.Delete
				kv = ev.PrevKv
			}
			var sk ServiceValue
			err := json.Unmarshal(kv.Value, &sk)
			if err != nil {
				continue
			}

			updates = append(updates, &naming.Update{Op: op, Addr: sk.Addr})
		}
		if len(updates) > 0 {
			w.updates <- updates
		}
	}
	return errors.New("etcd watch faild")
}

func (w *watcher) Next() ([]*naming.Update, error) {
	update := <-w.updates
	return update, nil
}
