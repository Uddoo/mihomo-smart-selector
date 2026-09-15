# 探测目标与判定规则

本清单对应 `web/public/connectivity-targets.json`，于 2026-09-15 逐项整理。所有请求均为 HTTPS GET，省略凭据和 Referer，并使用 `cache: no-store`。下表的预期响应是目标契约；`no-cors` 目标的状态与正文无法在当前浏览器面板中验证，不因声明了规则就显示验证通过。

## 选择依据与边界

- Google / YouTube 使用 `generate_204`，npm 使用 Registry 的 `/-/ping`；接口存在不代表它允许浏览器跨域读取。
- Cloudflare / ChatGPT 使用网络诊断入口，仅验证目标主机标识，不测试账号、模型推理或地区解锁。原始响应检查后即丢弃。
- GitHub 使用无需登录的只读 `/rate_limit`，检查状态与 JSON 键；公开接口仍可能限流，限流响应应显示异常。
- AI Studio 使用 Gemini API 的公开 Discovery 描述，仅检查 HTTP 200 与 JSON 内容类型；这是元数据入口，不能证明生成请求可用。其文档较大，收到头后取消正文下载。
- QQ 改为直接访问其图标 CDN，淘宝改为直接访问官方静态资源。京东使用不带随机参数的原始图标地址，避免此前带参数触发的错误跳转。
- 其余没有确认专用公开接口的服务保留资源探测。Takealot / Noon 的旧图标地址返回 404，改用网站入口并明确限制为可达性；入口可能返回风控页面、403 或超时，不能据此得出业务正常。
- `cors` 目标拒绝跳转，状态、内容类型或正文不符不会自动降级为通过；`no-cors` 按浏览器要求跟随跳转，每一跳仍受应用内置 HTTPS 域名清单约束。
- 本轮地址核对使用维护环境的 HTTP 请求；跨域头、重定向与公共服务行为可能变化。它不是所有用户网络或所有 48 个服务持续可用的保证。

## 48 个目标

