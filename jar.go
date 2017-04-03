package car60

import (
	"net/http"
	"net/url"
)

type CookieJar struct {
	cookies []*http.Cookie
}

func (j *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.cookies = cookies
}

func (j *CookieJar) Cookies(u *url.URL) []*http.Cookie {
	return j.cookies
}
