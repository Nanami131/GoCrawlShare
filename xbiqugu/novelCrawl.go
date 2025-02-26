package xbiqugu

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Novel 小说基本信息
type Novel struct {
	Index int
	Title string
	URL   string
}

// Chapter 章节基本信息
type Chapter struct {
	Index int    // 章节序号
	Title string // 章节名称
	URL   string // 章节链接
}

/*
searchNovels 搜索小说
这里本来想爬的网址是 笔趣阁而不是新笔趣阁
依次尝试了如下几个url
https://www.biquai.cc/
https://www.bi01.cc/
但是在爬取小说的搜索界面时就发生了障碍，详见ReadMe文档
*/
func searchNovels(searchTerm string) ([]Novel, error) {

	targetURL := "http://www.xbiqugu.la/modules/article/waps.php"

	// 这个网站是通过post修改网页内容，并不是get方式通过query参数查询
	formData := url.Values{
		"searchkey": {searchTerm},
	}
	body := bytes.NewBufferString(formData.Encode())

	client := &http.Client{}

	req, err := http.NewRequest("POST", targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
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
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码异常: %d", resp.StatusCode)
	}

	// 响应的内容可能存在压缩
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("解压gzip失败: %v", err)
		}
		defer gz.Close()
		reader = gz
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取响应内容失败: %v", err)
	}

	htmlContent := string(bodyBytes)

	//方便调试
	//fmt.Println("服务器返回的原始内容:", htmlContent)

	// 这里借助一下github上大佬的库解析HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %v", err)
	}

	var novels []Novel
	allRowsSelector := "#wrapper > #main > #content > form > table.grid > tbody > tr > td:nth-child(1) > a"
	doc.Find(allRowsSelector).Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		href, exists := s.Attr("href")
		if !exists {
			href = ""
		}
		novels = append(novels, Novel{
			Index: i + 1,
			Title: title,
			URL:   href,
		})
	})

	return novels, nil
}

// crawlNovelChapters 从小说目录页提取章节名称和URL
func crawlNovelChapters(url string) ([]Chapter, error) {

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 请求头与搜索时保持一致即可
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cookie", "_abcde_qweasd=0; _abcde_qweasd=0")
	req.Header.Set("Host", "www.xbiqugu.la")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "http://www.xbiqugu.la/modules/article/waps.php")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码异常: %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("解压gzip失败: %v", err)
		}
		defer gz.Close()
		reader = gz
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %v", err)
	}

	var chapters []Chapter
	chapterSelector := "#list > dl > dd > a"
	doc.Find(chapterSelector).Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		href, exists := s.Attr("href")
		if exists {
			fullURL := "http://www.xbiqugu.la" + href
			chapters = append(chapters, Chapter{
				Index: i + 1,
				Title: title,
				URL:   fullURL,
			})
		}
	})

	if len(chapters) == 0 {
		return nil, fmt.Errorf("未找到任何章节")
	}

	fmt.Println("提取的章节列表:")
	for _, ch := range chapters {
		fmt.Printf("%d: %s - %s\n", ch.Index, ch.Title, ch.URL)
	}

	return chapters, nil
}

// 单线程爬虫
func crawlNovel(url, novelDir string) error {
	// 获取章节列表
	chapters, err := crawlNovelChapters(url)
	if err != nil {
		return fmt.Errorf("获取章节列表失败: %v", err)
	}

	// 单线程调用FetchChapterContent
	for _, chapter := range chapters {
		err := FetchChapterContent(chapter.URL, novelDir, chapter.Title)
		if err != nil {
			fmt.Printf("爬取章节'%s'失败: %v\n", chapter.Title, err)
			continue // 跳过失败的章节，继续处理下一个
		}
	}

	return nil
}

/*
createNovelDir 创建小说存储目录，这里如果已经有同名小说不好处理，感觉直接覆盖也不合理，再交互有些冗余。
经过考量决定如果已经存在同名小说，则不进行下一步操作。
*/
func createNovelDir(novelTitle string) (string, error) {
	baseDir := "Novel"
	novelDir := filepath.Join(baseDir, novelTitle)

	// 创建或者进入第一层目录
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		err = os.Mkdir(baseDir, 0755)
		if err != nil {
			return "", fmt.Errorf("创建Novel目录失败: %v", err)
		}
	}

	// 检查第二层目录是否存在
	if _, err := os.Stat(novelDir); !os.IsNotExist(err) {
		return "", fmt.Errorf("小说文件夹已存在，请删除'%s'后再试", novelDir)
	}
	// 创建第二层目录
	err := os.Mkdir(novelDir, 0755)
	if err != nil {
		return "", fmt.Errorf("创建小说目录失败: %v", err)
	}

	return novelDir, nil
}

// FetchChapterContent 爬取单个章节的方法 每章保存一个文件
func FetchChapterContent(chapterURL, novelDir, chapterTitle string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", chapterURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cookie", "_abcde_qweasd=0; _abcde_qweasd=0")
	req.Header.Set("Host", "www.xbiqugu.la")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", filepath.Dir(chapterURL)+"/") // 动态设置Referer
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

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("解压gzip失败: %v", err)
		}
		defer gz.Close()
		reader = gz
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return fmt.Errorf("解析HTML失败: %v", err)
	}

	contentSelector := "#content"
	content := doc.Find(contentSelector).Text()
	if content == "" {
		return fmt.Errorf("未找到正文内容")
	}

	filePath := filepath.Join(novelDir, chapterTitle+".txt")
	err = ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("保存章节内容失败: %v", err)
	}

	fmt.Printf("已保存章节: %s\n", filePath)
	return nil
}

// GoGetNovel 提供给用户的对外接口，控制爬小说的全部流程
func GoGetNovel() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("请输入小说名（输入 'exit' 退出）: ")
		scanner.Scan()
		searchTerm := scanner.Text()

		if searchTerm == "exit" {
			fmt.Println("程序退出")
			break
		}

		novels, err := searchNovels(searchTerm)
		if err != nil {
			fmt.Println("错误:", err)
			continue
		}

		if len(novels) == 0 {
			fmt.Println("搜索结果: 未找到任何匹配的小说")
			continue
		}

		// 显示搜索结果
		fmt.Println("\n搜索结果:")
		for _, novel := range novels {
			fmt.Printf("%d: 标题: %s, 链接: %s\n", novel.Index, novel.Title, novel.URL)
		}

		// 选择序号
		fmt.Print("\n请输入要选择的小说序号（输入 0 取消）: ")
		scanner.Scan()
		input := scanner.Text()
		var selectedIndex int
		_, err = fmt.Sscanf(input, "%d", &selectedIndex)
		if err != nil || selectedIndex < 0 || selectedIndex > len(novels) {
			fmt.Println("无效的选择，已取消操作")
			continue
		}

		if selectedIndex == 0 {
			fmt.Println("已取消操作")
			continue
		}

		// 确认操作
		selectedNovel := novels[selectedIndex-1]
		fmt.Printf("你选择了: %s (%s)，确认吗？(Y/N): ", selectedNovel.Title, selectedNovel.URL)
		scanner.Scan()
		confirm := strings.ToUpper(strings.TrimSpace(scanner.Text()))
		if confirm != "Y" {
			fmt.Println("已取消操作")
			continue
		}

		novelDir, err := createNovelDir(selectedNovel.Title)
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Printf("正在处理小说: %s, URL: %s\n", selectedNovel.Title, selectedNovel.URL)

		err = crawlNovel(selectedNovel.URL, novelDir)
		if err != nil {
			fmt.Printf("爬取小说失败: %v\n", err)
			continue
		}
		fmt.Println("---------------------------------")
	}

}
