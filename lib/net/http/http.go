package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	pkgerr "github.com/pkg/errors"
)

type HttpClient struct {
	UrlPrefix string //域名前缀http://www.biadu.com
	cli       *http.Client
}

func NewClient(prefix string) *HttpClient {
	return &HttpClient{
		UrlPrefix: prefix,
		cli: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
}

func (hc *HttpClient) PostJSONInvoke(ctx context.Context, args, reply interface{}, uri string) error {
	data, err := json.Marshal(args)
	if err != nil {
		return pkgerr.Wrapf(err, "marshal args:%+v meet error:%+v", args, err)
	}
	buf := bytes.NewBuffer(data)
	req, err := http.NewRequest(http.MethodPost, hc.UrlPrefix+uri, buf)
	if err != nil {
		return pkgerr.Wrapf(err, "new req args:%+v meet error:%+v", []interface{}{args, uri, hc.UrlPrefix}, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return DoReq(hc, req, ctx, args, reply, uri)

}

func DoReq(hc *HttpClient, req *http.Request, ctx context.Context, args, reply interface{}, uri string) error {
	req = req.WithContext(ctx)
	resp, err := hc.cli.Do(req)
	if err != nil {
		return pkgerr.Wrapf(err, "do req args:%+v meet error:%+v", []interface{}{args, uri, hc.UrlPrefix}, err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return pkgerr.Wrapf(err, "read body for  req args:%+v meet error:%+v", []interface{}{args, uri, hc.UrlPrefix}, err)
	}
	err = json.Unmarshal(data, reply)
	return pkgerr.Wrapf(err, "unmarshal body for  req args:%+v meet error:%+v", []interface{}{args, uri, hc.UrlPrefix}, err)
}

func (hc *HttpClient) PostFormDataInvoke(ctx context.Context, args map[string]string, reply interface{}, uri string) error {
	var u = make(url.Values)
	for k, v := range args {
		u.Add(k, v)
	}

	buf := bytes.NewBuffer([]byte(u.Encode()))
	req, err := http.NewRequest(http.MethodPost, hc.UrlPrefix+uri, buf)
	if err != nil {
		return pkgerr.Wrapf(err, "new req args:%+v meet error:%+v", []interface{}{args, uri, hc.UrlPrefix}, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return DoReq(hc, req, ctx, args, reply, uri)
}
