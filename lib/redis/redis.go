package redis

import (
	"github.com/gopherchai/contrib/lib/config"

	"github.com/go-redis/redis"
	//"github.com/gomodule/redigo/redis"
)

type Client struct {
	*redis.Client
	Cfg       Config
	Namespace string
}

func NewClient(cfg Config, ns string) *Client {

	c := redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    cfg.Addr,

		OnConnect: func(rc *redis.Conn) error {
			return rc.Ping().Err()
		},
		Password:           cfg.Password,
		DB:                 cfg.Index,
		MaxRetries:         0,
		MinRetryBackoff:    0,
		MaxRetryBackoff:    0,
		DialTimeout:        cfg.DialTimeout.Duration.Duration,
		ReadTimeout:        cfg.ReadTimeout.Duration.Duration,
		WriteTimeout:       cfg.WriteTimeout.Duration.Duration,
		PoolSize:           cfg.PoolSize,
		MinIdleConns:       cfg.MinIdleConns,
		MaxConnAge:         cfg.MaxConnAge.Duration.Duration,
		PoolTimeout:        0,
		IdleTimeout:        cfg.IdleTimeout.Duration.Duration,
		IdleCheckFrequency: 0,
	})
	return &Client{Client: c, Namespace: ns}
}

type Config struct {
	Password     string
	Index        int
	Addr         string
	MaxIdle      int
	PoolSize     int
	MinIdleConns int
	DialTimeout  config.Duration
	MaxConnAge   config.Duration
	ReadTimeout  config.Duration
	WriteTimeout config.Duration
	IdleTimeout  config.Duration
}

// type Client struct {
// 	*redis.Pool
// }

// func NewClient(cfg Config) *Client {

// 	options := []redis.DialOption{redis.DialPassword(cfg.Password),
// 		redis.DialDatabase(cfg.Index),
// 		redis.DialReadTimeout(time.Second),
// 		redis.DialWriteTimeout(time.Second),
// 		redis.DialConnectTimeout(time.Second)}
// 	p := redis.NewPool(func() (redis.Conn, error) {
// 		return redis.Dial("tcp", cfg.Addr, options...)
// 	}, cfg.MaxIdle)

// 	return &Client{Pool: p}
// }

// func (c *Client) GetStats() redis.PoolStats {
// 	return c.Stats()
// }
// func (c *Client) Set(key,val string ,args...interface{}){
// 	rc:=c.Pool.Get()
// 	defer rc.Close()
// 	rc.se
// }
