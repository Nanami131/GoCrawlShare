package xbiqugu

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

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
	Index int
	Title string
	URL   string
}

// DownloadStatus 确认爬到的章节顺序以及状态的信号量
type DownloadStatus struct {
	Seq     int
	Title   string
	Success bool
}

// StartGUI 图形化交互接口
func StartGUI() {
	a := app.New()
	w := a.NewWindow("小说爬虫")
	w.Resize(fyne.NewSize(800, 600)) // 设置初始窗口大小

	input := widget.NewEntry()
	input.SetPlaceHolder("请输入小说名")

	resultList := widget.NewList(
		func() int { return 0 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {},
	)
	resultScroll := container.NewVScroll(resultList)
	resultScroll.SetMinSize(fyne.NewSize(0, 200))

	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	statusScroll := container.NewVScroll(statusLabel)
	statusScroll.SetMinSize(fyne.NewSize(0, 200))

	// 输出重定向函数
	updateStatus := func(text string) {
		current := statusLabel.Text
		if current == "" {
			statusLabel.SetText(text)
		} else {
			statusLabel.SetText(current + "\n" + text)
		}
		statusScroll.ScrollToBottom()
	}

	searchBtn := widget.NewButton("搜索", func() {
		novels, err := searchNovels(input.Text)
		if err != nil {
			updateStatus(fmt.Sprintf("错误: %v", err))
			return
		}
		if len(novels) == 0 {
			updateStatus("搜索结果: 未找到任何匹配的小说")
			return
		}
		updateStatus("\n搜索结果:")
		resultList.Length = func() int { return len(novels) }
		resultList.CreateItem = func() fyne.CanvasObject { return widget.NewLabel("") }
		resultList.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(fmt.Sprintf("%d: 标题: %s, 链接: %s", novels[id].Index, novels[id].Title, novels[id].URL))
		}
		resultList.Refresh()
		updateStatus("请从列表中选择小说")
	})

	var selectedNovel Novel
	crawlBtn := widget.NewButton("爬取", func() {
		if selectedNovel.URL == "" {
			updateStatus("请先选择一本小说")
			return
		}
		go func() {
			novelDir, err := createNovelDir(selectedNovel.Title)
			if err != nil {
				updateStatus(err.Error())
				return
			}
			updateStatus(fmt.Sprintf("正在处理小说: %s, URL: %s", selectedNovel.Title, selectedNovel.URL))
			statuses, err := crawlNovel(selectedNovel.URL, novelDir, updateStatus)
			if err != nil {
				updateStatus(fmt.Sprintf("爬取小说失败: %v", err))
				return
			}
			successCount := 0
			failureText := ""
			for _, s := range statuses {
				if s.Success {
					successCount++
				} else {
					failureText += fmt.Sprintf("失败章节: %d - %s\n", s.Seq, s.Title)
				}
			}
			updateStatus(fmt.Sprintf("下载完成：成功 %d 章，失败 %d 章\n%s", successCount, len(statuses)-successCount, failureText))
			updateStatus("爬取完成，请在当前路径/Novel/小说名 目录下选择按照文件名称排序")
		}()
	})

	exitBtn := widget.NewButton("退出", func() {
		a.Quit()
	})

	resultList.OnSelected = func(id widget.ListItemID) {
		novels, _ := searchNovels(input.Text)
		selectedNovel = novels[id]
		updateStatus(fmt.Sprintf("已选择: %s (%s)", selectedNovel.Title, selectedNovel.URL))
	}

	content := container.NewVBox(input, searchBtn, crawlBtn, exitBtn, resultScroll, statusScroll)
	w.SetContent(content)
	w.ShowAndRun()
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

	//fmt.Println("提取的章节列表:")
	//for _, ch := range chapters {
	//	fmt.Printf("%d: %s - %s\n", ch.Index, ch.Title, ch.URL)
	//}

	return chapters, nil
}

// crawlNovel 实现对选定小说的爬虫，同时根据小说规模决定线程数
func crawlNovel(url, novelDir string, updateStatus func(string)) ([]DownloadStatus, error) {
	chapters, err := crawlNovelChapters(url)
	if err != nil {
		return nil, fmt.Errorf("获取章节列表失败: %v", err)
	}
	numChapters := len(chapters)
	numWorkers := calculateWorkers(numChapters)
	var statuses []DownloadStatus
	successCount := 0

	if numWorkers == 1 {
		for _, chapter := range chapters {
			err := fetchChapterContent(novelDir, chapter, updateStatus)
			status := DownloadStatus{
				Seq:     chapter.Index,
				Title:   chapter.Title,
				Success: err == nil,
			}
			statuses = append(statuses, status)
			if err != nil {
				updateStatus(fmt.Sprintf("爬取章节'%s'失败: %v", chapter.Title, err))
			} else {
				successCount++
			}
		}
	} else {
		taskChan := make(chan Chapter, numChapters)
		resultChan := make(chan DownloadStatus, numChapters)
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go worker(taskChan, resultChan, novelDir, &wg, updateStatus)
		}

		go func() {
			for _, chapter := range chapters {
				taskChan <- chapter
			}
			close(taskChan)
		}()

		for i := 0; i < numChapters; i++ {
			status := <-resultChan
			statuses = append(statuses, status)
			if status.Success {
				successCount++
			}
		}

		wg.Wait()
		close(resultChan)
	}

	return statuses, nil
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

