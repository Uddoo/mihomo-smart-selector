package config

// defaultProbeProfiles intentionally contain only no-credential, read-only
// requests. A profile describes a service's reachable surface; it must never
// be presented as proof that a subscription, account, stream or purchase is
// usable.
func defaultProbeProfiles() []ProbeProfile {
	return []ProbeProfile{
		profile("internet-baseline", "基础 Internet 连通性", "通用 HTTPS 可达性与时延；不代表任何特定业务服务可用。", []string{"GLOBAL", "全球直连", "漏网之鱼"}, []Probe{
			probe("google-connectivity", "https://www.gstatic.com/generate_204", "204"),
		}),
		profile("chatgpt", "ChatGPT 服务可达性", "测量 ChatGPT Web 可达性与时延。严格 API 401 验证仅在专用探测选择器已配置时执行。", []string{"ChatGPT", "AI服务"}, []Probe{
			probe("chatgpt-trace", "https://chatgpt.com/cdn-cgi/trace", "200"),
		}, StrictProbe{Name: "openai-api-auth-boundary", URL: "https://api.openai.com/v1/models", ExpectedStatus: "401"}),
		profile("google", "Google 服务可达性", "测量 Google 静态连通性；不代表 Gmail、Gemini 或帐号功能可用。", []string{"谷歌服务"}, []Probe{
			probe("google-connectivity", "https://www.gstatic.com/generate_204", "204"),
		}),
		profile("youtube", "YouTube Web 可达性", "测量 YouTube Web 前端连通性；不代表具体视频、Premium 或地区版权可播放。", []string{"YouTube"}, []Probe{
			probe("youtube-connectivity", "https://www.youtube.com/generate_204", "204"),
		}),
		profile("google-fcm", "Google FCM API 可达性", "读取公开 API Discovery 文档；不会发送通知，也不需要 Google 凭据。", []string{"谷歌FCM"}, []Probe{
			probe("fcm-discovery", "https://fcm.googleapis.com/$discovery/rest?version=v1", "200"),
		}),
		profile("github", "GitHub API 可达性", "读取 GitHub 限流状态；建议仅用于低频、缓存后的稳定性扫描。", []string{"GitHub"}, []Probe{
			probe("github-rate-limit", "https://api.github.com/rate_limit", "200"),
		}),
		profile("discord", "Discord Gateway 可达性", "读取无需登录的 Gateway 地址；不代表帐号登录、消息收发或语音可用。", []string{"社交媒体", "即时通讯"}, []Probe{
			probe("discord-gateway", "https://discord.com/api/v10/gateway", "200"),
		}),
		profile("apple-services", "Apple 服务可达性", "测量 App Store Web 前端可达性；不代表 iCloud、推送或帐号状态。", []string{"苹果服务"}, []Probe{
			probe("apple-app-store", "https://apps.apple.com/", "200"),
		}),
		profile("apple-tv", "Apple TV+ Web 可达性", "测量 Apple TV Web 前端可达性；不代表订阅、登入或节目播放。", []string{"AppleTV+"}, []Probe{
			probe("apple-tv-web", "https://tv.apple.com/", "200"),
		}),
		profile("netflix", "Netflix Web 可达性", "测量 Netflix Web 入口可达性；不将首页可达误判为完整解锁。", []string{"Netflix"}, []Probe{
			probe("netflix-web", "https://www.netflix.com/", "200"),
		}),
		profile("disney-plus", "Disney+ Web 可达性", "测量 Disney+ Web 入口可达性；地区页、验证码或 403 会作为服务受限信号，而非成功。", []string{"DisneyPlus"}, []Probe{
			probe("disney-plus-web", "https://www.disneyplus.com/", "200"),
		}),
		profile("max", "Max Web 可达性", "测量 Max（原 HBO Max）Web 入口可达性；不代表片库或节目播放验证。", []string{"HBO"}, []Probe{
			probe("max-web", "https://www.max.com/", "200"),
		}),
		profile("prime-video", "Prime Video Web 可达性", "测量 Prime Video Web 入口可达性；实际地区应按你所用 Amazon 站点另外配置。", []string{"PrimeVideo"}, []Probe{
			probe("prime-video-web", "https://www.primevideo.com/", "200"),
		}),
		profile("steam", "Steam 商店可达性", "测量 Steam Store Web 入口可达性；不代表游戏下载、联机或游戏服务器 UDP 质量。", []string{"Steam"}, []Probe{
			probe("steam-store", "https://store.steampowered.com/", "200"),
		}),
		profile("spotify", "Spotify 服务可达性", "Web 前端用于可达性；API 401 严格验证只在专用探测选择器已配置时执行。", []string{"Spotify"}, []Probe{
			probe("spotify-web", "https://open.spotify.com/", "200"),
		}, StrictProbe{Name: "spotify-api-auth-boundary", URL: "https://api.spotify.com/v1/me", ExpectedStatus: "401"}),
		profile("tiktok", "TikTok Web 可达性", "TikTok 可能因风控或验证码返回限制页面；结果不等同于内容浏览、上传或地区解锁。", []string{"TikTok"}, []Probe{
			probe("tiktok-web", "https://www.tiktok.com/", "200"),
		}),
		profile("bahamut", "Bahamut 动画疯可达性", "测量动画疯 Web 前端可达性；不代表特定动画、帐号或串流 CDN 已验证。", []string{"Bahamut"}, []Probe{
			probe("bahamut-anime", "https://ani.gamer.com.tw/", "200"),
		}),
		profile("microsoft", "Microsoft 连通性", "使用 Microsoft Connect Test 可达性；严格正文断言只在专用探测选择器已配置时执行。", []string{"微软服务"}, []Probe{
			probe("microsoft-connect-test", "https://www.msftconnecttest.com/connecttest.txt", "200"),
		}, StrictProbe{Name: "microsoft-connect-content", URL: "https://www.msftconnecttest.com/connecttest.txt", ExpectedStatus: "200", BodyContains: "Microsoft Connect Test"}),
		profile("ecommerce-jp", "海外电商（日本）可达性", "测量 Amazon 日本站入口。若你使用其他国家站点，请在路由器配置中覆写此 Profile。", []string{"国外电商"}, []Probe{
			probe("amazon-japan", "https://www.amazon.co.jp/", "200"),
		}),
		{
			ID:                    "emby",
			Label:                 "Emby 私有服务",
			Description:           "必须指向你自己的 Emby 实例，公共站点不能代表你的媒体库。",
			GroupNames:            []string{"Emby"},
			TransportScope:        "HTTP latency only",
			RequiresConfiguration: true,
			SetupHint:             "在 router config 中覆写 emby Profile，并使用 https://<你的-Emby-域名>/emby/System/Info/Public 的 200 探测。",
		},
	}
}

func profile(id, label, description string, groups []string, probes []Probe, strict ...StrictProbe) ProbeProfile {
	return ProbeProfile{
		ID:                    id,
		Label:                 label,
		Description:           description,
		ExposeTargetAddresses: true,
		GroupNames:            groups,
		Probes:                probes,
		StrictProbes:          strict,
		TransportScope:        "HTTP latency only",
	}
}

func probe(name, endpoint, expectedStatus string) Probe {
	return Probe{Name: name, URL: endpoint, ExpectedStatus: expectedStatus}
}
