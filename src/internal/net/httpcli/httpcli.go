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

func Do2xxWithTimeout(m, url string, t time.Duration, data io.Reader, h ...string) (io.Reader, http.Header, int, error) {
	req, err := http.NewRequest(m, url, data)
	if err != nil {
		return nil, nil, 0, err
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if t > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), t)
		defer cancel()
		req = req.WithContext(ctx)
	}

	makeHeader(req.Header, h...)

	res, err := cli.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer closeBody(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, nil, res.StatusCode, fmt.Errorf("request failed with code %v", res.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		return nil, nil, 0, err
	}

	return buf, res.Header, res.StatusCode, nil
}

func closeBody(c io.Closer) {
	if c != nil {
		_ = c.Close()
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
