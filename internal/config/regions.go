package config

import "strings"

// BuiltinRegions is a name dictionary, not an IP geolocation database. Return
// fresh slices so configuration merging cannot mutate defaults across instances.
func BuiltinRegions() []Region {
	return []Region{
		region("JP", "日本", "Japan", "JPN", "Tokyo", "东京", "東京", "Osaka", "大阪"),
		region("US", "美国", "United States", "USA", "美國", "Los Angeles", "洛杉矶", "洛杉磯", "Seattle", "西雅图", "New York", "纽约", "紐約", "San Jose", "圣何塞"),
		region("KR", "韩国", "South Korea", "KOR", "Korea", "Korean", "韓國", "Seoul", "首尔", "首爾", "Busan", "釜山"),
		region("HK", "香港", "Hong Kong", "HKG", "HongKong"),
		region("TW", "台湾", "Taiwan", "TWN", "台灣", "Taipei", "台北", "臺北"),
		region("SG", "新加坡", "Singapore", "SGP", "狮城", "獅城"),
		region("AE", "阿联酋", "United Arab Emirates", "ARE", "阿聯酋", "Dubai", "迪拜", "杜拜", "Abu Dhabi", "阿布扎比"),
		region("AU", "澳大利亚", "Australia", "AUS", "澳大利亞", "澳洲", "Sydney", "悉尼", "雪梨", "Melbourne", "墨尔本", "墨爾本"),
		region("BR", "巴西", "Brazil", "BRA", "Sao Paulo", "São Paulo", "圣保罗", "聖保羅"),
		region("CA", "加拿大", "Canada", "CAN", "Toronto", "多伦多", "多倫多", "Vancouver", "温哥华", "溫哥華"),
		region("CL", "智利", "Chile", "CHL", "Santiago", "圣地亚哥", "聖地亞哥"),
		region("DE", "德国", "Germany", "DEU", "德國", "Frankfurt", "法兰克福", "法蘭克福", "Berlin", "柏林"),
		region("FR", "法国", "France", "FRA", "法國", "Paris", "巴黎"),
		region("GB", "英国", "United Kingdom", "GBR", "UK", "英國", "Britain", "London", "伦敦", "倫敦"),
		region("ID", "印度尼西亚", "Indonesia", "IDN", "印度尼西亞", "印尼", "Jakarta", "雅加达", "雅加達"),
		region("IN", "印度", "India", "IND", "Mumbai", "孟买", "孟買", "New Delhi", "新德里"),
		region("IT", "意大利", "Italy", "ITA", "義大利", "Milan", "米兰", "米蘭", "Rome", "罗马", "羅馬"),
		region("MX", "墨西哥", "Mexico", "MEX"),
		region("MY", "马来西亚", "Malaysia", "MYS", "馬來西亞", "Kuala Lumpur", "吉隆坡"),
		region("NL", "荷兰", "Netherlands", "NLD", "荷蘭", "Amsterdam", "阿姆斯特丹"),
		region("PH", "菲律宾", "Philippines", "PHL", "菲律賓", "Manila", "马尼拉", "馬尼拉"),
		region("RU", "俄罗斯", "Russia", "RUS", "俄羅斯", "Moscow", "莫斯科"),
		region("TH", "泰国", "Thailand", "THA", "泰國", "Bangkok", "曼谷"),
		region("VN", "越南", "Vietnam", "VNM", "Ho Chi Minh", "胡志明", "Hanoi", "河内", "河內"),
		region("MO", "澳门", "Macao", "MAC", "澳門", "Macau"),
		region("CN", "中国大陆", "Mainland China", "CHN", "中国", "中國", "China", "Beijing", "北京", "Shanghai", "上海"),
		region("NZ", "新西兰", "New Zealand", "NZL", "新西蘭", "紐西蘭", "Auckland", "奥克兰", "奧克蘭"),
		region("CH", "瑞士", "Switzerland", "CHE", "Zurich", "苏黎世", "蘇黎世"),
		region("SE", "瑞典", "Sweden", "SWE", "Stockholm", "斯德哥尔摩", "斯德哥爾摩"),
		region("NO", "挪威", "Norway", "NOR", "Oslo", "奥斯陆", "奧斯陸"),
		region("FI", "芬兰", "Finland", "FIN", "芬蘭", "Helsinki", "赫尔辛基", "赫爾辛基"),
		region("DK", "丹麦", "Denmark", "DNK", "丹麥", "Copenhagen", "哥本哈根"),
		region("IE", "爱尔兰", "Ireland", "IRL", "愛爾蘭", "Dublin", "都柏林"),
		region("ES", "西班牙", "Spain", "ESP", "Madrid", "马德里", "馬德里"),
		region("PT", "葡萄牙", "Portugal", "PRT", "Lisbon", "里斯本"),
		region("AT", "奥地利", "Austria", "AUT", "奧地利", "Vienna", "维也纳", "維也納"),
		region("BE", "比利时", "Belgium", "BEL", "比利時", "Brussels", "布鲁塞尔", "布魯塞爾"),
		region("PL", "波兰", "Poland", "POL", "波蘭", "Warsaw", "华沙", "華沙"),
		region("TR", "土耳其", "Turkey", "TUR", "Türkiye", "Istanbul", "伊斯坦布尔", "伊斯坦堡"),
		region("ZA", "南非", "South Africa", "ZAF", "Johannesburg", "约翰内斯堡", "約翰內斯堡"),
	}
}

func region(code, name, english string, aliases ...string) Region {
	runes := []rune(code)
	emoji := string([]rune{runes[0] - 'A' + 0x1f1e6, runes[1] - 'A' + 0x1f1e6})
	return Region{Code: code, Name: name, Emoji: emoji, Aliases: append([]string{code, name, english, emoji}, aliases...)}
}

// MergeRegions extends the built-ins; a legacy three-region YAML is not a
// whitelist. Custom names/emoji override metadata while aliases are additive.
// Validate rejects duplicate custom entries before Load materializes the merge.
func MergeRegions(custom []Region) []Region {
	result := BuiltinRegions()
	positions := map[string]int{}
	for i, entry := range result {
		positions[entry.Code] = i
	}
	for _, entry := range custom {
		entry.Code = strings.ToUpper(strings.TrimSpace(entry.Code))
		index, exists := positions[entry.Code]
		if !exists {
			index = len(result)
			positions[entry.Code] = index
			result = append(result, Region{Code: entry.Code})
		}
		target := &result[index]
		if strings.TrimSpace(entry.Name) != "" {
			target.Name = strings.TrimSpace(entry.Name)
		}
		if entry.Emoji != "" {
			target.Emoji = entry.Emoji
		}
		seen := map[string]bool{}
		for _, alias := range target.Aliases {
			seen[strings.ToLower(alias)] = true
		}
		for _, alias := range entry.Aliases {
			alias = strings.TrimSpace(alias)
			key := strings.ToLower(alias)
			if alias != "" && !seen[key] {
				target.Aliases = append(target.Aliases, alias)
				seen[key] = true
			}
		}
	}
	return result
}
