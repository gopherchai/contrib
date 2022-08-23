package model

import "time"

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type ApiAuthKey struct {
	Id           int64
	AppAccessId  string
	AppSecretKey string
}
