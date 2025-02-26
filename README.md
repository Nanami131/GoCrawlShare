# Golang 爬虫实践

简单的爬虫小项目。体验golang高并发爬虫的优势，顺便提供一点便捷的功能？

笔者最近在尝试转Go，与君共勉。

## 一、笔趣阁爬取小说

### 失败尝试

一开始想在程序里提供一个搜索小说功能，一步到位。但是爬取时遇到了困难。

经过权衡后决定先换一个相对好爬的网页平替，先实现功能。等技术提升以后再回来尝试

爬取笔趣阁（`https://www.bi01.cc`）的小说搜索结果时，捕获了三个关键请求：`https://www.bi01.cc/s?q=235`、`https://www.bi01.cc/user/search.html?q=235&so=undefined` 和 `https://www.bi01.cc/user/hm.html?q=235`。

这三个请求均返回状态码 200 OK，但最终未能成功获取预期的小说信息 JSON 数据。这里以235作为query参数为例，

`https://www.bi01.cc/s?q=235` 是一个直接的 GET 请求，带有 `Accept: text/html` 和 `sec-fetch-dest: document`，表明这是浏览器加载搜索页面的初始请求，响应头中没有 `set-cookie`，但请求携带了现有的 `Cookie`（如 `hm=a41419f17cc1f37068e55ed5c8dbd172`），这也是直接进入网页看到的最直接的请求。

`https://www.bi01.cc/user/hm.html?q=235` 紧接着出现，时间戳比初始请求晚 2 秒。这个请求带有 `X-Requested-With: XMLHttpRequest` 和 `Accept: application/json`，表明它是一个 Ajax 请求，且响应中包含 `set-cookie: hm=ef9a33239e741aba9a58238603b0baad`，更新了 `hm` 值。

`https://www.bi01.cc/user/search.html?q=235&so=undefined` 发起的时间戳更晚，同样是 Ajax 请求，携带更新后的 `Cookie`（`hm=ef9a33239e741aba9a58238603b0baad`），并预期返回搜索结果的 JSON 数据。

最终排除缓存的情况，爬虫程序爬取第三个请求的结果是“1”，状态码为200，推测是爬虫被网站识别。

推测的逻辑如下：

初始页面请求触发了会话验证（`hm.html`），生成特定于搜索词的 `Cookie`，然后 `search.html` 使用该 `Cookie` 获取结果。 Ajax 的处理逻辑可以从请求头和响应推导出来。初始请求 `/s?q=235` 加载搜索页面后，页面内的 JavaScript（可能类似 `$.getJSON`）会异步调用 `/user/hm.html?q=235`，这是一个前置请求，用于验证或初始化会话。`hm.html` 返回的响应（可能是简单的 JSON 或文本）不重要，但其 `set-cookie` 更新了 `hm` 值，可能包含与查询词 “235” 绑定的校验信息。随后，脚本发起 `/user/search.html?q=235&so=undefined`，携带新 `Cookie` 和 `Referer: https://www.bi01.cc/s?q=235`，服务器验证 `Cookie` 和 `Referer` 的一致性后返回 JSON 数据。

推测网站可能通过动态 `Cookie` 和来源检查防止直接请求，确保只有页面触发的调用有效。 爬取失败的原因可能与未能正确模拟这一流程有关。程序最初尝试直接请求 `/user/search.html?q=235`，使用静态 `Cookie` 或无 `Cookie`，但服务器返回 “1”，表明请求无效。后来发现使用浏览器抓取的特定 `Cookie`（如搜索 “235” 时的 `hm=ef9a33239e741aba9a58238603b0baad`）只能成功获取 “235” 的结果，其他输入（如 “1” 或 “斗破苍穹”）仍返回 “1”。这提示 `Cookie` 的 `hm` 值与搜索词绑定，可能是服务器生成的一个校验码。进一步尝试通过预请求 `/user/search.html` 获取动态 `Cookie`，再发起正式请求，但仍失败。

推测原因包括： Ajax 请求缺少上下文；服务器可能有额外的防爬机制（如检查请求时间间隔、IP、或未捕获的头信息），拒绝非浏览器环境的请求。

未能完整复现浏览器 Ajax 的两步逻辑（验证 + 获取）和动态 `Cookie` 的生成过程，导致爬取失败。
