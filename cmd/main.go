package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/gofly/car60"
)

var (
	dirPath          string
	lastModifiedFile *os.File
	lastModifiedLock = &sync.RWMutex{}
	lastModifiedMap  = make(map[string]string)

	service *car60.Car60Service
)

func readLastModifiedFile() error {
	r := bufio.NewReader(lastModifiedFile)
	for {
		line, _, err := r.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		parts := strings.SplitN(string(line), "\t", 2)
		if len(parts) == 2 {
			lastModifiedMap[parts[0]] = parts[1]
		}
	}
	return nil
}

func getLastModified(file string) string {
	lastModifiedLock.RLock()
	defer lastModifiedLock.RUnlock()
	return lastModifiedMap[file]
}

func setLastModified(file, modified string) error {
	lastModifiedLock.Lock()
	defer lastModifiedLock.Unlock()
	lastModifiedMap[file] = modified

	err := lastModifiedFile.Truncate(0)
	if err != nil {
		return err
	}
	for filePath, lastModified := range lastModifiedMap {
		_, err := lastModifiedFile.WriteString(filePath + "\t" + lastModified + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	dirPath = fmt.Sprintf("./pdf/%s", time.Now().Format("2006-01-02"))
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		log.Fatalf("[ERROR] 创建目录[%s]出错：%s\n", dirPath, err)
	}
	lastModifiedFile, err = os.OpenFile("last_modified.txt", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("[ERROR] 打开PFD修改时间文件出错：%s", err)
	}
	err = readLastModifiedFile()
	if err != nil {
		log.Fatalf("[ERROR] 读取PFD修改时间文件出错：%s", err)
	}
	service = car60.NewCar60Service("http://www.car60.com")
}

func downloadPDF(sub string, item *car60.CarItem, pdfURL string) error {
	filePath := path.Join(dirPath, pdfURL)
	lastModified := getLastModified(pdfURL)
	log.Printf("[INFO] ----- 正在下载并保存[%s->%s->%s]PDF文件 -----\n", sub, item.ID, pdfURL)
	err := service.DownloadPDF(pdfURL, lastModified, func(status int, lastModified string, r io.Reader) error {
		switch status {
		case http.StatusOK:
			err := setLastModified(pdfURL, lastModified)
			if err != nil {
				log.Printf("[WARN] 设置文件修改时间[%s]出错：%s\n", lastModified, err)
			}

			parentPath := path.Join(dirPath, path.Dir(pdfURL))
			err = os.MkdirAll(parentPath, 0755)
			if err != nil {
				log.Printf("[ERROR] 创建目录[%s]出错：%s\n", parentPath, err)
				return err
			}
			f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Printf("[ERROR] 创建PDF文件[%s]出错：%s\n", filePath, err)
				return err
			}
			n, err := io.Copy(f, r)
			f.Close()
			if err != nil {
				log.Printf("[ERROR] 下载PDF文件[%s]出错：%s\n", filePath, err)
				return err
			}
			log.Printf("[INFO] PDF文件[%s]大小：%d", filePath, n)
			log.Printf("[INFO] 成功下载并保存[%s->%s->%s]PDF文件\n", sub, item.ID, pdfURL)
		case http.StatusNotModified:
			log.Printf("[INFO] PDF文件[%s->%s->%s]无更新\n", sub, item.ID, pdfURL)
		}
		return nil
	})
	if err != nil {
		log.Printf("[ERROR] 下载PDF文件[%s]出错：%s\n", filePath, err)
	}
	return err
}

func downloadItems(sub string, item car60.CarItem) error {
	if item.Link != "" {
		parentPath := path.Join(dirPath, item.ParentPath)
		err := os.MkdirAll(parentPath, 0755)
		if err != nil {
			log.Printf("[ERROR] 创建目录[%s]出错：%s\n", parentPath, err)
			return err
		}
		filePath := path.Join(parentPath, item.Name+".txt")
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[ERROR] 创建文件[%s]出错：%s\n", filePath, err)
			return err
		}
		fmt.Fprintln(f, "ID：", item.ID)
		fmt.Fprintln(f, "车型：", item.Name)
		fmt.Fprintln(f, "描述：", item.Description)
		fmt.Fprintln(f, "链接：", item.Link)
		fmt.Fprintln(f, "密码：", item.Password)
		f.Close()
	} else {
		log.Printf("[INFO] ----- 正在下载[%s->%s]PDF文件列表 -----\n", sub, item.ID)
		pdfURLList, err := service.PfdURLList(item.ID)
		if err != nil {
			log.Printf("[ERROR] 下载PDF文件列表出错：%s\n", err)
			return err
		}
		log.Printf("[INFO] 成功下载[%s->%s]PDF文件列表\n", sub, item.ID)

		wg := &sync.WaitGroup{}
		pdfURLChan := make(chan string, 5)
		for i := 0; i < cap(pdfURLChan); i++ {
			go func() {
				for pdfURL := range pdfURLChan {
					downloadPDF(sub, &item, pdfURL)
					wg.Done()
				}
			}()
		}
		for _, pdfURL := range pdfURLList {
			wg.Add(1)
			pdfURLChan <- pdfURL
		}
		wg.Wait()
		close(pdfURLChan)
	}
	return nil
}
func main() {
	defer func() {
		lastModifiedFile.Close()
	}()
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s [username] [password]", os.Args[0])
	}

	log.Println("[INFO] ----- 正在登录car60 -----")
	err := service.Login(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatalf("[ERROR] 登录car60出错：%s", err)
	}
	log.Println("[INFO] 成功登录car60")

	log.Println("[INFO] ----- 正在下载维修手册分类 -----")
	list, err := service.CatagoryList()
	if err != nil {
		log.Fatalf("[ERROR] 获取维修手册分类出错： %s", err)
	}
	log.Println("[INFO] 成功下载维修手册分类")

	for _, sub := range list {
		log.Printf("[INFO] ----- 正在下载[%s]车型列表 -----\n", sub)
		items, err := service.CarList(sub)
		if err != nil {
			log.Printf("[ERROR] 获取车型列表出错：%s, URL: %s\n", err, sub)
			time.Sleep(time.Second * 3)
			continue
		}
		log.Printf("[INFO] 成功下载车型列表[%s]车型列表\n", sub)
		for _, item := range items {
			downloadItems(sub, item)
		}
	}
}
