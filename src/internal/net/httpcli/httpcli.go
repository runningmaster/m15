package httpcli

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	cli  *http.Client
	ctx1 context.Context
)

func init() {
	cli = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // workaround
			},
		},
	}
}

func DoWithTimeout(m, url string, d time.Duration, data io.Reader, h ...string) (int, http.Header, io.Reader, error) {
	req, err := http.NewRequest(m, url, data)
	if err != nil {
		return 0, nil, nil, err
	}

	if d > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()
		req = req.WithContext(ctx)
	}

	makeHeader(req.Header, h...)

	res, err := cli.Do(req)
	if res != nil {
		defer closeBody(res.Body)
	}
	if err != nil {
		return 0, nil, nil, err
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		return 0, nil, nil, err
	}

	return res.StatusCode, res.Header, buf, nil
}

func DoWithTimeoutAndMust2xx(m, url string, t time.Duration, data io.Reader, h ...string) (http.Header, io.Reader, error) {
	code, head, body, err := DoWithTimeout(m, url, t, data, h...)
	if err != nil {
		return nil, nil, err
	}

	if code < http.StatusOK || code > http.StatusIMUsed {
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(body)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("request failed with code %d and msg: %s", code, buf.String())
	}

	return head, body, nil
}

func closeBody(c io.Closer) {
	if c == nil {
		return
	}
	err := c.Close()
	if err != nil {
		panic(err)
	}
}

func makeHeader(h http.Header, headers ...string) {
	var s []string
	for _, v := range headers {
		s = strings.Split(v, ":")
		if len(s) < 2 {
			continue
		}
		h.Set(strings.TrimSpace(s[0]), strings.TrimSpace(s[1]))
	}
}