// fetchChapterContent 爬取单个章节的方法 每章保存一个文件
func fetchChapterContent(novelDir string, c Chapter, updateStatus func(string)) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", c.URL, nil)
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
	req.Header.Set("Referer", filepath.Dir(c.URL)+"/")
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
	// 由于windows存在不允许用在文件命名中的非法字符，在这里做转义处理。
	// 如果要测试部分章节下载失败的情况，可以注释这几行进行测试
	sanitizedTitle := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	).Replace(c.Title)
	filePath := filepath.Join(novelDir, fmt.Sprintf("%04d-%s.txt", c.Index, sanitizedTitle))
	err = ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("保存章节内容失败: %v", err)
	}
	updateStatus(fmt.Sprintf("已保存章节: %s", filePath))
	//没有测试请求过多会不会封ip
	time.Sleep(time.Second / 8)
	return nil
}

// GoGetNovel 提供给用户的对外接口，控制爬取小说的全部流程
//func GoGetNovel() {
//	scanner := bufio.NewScanner(os.Stdin)
//	for {
//		fmt.Print("请输入小说名（输入 'exit' 退出）: ")
//		scanner.Scan()
//		searchTerm := scanner.Text()
//		if searchTerm == "exit" {
//			fmt.Println("程序退出")
//			break
//		}
//		novels, err := searchNovels(searchTerm)
//		if err != nil {
//			fmt.Println("错误:", err)
//			continue
//		}
//		if len(novels) == 0 {
//			fmt.Println("搜索结果: 未找到任何匹配的小说")
//			continue
//		}
//		// 显示搜索结果
//		fmt.Println("\n搜索结果:")
//		for _, novel := range novels {
//			fmt.Printf("%d: 标题: %s, 链接: %s\n", novel.Index, novel.Title, novel.URL)
//		}
//		// 选择序号
//		fmt.Print("\n请输入要选择的小说序号（输入 0 取消）: ")
//		scanner.Scan()
//		input := scanner.Text()
//		var selectedIndex int
//		_, err = fmt.Sscanf(input, "%d", &selectedIndex)
//		if err != nil || selectedIndex < 0 || selectedIndex > len(novels) {
//			fmt.Println("无效的选择，已取消操作")
//			continue
//		}
//		if selectedIndex == 0 {
//			fmt.Println("已取消操作")
//			continue
//		}
//		// 确认操作
//		selectedNovel := novels[selectedIndex-1]
//		fmt.Printf("你选择了: %s (%s)，确认吗？(Y/N): ", selectedNovel.Title, selectedNovel.URL)
//		scanner.Scan()
//		confirm := strings.ToUpper(strings.TrimSpace(scanner.Text()))
//		if confirm != "Y" {
//			fmt.Println("已取消操作")
//			continue
//		}
//		novelDir, err := createNovelDir(selectedNovel.Title)
//		if err != nil {
//			fmt.Println(err)
//			continue
//		}
//		fmt.Printf("正在处理小说: %s, URL: %s\n", selectedNovel.Title, selectedNovel.URL)
//		err = crawlNovel(selectedNovel.URL, novelDir)
//		if err != nil {
//			fmt.Printf("爬取小说失败: %v\n", err)
//			continue
//		}
//		fmt.Println("爬取完成，请在当前路径/Novel/小说名 目录下选择按照文件名称排序")
//		fmt.Println("---------------------------------")
//	}
//}

// calculateWorkers 根据章节数量分配线程数量，最多指定了10个线程。这个分配算法是随便定的，没有太严谨的考量
func calculateWorkers(numChapters int) int {
	if numChapters <= 100 {
		return 1
	}
	workers := int(math.Ceil(float64(numChapters-100)/100)) + 1
	if workers > 10 {
		return 10
	}
	return workers
}

// worker 多线程爬取小说章节
func worker(taskChan chan Chapter, resultChan chan DownloadStatus, novelDir string, wg *sync.WaitGroup, updateStatus func(string)) {
	defer wg.Done()
	for chapter := range taskChan {
		err := fetchChapterContent(novelDir, chapter, updateStatus)
		resultChan <- DownloadStatus{Seq: chapter.Index, Title: chapter.Title, Success: err == nil}
	}
}
