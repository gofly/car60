package car60

import (
	"io"
	"os"
	"path"
	"testing"
)

var (
	username = ""
	password = ""
	service  = NewCar60Service("")
)

func TestLogin(t *testing.T) {
	err := service.Login(username, password)
	if err != nil {
		t.Errorf("error: %s", err)
	}
}

func TestCatagoryList(t *testing.T) {
	list, err := service.CatagoryList()
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}
	t.Logf("list: %+v", list)
}

func TestCarList(t *testing.T) {
	list, err := service.CarList("car_list?country=2&factory=135&cat_id=136")
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}
	t.Logf("list: %+v", list)
}

func TestPfdURLList(t *testing.T) {
	list, err := service.PfdURLList("213")
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}
	t.Logf("list: %+v", list)
}

func TestDownloadPDF(t *testing.T) {
	dirPath := "/Volumes/RamDisk/car602"
	filePath := "中国车系/奇瑞/A1/2007/01-动力总成部分.pdf"
	lastModified := "Wed, 19 Oct 2011 03:20:30 GMT"
	err := service.DownloadPDF(filePath, lastModified, func(status int, lastModified string, r io.Reader) error {
		t.Logf("last modified: %s", lastModified)
		err := os.MkdirAll(path.Join(dirPath, path.Dir(filePath)), 0755)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(path.Join(dirPath, filePath), os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		n, err := io.Copy(f, r)
		t.Logf("read bytes: %d", n)
		return err
	})
	if err != nil {
		t.Logf("download error: %s", err)
	}
}
