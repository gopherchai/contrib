package mongo

import (
	"context"
	"testing"
)

var (
	cli *Client
)

func setup(t *testing.T) {
	var err error
	cfg := Config{
		Uri:            "mongodb://root:example@localhost:27018",
		Db:             "db",
		Username:       "",
		Password:       "",
		CollectionName: "coll",
	}
	cli, err = New(cfg)
	if err != nil {
		t.Errorf("declare client meet error:%+v with cfg:%+v", err, cfg)
	}
}

func TestClient_Insert(t *testing.T) {
	setup(t)
	type args struct {
		ctx  context.Context
		db   string
		coll string
		data map[string]interface{}
	}
	tests := []struct {
		name string

		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				db:   "bpm",
				coll: "user",
				data: map[string]interface{}{
					"user_id": 100,
					"age":     3,
					"sex":     "men",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := cli.Insert(tt.args.ctx, tt.args.db, tt.args.coll, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Insert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Client.Insert() = %v, want %v", got, tt.want)
			}
		})
	}
}