| 服务 | 对象 | 地址 | 可读取校验 | 预期响应 | 跳转 | 防缓存参数 |
| --- | --- | --- | --- | --- | --- | --- |
| DeepSeek | 静态资源 | [deepseek](https://www.deepseek.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 抖音 | 静态资源 | [douyin](https://lf1-cdn-tos.bytegoofy.com/goofy/ies/douyin_web/public/favicon.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| 哔哩哔哩 | 静态资源 | [bilibili](https://www.bilibili.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 京东 | 静态资源 | [jd](https://www.jd.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 腾讯QQ | 静态资源 | [qq](https://mat1.gtimg.com/qqcdn/xw/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 微信 | 静态资源 | [wechat](https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| 小红书 | 静态资源 | [xiaohongshu](https://www.xiaohongshu.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 新浪微博 | 静态资源 | [weibo](https://weibo.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 百度 | 静态资源 | [baidu](https://www.baidu.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 网易 | 静态资源 | [netease](https://www.163.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 淘宝 | 静态资源 | [taobao](https://gw.alicdn.com/imgextra/i4/O1CN01qOI6vB1zaqrBKbyFr_!!6000000006731-73-tps-64-64.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| 小米 | 静态资源 | [xiaomi](https://www.mi.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Sony | 静态资源 | [sony](https://www.sony.jp/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| 任天堂 | 静态资源 | [nintendo](https://www.nintendo.co.jp/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Yahoo! JP | 静态资源 | [yahoo](https://www.yahoo.co.jp/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| LINE | 静态资源 | [line](https://line.me/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Apple | 静态资源 | [apple](https://www.apple.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Google | 专用连通性 | [google](https://www.google.com/generate_204) | 否（仅响应证据） | HTTP 204；空正文 | 跟随，限内置域名 | 是 |
| YouTube | 专用连通性 | [youtube](https://www.youtube.com/generate_204) | 否（仅响应证据） | HTTP 204；空正文 | 跟随，限内置域名 | 是 |
| GitHub | 公开只读 API | [github](https://api.github.com/rate_limit) | 是 | HTTP 200；JSON 对象，包含 resources, rate | 拒绝 | 否 |
| Cloudflare | 网络诊断 | [cloudflare](https://www.cloudflare.com/cdn-cgi/trace) | 是 | HTTP 200；包含 'h=www.cloudflare.com\n' | 拒绝 | 是 |
| Claude | 静态资源 | [claude](https://claude.ai/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| ChatGPT | 网络诊断 | [chatgpt](https://chatgpt.com/cdn-cgi/trace) | 是 | HTTP 200；包含 'h=chatgpt.com\n' | 拒绝 | 是 |
| AI Studio | 公开只读 API | [aistudio](https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta) | 是 | HTTP 200；不读正文；application/json | 拒绝 | 否 |
| Amazon | 静态资源 | [amazon](https://www.amazon.com/favicon.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| Bing | 静态资源 | [bing](https://www.bing.com/favicon.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| Steam | 静态资源 | [steam](https://store.steampowered.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Oracle | 静态资源 | [oracle](https://www.oracle.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Zoom | 静态资源 | [zoom](https://st1.zoom.us/favicon.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| Facebook | 静态资源 | [facebook](https://static.xx.fbcdn.net/rsrc.php/yb/r/hLRJ1GG_y0J.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Instagram | 静态资源 | [instagram](https://static.cdninstagram.com/rsrc.php/yb/r/hLRJ1GG_y0J.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| X | 静态资源 | [x](https://abs.twimg.com/favicons/twitter.3.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| Reddit | 静态资源 | [reddit](https://www.reddit.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| LinkedIn | 静态资源 | [linkedin](https://www.linkedin.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Twitch | 静态资源 | [twitch](https://static.twitchcdn.net/assets/favicon-32-e29e246c157142c94346.png) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Netflix | 静态资源 | [netflix](https://assets.nflxext.com/us/ffe/siteui/common/icons/nficon2016.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| TikTok | 静态资源 | [tiktok](https://www.tiktok.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Spotify | 静态资源 | [spotify](https://open.spotify.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| npm | 公开只读 API | [npm](https://registry.npmjs.org/-/ping) | 否（仅响应证据） | HTTP 200；JSON 对象 | 跟随，限内置域名 | 否 |
| Takealot | 网站入口 | [takealot](https://www.takealot.com/) | 否（仅响应证据） | HTTP 200；不读正文；text/html | 跟随，限内置域名 | 否 |
| PixPix | 静态资源 | [pixpix](https://www.pixpix.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Naver | 静态资源 | [naver](https://www.naver.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Noon | 网站入口 | [noon](https://www.noon.com/) | 否（仅响应证据） | HTTP 200；不读正文；text/html | 跟随，限内置域名 | 否 |
| Wikipedia | 静态资源 | [wikipedia](https://www.wikipedia.org/static/favicon/wikipedia.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| BBC | 静态资源 | [bbc](https://www.bbc.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Mistral AI | 静态资源 | [mistral](https://mistral.ai/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |
| Yandex | 静态资源 | [yandex](https://yastatic.net/favicon.ico) | 是 | HTTP 200；不读正文；image/ | 拒绝 | 否 |
| MercadoLibre | 静态资源 | [mercadolibre](https://www.mercadolibre.com/favicon.ico) | 否（仅响应证据） | HTTP 200；不读正文；image/ | 跟随，限内置域名 | 否 |

## 变更检查

变更目标时同时核对原始地址、重定向最终位置、预期状态、真实 Content-Type、正文规则和允许的跨域来源。不能仅凭命令行 HTTP 200 就将目标标为浏览器可校验。修改清单后运行前端单元测试、生产构建、CSP 测试及浏览器回归；固定图标 URL 还需在不拦截网络请求的浏览器中验收。服务端 CSP 自动从嵌入清单构建，不添加泛域名或不受约束的代理。
