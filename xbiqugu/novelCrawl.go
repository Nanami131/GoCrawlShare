package xbiqugu

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

/*
这里本来想爬的网址是 笔趣阁而不是新笔趣阁
依次尝试了如下几个url
https://www.biquai.cc/
https://www.bi01.cc/
但是在爬取小说的搜索界面时就发生了障碍，详见ReadMe文档
*/

func searchNovel(searchTerm string) error {

	targetURL := "http://www.xbiqugu.la/modules/article/waps.php"

	// 这个网站是通过post修改网页内容，并不是get方式通过query参数查询
	formData := url.Values{
		"searchkey": {searchTerm},
	}
	body := bytes.NewBufferString(formData.Encode())

	client := &http.Client{}

	req, err := http.NewRequest("POST", targetURL, body)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// cookie显然不会过期
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "_abcde_qweasd=0; _abcde_qweasd=0")
	req.Header.Set("Host", "www.xbiqugu.la")
	req.Header.Set("Origin", "http://www.xbiqugu.la")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Proxy-Connection", "keep-alive")
	req.Header.Set("Referer", "http://www.xbiqugu.la/")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("状态码异常: %d", resp.StatusCode)
	}

	// 响应的内容可能存在压缩
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("解压gzip失败: %v", err)
		}
		defer gz.Close()
		reader = gz
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("读取响应内容失败: %v", err)
	}

	htmlContent := string(bodyBytes)
	fmt.Println("服务器返回的原始内容:", htmlContent)

	// 这里借助一下github上大佬的库解析HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return fmt.Errorf("解析HTML失败: %v", err)
	}

	allRowsSelector := "#wrapper > #main > #content > form > table.grid > tbody > tr > td:nth-child(1) > a"
	fmt.Println("\n解析所有结果:")
	doc.Find(allRowsSelector).Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		href, exists := s.Attr("href")
		if !exists {
			href = "未找到链接"
		}
		fmt.Printf("%d: 标题: %s, 链接: %s\n", i+1, title, href)
	})

	if doc.Find(allRowsSelector).Length() == 0 {
		fmt.Println("搜索结果: 未找到任何匹配的小说")
	}

	return nil
}

// GoSearch 提供给用户的接口，控制爬小说的流程
func GoSearch() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("请输入小说名（输入 'exit' 退出）: ")
		scanner.Scan()
		searchTerm := scanner.Text()

		if searchTerm == "exit" {
			break
		}

		err := searchNovel(searchTerm)
		if err != nil {
			fmt.Println("错误:", err)
		}
		fmt.Println("---------------------------------")
	}
}
