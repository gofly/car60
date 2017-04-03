package car60

import (
	"fmt"
	"net/http"
	"strings"

	"regexp"

	"net/url"

	"errors"

	"io"

	"github.com/PuerkitoBio/goquery"
)

var (
	trimSpanReg = regexp.MustCompile(`<span.*span>`)
	passwordReg = regexp.MustCompile("密码.*?([a-z0-9]+)")
	urlReg      = regexp.MustCompile("var url='(.*?)';")
)

type CarItem struct {
	ID          string
	Name        string
	ParentPath  string
	Description string
	Link        string
	Password    string
}

type Car60Service struct {
	host   string
	client *http.Client
}

func EscapeURL(u string) string {
	parts := strings.Split(u, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func NewCar60Service(host string) *Car60Service {
	return &Car60Service{
		host: host,
		client: &http.Client{
			Transport: &Transport{},
			Jar:       &CookieJar{},
		},
	}
}

func (s *Car60Service) Login(username, password string) error {
	u := fmt.Sprintf("%s/User/login", s.host)
	resp, err := s.client.PostForm(u, url.Values{
		"username": []string{username},
		"password": []string{password},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status %s", resp.StatusCode)
	}
	return nil
}

func (s *Car60Service) CatagoryList() ([]string, error) {
	u := fmt.Sprintf("%s/data/data", s.host)
	resp, err := s.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %s", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	list := make([]string, 0)
	doc.Find(".aside-cList li dd a.car").Each(func(_ int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}
		list = append(list, href)
	})
	return list, nil
}

func (s *Car60Service) CarList(sub string) ([]CarItem, error) {
	u := fmt.Sprintf("%s/data/%s", s.host, sub)
	resp, err := s.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %s", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	parentPath := strings.Replace(doc.Find(".right h2.factory span").Text(), ">", "/", -1)
	list := make([]CarItem, 0)
	doc.Find(".right .car_list").Each(func(_ int, sel *goquery.Selection) {
		itemID, exists := sel.Find("input[name=data_id]").Attr("value")
		if !exists {
			return
		}
		title, _ := sel.Find("h4").Html()
		item := CarItem{
			ID:         itemID,
			Name:       trimSpanReg.ReplaceAllString(title, ""),
			ParentPath: parentPath,
		}

		descriptions := make([]string, 0)
		sel.Find("li").Each(func(_ int, sel *goquery.Selection) {
			class, exists := sel.Attr("class")
			if exists {
				switch class {
				case "desc":
					desc := sel.Find("p").Text()
					if desc != "" {
						descriptions = append(descriptions, desc)
					}
				case "address":
					item.Link, exists = sel.Find("a").Attr("href")
					if !exists {
						return
					}

					matches := passwordReg.FindStringSubmatch(sel.Text())
					if len(matches) == 2 {
						item.Password = matches[1]
					}
				}
			} else {
				descriptions = append(descriptions, sel.Text())
			}
		})
		item.Description = strings.Join(descriptions, "\n")

		list = append(list, item)
	})

	return list, nil
}

func (s *Car60Service) PfdURLList(id string) ([]string, error) {
	u := fmt.Sprintf("%s/Home/Read/read_index", s.host)
	resp, err := s.client.PostForm(u, url.Values{
		"data_id": []string{id},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %s", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	directories := make([]string, 0)
	doc.Find("script").Each(func(_ int, sel *goquery.Selection) {
		if len(directories) > 0 {
			return
		}
		matches := urlReg.FindStringSubmatch(sel.Text())
		if len(matches) > 0 {
			directories = append(directories, matches[1])
		}
	})
	if len(directories) == 0 {
		return nil, errors.New("body not match")
	}

	list := make([]string, 0)
	doc.Find("ul#tree li span.file").Each(func(_ int, sel *goquery.Selection) {
		parents := make([]string, 0)
		sel.ParentsFiltered("ul").Each(func(_ int, sel *goquery.Selection) {
			parent := sel.PrevAllFiltered("span.folder").Text()
			if parent != "" {
				parents = append(parents, parent)
			}
		})
		for i := 0; i < len(parents)/2; i++ {
			parents[i], parents[len(parents)-1-i] = parents[len(parents)-1-i], parents[i]
		}
		parents = append(directories, parents...)
		parents = append(parents, sel.Text())
		pdfURL := fmt.Sprintf("%s.pdf", strings.Join(parents, "/"))
		list = append(list, pdfURL)
	})

	return list, nil
}

func (s *Car60Service) DownloadPDF(u string, lastModified string, fn func(int, string, io.Reader) error) error {
	u = fmt.Sprintf("%s/Public/pdf/%s", s.host, EscapeURL(u))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		lastModified = resp.Header.Get("Last-Modified")
		return fn(resp.StatusCode, lastModified, resp.Body)
	case http.StatusNotModified:
		return fn(resp.StatusCode, lastModified, nil)
	}

	return fmt.Errorf("http status: %d", resp.StatusCode)
}
