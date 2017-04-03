package car60

import (
	"net/http"
)

type Transport struct{}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,en-US;q=0.6,en;q=0.4")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/57.0.2987.98 Safari/537.36")
	req.Header.Set("Referer", "http://www.car60.com")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	return http.DefaultTransport.RoundTrip(req)
}
