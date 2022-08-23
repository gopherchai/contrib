package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pkgerr "github.com/pkg/errors"
)

type Config struct {
	Uri            string
	Db             string
	Username       string
	Password       string
	CollectionName string
}
type Client struct {
	cli *mongo.Client
	cfg Config
}

func New(cfg Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	var opts []*options.ClientOptions
	opt := options.Client().ApplyURI(cfg.Uri)
	opts = append(opts, opt) //
	// options.Client().SetAuth(options.Credential{
	// 	AuthMechanism: "PLAIN",

	// 	AuthSource:  "",
	// 	Username:    cfg.Username,
	// 	Password:    cfg.Password,
	// 	PasswordSet: false,
	// })

	cli, err := mongo.Connect(ctx, opts...)
	return &Client{cli, cfg}, err
}

func (c *Client) Insert(ctx context.Context, db, coll string, data map[string]interface{}) (string, error) {
	var d bson.D
	for k, v := range data {
		d = append(d, bson.E{k, v})

	}
	res, err := c.cli.Database(c.cfg.Db).Collection(coll).InsertOne(ctx, d)
	if err != nil {
		return "", pkgerr.Wrapf(err, "insert db:%s  coll:%s with args:%+v", c.cfg.Db, coll, data)
	}

	return fmt.Sprintf("%+v", res.InsertedID), nil
}

func (c *Client) InsertMany(ctx context.Context, db, coll string, docs []interface{}) (*mongo.InsertManyResult, error) {
	res, err := c.cli.Database(c.cfg.Db).Collection(coll).InsertMany(ctx, docs)

	return res, err
}

func (c *Client) Find(ctx context.Context, db, coll string, filter interface{}) (interface{}, error) {
	cur, err := c.cli.Database(db).Collection(coll).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var res []bson.M

	err = cur.All(ctx, &res)
	return nil, err
}
