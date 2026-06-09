package link

import "testing"

func TestExtractProviderCorpus(t *testing.T) {
	extractor := NewExtractor()
	cases := []struct {
		name     string
		text     string
		wantType string
		wantURL  string
		wantPass string
	}{
		{"115", "https://115.com/s/abc-123?password=a1B2", "115", "https://115.com/s/abc-123?password=a1B2", "a1B2"},
		{"115 http", "http://115.com/s/abc123?password=a1B2#", "115", "http://115.com/s/abc123?password=a1B2", "a1B2"},
		{"115cdn", "https://115cdn.com/s/share_1", "115", "https://115cdn.com/s/share_1", ""},
		{"anxia", "https://anxia.com/s/share-2 密码: z9", "115", "https://anxia.com/s/share-2", "z9"},
		{"xunlei", "https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd#", "xunlei", "https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd", "kewd"},
		{"xunlei http", "http://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd#", "xunlei", "http://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd", "kewd"},
		{"baidu share", "https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub", "baidu", "https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub", "ruub"},
		{"baidu http", "http://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub", "baidu", "http://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub", "ruub"},
		{"baidu init", "https://pan.baidu.com/share/init?surl=abc-123&pwd=7788", "baidu", "https://pan.baidu.com/share/init?surl=abc-123&pwd=7788", "7788"},
		{"pikpak", "https://mypikpak.com/s/Vabc123?pwd=p9", "pikpak", "https://mypikpak.com/s/Vabc123?pwd=p9", "p9"},
		{"tianyi web", "https://cloud.189.cn/web/share?code=AbCd", "tianyi", "https://cloud.189.cn/web/share?code=AbCd", ""},
		{"tianyi encoded t", "https://cloud.189.cn/t/AbCd%E8%AE%BF%E9%97%AE", "tianyi", "https://cloud.189.cn/t/AbCd%E8%AE%BF%E9%97%AE", ""},
		{"tianyi t code", "https://cloud.189.cn/t/AbCd（访问码：7x9q）", "tianyi", "https://cloud.189.cn/t/AbCd", "7x9q"},
		{"tianyi encoded code", "https://cloud.189.cn/t/AbCd%EF%BC%88%E8%AE%BF%E9%97%AE%E7%A0%81%EF%BC%9A7x9q%EF%BC%89", "tianyi", "https://cloud.189.cn/t/AbCd", "7x9q"},
		{"tianyi h5", "https://h5.cloud.189.cn/share.html#/t/AbCd", "tianyi", "https://h5.cloud.189.cn/share.html#/t/AbCd", ""},
		{"mobile caiyun m", "https://caiyun.139.com/m/i?abc123", "mobile", "https://caiyun.139.com/m/i?abc123", ""},
		{"mobile caiyun m www", "https://www.caiyun.139.com/m/i?abc123&foo=bar", "mobile", "https://www.caiyun.139.com/m/i?abc123&foo=bar", ""},
		{"mobile caiyun adjacent label", "https://www.caiyun.139.com/m/i?abc123&foo=bar标签：短剧", "mobile", "https://www.caiyun.139.com/m/i?abc123&foo=bar", ""},
		{"mobile yun shareweb", "https://yun.139.com/shareweb/#/w/i/abc123", "mobile", "https://yun.139.com/shareweb/#/w/i/abc123", ""},
		{"mobile yun shareweb www", "https://www.yun.139.com/shareweb/#/w/i/abc123", "mobile", "https://www.yun.139.com/shareweb/#/w/i/abc123", ""},
		{"mobile caiyun w", "https://caiyun.139.com/w/i/abc123", "mobile", "https://caiyun.139.com/w/i/abc123", ""},
		{"mobile feixin", "https://caiyun.feixin.10086.cn/abc123", "mobile", "https://caiyun.feixin.10086.cn/abc123", ""},
		{"quark", "https://pan.quark.cn/s/8a16ab9c06b9", "quark", "https://pan.quark.cn/s/8a16ab9c06b9", ""},
		{"quark http", "http://pan.quark.cn/s/8a16ab9c06b9", "quark", "http://pan.quark.cn/s/8a16ab9c06b9", ""},
		{"uc password", "https://drive.uc.cn/s/d5eaad53?password=xy9z", "uc", "https://drive.uc.cn/s/d5eaad53?password=xy9z", "xy9z"},
		{"uc public", "https://drive.uc.cn/s/d5eaad53da684?public=1", "uc", "https://drive.uc.cn/s/d5eaad53da684?public=1", ""},
		{"uc adjacent password", "https://drive.uc.cn/s/d5eaad53da684?public=1提取码:xy9z", "uc", "https://drive.uc.cn/s/d5eaad53da684?public=1", "xy9z"},
		{"uc fast", "https://fast.uc.cn/s/abc123", "uc", "https://fast.uc.cn/s/abc123", ""},
		{"aliyun folder", "https://www.aliyundrive.com/s/abc123/folder/folder456?password=qwer", "aliyun", "https://www.aliyundrive.com/s/abc123/folder/folder456?password=qwer", "qwer"},
		{"alipan", "https://www.alipan.com/s/MHf34XusdVK", "aliyun", "https://www.alipan.com/s/MHf34XusdVK", ""},
		{"alipan no www", "https://alipan.com/s/MHf34XusdVK", "aliyun", "https://alipan.com/s/MHf34XusdVK", ""},
		{"123 inline", "https://123pan.com/s/abc123提取码:9a8b", "123", "https://123pan.com/s/abc123", "9a8b"},
		{"123 html", "https://www.123pan.com/s/abc123.html?提取码:9a8b", "123", "https://www.123pan.com/s/abc123.html", "9a8b"},
		{"123 numeric com", "https://123865.com/s/abc_123", "123", "https://123865.com/s/abc_123", ""},
		{"123 pan cn", "https://www.123pan.cn/s/abc-123?提取码:9a8b", "123", "https://www.123pan.cn/s/abc-123", "9a8b"},
		{"123 share pan cn", "https://1850896530.share.123pan.cn/123pan/tSkpvd-K1Ggh?pwd=Zlwl", "123", "https://1850896530.share.123pan.cn/123pan/tSkpvd-K1Ggh?pwd=Zlwl", "Zlwl"},
		{"guangya", "https://www.guangyapan.com/s/ABC_123", "guangya", "https://www.guangyapan.com/s/ABC_123", ""},
		{"magnet", "magnet:?xt=urn:btih:abcdef", "magnet", "magnet:?xt=urn:btih:abcdef", ""},
		{"ed2k", "ed2k://|file|movie.mkv|123|HASH|/", "ed2k", "ed2k://|file|movie.mkv|123|HASH|/", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			links := extractor.Extract(tc.text)
			if len(links) != 1 {
				t.Fatalf("len = %d, want 1: %+v", len(links), links)
			}
			if links[0].Type != tc.wantType || links[0].URL != tc.wantURL || links[0].Password != tc.wantPass {
				t.Fatalf("link = %+v, want type=%s url=%s password=%s", links[0], tc.wantType, tc.wantURL, tc.wantPass)
			}
		})
	}
}

func TestExtractRealMessageCorpus(t *testing.T) {
	text := `海报
名称：2026年6月6日 短剧更新目录12

链接：
🔗 夸克网盘：https://pan.quark.cn/s/8a16ab9c06b9
🔗 百度网盘：https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub
🔑 提取码：ruub
🔗 UC 网盘：https://drive.uc.cn/s/d5eaad53da684?public=1
🔗 迅雷云盘：https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd#
🔑 提取码：kewd
🔗 阿里云盘：https://www.alipan.com/s/MHf34XusdVK

🏷 标签：#短剧 #最新短剧 #合集
📢 频道：https://t.me/+Djia5z2lVsI5ODRl
👥 群组：@Quark_Share_Group (https://t.me/Quark_Share_Group)
🤖 投稿：@QuarkRobot (https://t.me/QuarkRobot)`

	links := NewExtractor().Extract(text)
	byType := map[string][]string{}
	for _, item := range links {
		byType[item.Type] = append(byType[item.Type], item.URL)
	}
	want := map[string]string{
		"quark":  "https://pan.quark.cn/s/8a16ab9c06b9",
		"baidu":  "https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub",
		"uc":     "https://drive.uc.cn/s/d5eaad53da684?public=1",
		"xunlei": "https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd",
		"aliyun": "https://www.alipan.com/s/MHf34XusdVK",
	}
	for typ, url := range want {
		if !contains(byType[typ], url) {
			t.Fatalf("missing %s %s in links %+v", typ, url, links)
		}
	}
	for _, typ := range []string{"quark", "baidu", "uc", "xunlei", "aliyun"} {
		if len(byType[typ]) != 1 {
			t.Fatalf("type %s count = %d, want 1: %+v", typ, len(byType[typ]), links)
		}
	}
	if len(byType["url"]) != 0 {
		t.Fatalf("fallback url count = %d, want telegram links ignored: %+v", len(byType["url"]), byType["url"])
	}
}

func TestExtractIgnoresTelegramLinks(t *testing.T) {
	text := `频道：https://t.me/+Djia5z2lVsI5ODRl
群组：http://t.me/Quark_Share_Group
投稿：https://T.ME/QuarkRobot
资源：https://pan.quark.cn/s/abc123
官网：https://example.com/post`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want only non-telegram links: %+v", len(links), links)
	}
	if links[0].Type != "quark" || links[0].URL != "https://pan.quark.cn/s/abc123" {
		t.Fatalf("first link = %+v, want quark link", links[0])
	}
	if links[1].Type != "url" || links[1].URL != "https://example.com/post" {
		t.Fatalf("second link = %+v, want fallback url", links[1])
	}
}

func TestExtractDeduplicatesProviderAndFallback(t *testing.T) {
	links := NewExtractor().Extract("https://pan.quark.cn/s/abc123 https://pan.quark.cn/s/abc123")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	if links[0].Type != "quark" {
		t.Fatalf("type = %q, want quark", links[0].Type)
	}
}

func TestExtractAssignsNoteFromTitleBeforeLink(t *testing.T) {
	text := `庆余年 S02 4K 全集
夸克网盘：https://pan.quark.cn/s/abc123

凡人修仙传 最新
阿里云盘：https://www.alipan.com/s/def456`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(links), links)
	}
	if links[0].URL != "https://pan.quark.cn/s/abc123" || links[0].Note != "庆余年 S02 4K 全集" {
		t.Fatalf("first link = %+v, want note from preceding title", links[0])
	}
	if links[1].URL != "https://www.alipan.com/s/def456" || links[1].Note != "凡人修仙传 最新" {
		t.Fatalf("second link = %+v, want note from preceding title", links[1])
	}
}

func TestExtractAssignsNoteAcrossLinkLabelLine(t *testing.T) {
	text := `庆余年 S02 4K
链接：
https://pan.quark.cn/s/abc123`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	if links[0].Note != "庆余年 S02 4K" {
		t.Fatalf("note = %q, want title above link label line", links[0].Note)
	}
}

func TestExtractLeavesNoteEmptyForProviderOnlyLabels(t *testing.T) {
	links := NewExtractor().Extract("夸克网盘：https://pan.quark.cn/s/abc123")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	if links[0].Note != "" {
		t.Fatalf("note = %q, want empty provider label is not a title", links[0].Note)
	}
}

func TestExtractFallbackURLAndFalsePositive(t *testing.T) {
	links := NewExtractor().Extract("官网 https://example.com/a 不是网盘 pan.baidu.com/s/no-scheme")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	if links[0].Type != "url" || links[0].URL != "https://example.com/a" {
		t.Fatalf("link = %+v, want fallback url", links[0])
	}
}

func TestExtractDoesNotClassifyUnknown123Domain(t *testing.T) {
	links := NewExtractor().Extract("官网 https://123abc.com/s/not-pan")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1 fallback url: %+v", len(links), links)
	}
	if links[0].Type != "url" {
		t.Fatalf("type = %q, want fallback url", links[0].Type)
	}
}

func TestExtractResourceFields(t *testing.T) {
	text := `资源合集
夸克：https://pan.quark.cn/s/abc123
磁力：magnet:?xt=urn:btih:abcdef
电驴：ed2k://|file|movie.mkv|123|HASH|/
官网：https://example.com/post`

	links := NewExtractor().Extract(text)
	byURL := map[string]struct {
		category string
		snippet  string
	}{}
	for _, item := range links {
		byURL[item.URL] = struct {
			category string
			snippet  string
		}{category: item.Category, snippet: item.SourceSnippet}
	}
	want := map[string]string{
		"https://pan.quark.cn/s/abc123":     "cloud_drive",
		"magnet:?xt=urn:btih:abcdef":        "magnet",
		"ed2k://|file|movie.mkv|123|HASH|/": "ed2k",
		"https://example.com/post":          "http",
	}
	for url, category := range want {
		got, ok := byURL[url]
		if !ok {
			t.Fatalf("missing url %s in %+v", url, links)
		}
		if got.category != category {
			t.Fatalf("category for %s = %q, want %q", url, got.category, category)
		}
		if got.snippet == "" {
			t.Fatalf("source snippet for %s is empty", url)
		}
	}
}

func TestExtractMediaMessageEd2KWithSpaces(t *testing.T) {
	text := `📺 电视剧：斗破苍穹 (2017) - S05E202
🍿 TMDB ID: 79481
⭐️ 评分: 8.2
🎭 类型: 动画,动作冒险,Sci-Fi & Fantasy
📂 分类: 国漫
🎞️ 质量: WEB-DL 2160p
📦 文件: 1 个
💾 大小: 854.19 MB
👥 主演: 刘三木,刘雨轩,万苏婉,鬼月,陈奕雯
📝 简介: 萧炎曾是家族里公认的斗气天才，年仅11岁便已经抵达了常人穷尽一生都无法修炼到的境界。可12岁那年，一场意外让萧炎的全部努力都化为了乌有，失去一切的他体会到了人情的冷暖和世态的炎凉，之后，萧炎和纳兰嫣然...

🔗 链接: 
ed2k://|file|斗破苍穹.2017 - S05E202 - 第 202 集 - 2160p.WEB-DL.SDR.HEVC.AAC 2.0.{tmdb-79481}.mp4|895680618|24E12F6E5868DC08F432B28CDA67172B|/
#国漫`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	wantURL := "ed2k://|file|斗破苍穹.2017 - S05E202 - 第 202 集 - 2160p.WEB-DL.SDR.HEVC.AAC 2.0.{tmdb-79481}.mp4|895680618|24E12F6E5868DC08F432B28CDA67172B|/"
	if links[0].Type != "ed2k" || links[0].URL != wantURL {
		t.Fatalf("link = %+v, want ed2k url %s", links[0], wantURL)
	}
	if links[0].Note != "斗破苍穹 (2017) - S05E202" {
		t.Fatalf("note = %q, want media title", links[0].Note)
	}
}

func TestExtractAssignsMediaMetadataToCloudDriveLinks(t *testing.T) {
	text := `名称：开始推理吧 第四季 (2026)  刘宇宁 金靖 张凌赫 程鑫  综艺  真人秀真人秀  0607期

描述：又名: 开始推理吧 4

链接：
🔗 百度网盘：https://pan.baidu.com/s/1sQSU-e5CoYds6MeFEymS1A?pwd=3345
🔑 提取码：3345
🏷 标签：#刘宇宁 #金靖 #真人秀 #开始推理吧`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	link := links[0]
	if link.MediaTitle != "开始推理吧 第四季" {
		t.Fatalf("media title = %q, want 开始推理吧 第四季", link.MediaTitle)
	}
	if link.MediaYear != "2026" || link.MediaEpisode != "0607期" || link.MediaCategory != "综艺" {
		t.Fatalf("metadata = %+v, want year 2026 episode 0607期 category 综艺", link)
	}
	if link.MediaTags != "刘宇宁 金靖 真人秀 开始推理吧" {
		t.Fatalf("tags = %q", link.MediaTags)
	}
}

func TestExtractMediaMetadataSkipsNearbyNoiseBeforeLink(t *testing.T) {
	text := `憨婿
4K S01E01 - E25 HiveWeb

简介：理工高材生韦浩意外魂穿大庸朝
分享：Pluto (https://hdhive.com/user/17888)
大小：2.6GB
链接：直达链接 (https://pan.baidu.com/s/1yHyPAA47gToHykEQfrn3Pw?pwd=t67z)
标签：#憨婿 #剧情`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want hdhive fallback and baidu link: %+v", len(links), links)
	}
	link := links[1]
	if link.Type != "baidu" || link.MediaTitle != "憨婿" || link.Note != "憨婿" {
		t.Fatalf("baidu link = %+v, want media title 憨婿", link)
	}
	if link.MediaSeason != "S01" || link.MediaEpisode != "E01" || link.MediaQuality != "4K" || link.MediaSize != "2.6GB" {
		t.Fatalf("metadata = %+v, want season/episode/quality/size", link)
	}
}

func TestExtractMediaMetadataFromStructuredTVMessage(t *testing.T) {
	text := `📺 电视剧：塬上风云 (2026) - S01E30
🍿 TMDB ID: 323346
🎭 类型: 剧情,War & Politics
📂 分类: 国产剧
🎞️ 质量: WEB-DL 2160p HDR10
💾 大小: 3.23 GB

🔗 链接: https://115cdn.com/s/swsznow33xj?password=q474`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	link := links[0]
	if link.MediaTitle != "塬上风云" || link.MediaYear != "2026" || link.MediaSeason != "S01" || link.MediaEpisode != "E30" {
		t.Fatalf("title/sequence metadata = %+v", link)
	}
	if link.MediaTMDBID != "323346" || link.MediaCategory != "国产剧" || link.MediaQuality != "WEB-DL 2160p HDR10" || link.MediaSize != "3.23 GB" {
		t.Fatalf("structured metadata = %+v", link)
	}
}

func TestExtractMediaMetadataFromShortDramaTitle(t *testing.T) {
	text := `短剧-完了，这破农场来的全是祖宗第二季（80集）

夸克：https://pan.quark.cn/s/8fd1235933b5
度盘：https://pan.baidu.com/s/1idK69_EZ6stsf5ra1qPwjQ?pwd=3l6e`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(links), links)
	}
	for _, link := range links {
		if link.MediaTitle != "完了，这破农场来的全是祖宗第二季" || link.MediaEpisode != "80集" || link.MediaCategory != "短剧" {
			t.Fatalf("link = %+v, want short drama metadata", link)
		}
	}
}

func TestExtractMediaMetadataFromMagnetAndED2KURL(t *testing.T) {
	text := `资源
magnet:?xt=urn:btih:abcdef&dn=维京传奇.2013.S01.2160p.WEB-DL.mkv
ed2k://|file|刀.1995.USA.UHD.Blu-ray.2160p.DV.HDR.mkv|94070593331|ABCDEF0123456789|/`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(links), links)
	}
	if links[0].MediaTitle != "维京传奇.2013.S01.2160p.WEB-DL" || links[0].MediaYear != "2013" || links[0].MediaSeason != "S01" {
		t.Fatalf("magnet metadata = %+v", links[0])
	}
	if links[1].MediaTitle != "刀.1995.USA.UHD.Blu-ray.2160p.DV.HDR" || links[1].MediaYear != "1995" || links[1].MediaQuality == "" || links[1].MediaSize == "" {
		t.Fatalf("ed2k metadata = %+v", links[1])
	}
}

func TestExtractMediaMetadataFromResourceNameAndPikPak(t *testing.T) {
	text := `资源名称：圆桌派

描述：圆桌派全季全集，《圆桌派》，别名《圆桌π》是一档的聊天文化类网络电视节目。

🧲 链接: https://mypikpak.com/s/VO

👉使用 PikPak 秒存，立即在线观看👈 (https://toapp.mypikpak.com/toapp?__add_url=https://mypikpak.com/s/VO&source=pptg&__campaign=/s/VO)

📁 文件大小：86.7GB
🏷 文件类型：#脱口秀#综艺##文化节目#中国文化`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want only real pikpak resource: %+v", len(links), links)
	}
	link := links[0]
	if link.Type != "pikpak" || link.URL != "https://mypikpak.com/s/VO" {
		t.Fatalf("link = %+v, want pikpak resource", link)
	}
	if link.MediaTitle != "圆桌派" || link.Note != "圆桌派" || link.MediaSize != "86.7GB" || link.MediaCategory != "综艺" {
		t.Fatalf("metadata = %+v, want title/size/category", link)
	}
	if link.MediaTags != "脱口秀 综艺 文化节目 中国文化" {
		t.Fatalf("tags = %q", link.MediaTags)
	}
}

func TestExtractMediaMetadataFromPlainTVTitle(t *testing.T) {
	text := `电视剧 超感迷宫 2025 4K 全20集
链接：https://cloud.189.cn/t/Y7rUvynue6vm`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	link := links[0]
	if link.MediaTitle != "超感迷宫" || link.MediaYear != "2025" || link.MediaQuality != "4K" || link.MediaEpisode != "20集" || link.MediaCategory != "电视剧" {
		t.Fatalf("metadata = %+v, want plain tv title metadata", link)
	}
}

func TestExtractMediaMetadataFromAngleBracketCategory(t *testing.T) {
	text := `《国产剧》迷墙 (2026)  2160p WEB-DL H265 DDP5.1 主演: 郭京飞 / 任素汐 / 谷嘉诚 / 漆昱辰 / 温峥嵘

123云盘：https://1856557151.share.123pan.cn/123pan/oJqrvd-YON9d

百度网盘：https://pan.baidu.com/s/184SrcMA15CApBQsvjtLFVQ?pwd=x7hv`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(links), links)
	}
	for _, link := range links {
		if link.MediaTitle != "迷墙" || link.MediaYear != "2026" || link.MediaCategory != "国产剧" {
			t.Fatalf("link = %+v, want title/year/category", link)
		}
		if link.MediaQuality != "2160p WEB-DL H265 DDP5.1" {
			t.Fatalf("quality = %q", link.MediaQuality)
		}
	}
}

func TestExtractMediaMetadataFromInlineDriveLinksAndTags(t *testing.T) {
	text := `海贼王合集 国语日语

◎年  代 1999
◎产  地 日本
◎类  别 喜剧 / 动作 / 动画 / 奇幻 / 冒险
◎豆  瓣 9.5

大小：1.5T
标签：#海贼王 #航海王 #ワンピース #OnePiece #动画 #动漫 #爷青回 阿里   https://www.aliyundrive.com/s/QyVTWdmGM1o 115     https://115cdn.com/s/swfx55h3ffc?password=s367#
访问码：s367`

	links := NewExtractor().Extract(text)
	if len(links) != 2 {
		t.Fatalf("len = %d, want aliyun and 115 links: %+v", len(links), links)
	}
	for _, link := range links {
		if link.MediaTitle != "海贼王合集 国语日语" || link.MediaYear != "1999" || link.MediaSize != "1.5T" {
			t.Fatalf("link = %+v, want title/year/size", link)
		}
		if link.MediaTags != "海贼王 航海王 ワンピース OnePiece 动画 动漫 爷青回 阿里" {
			t.Fatalf("tags = %q", link.MediaTags)
		}
	}
	if links[1].Type != "115" || links[1].Password != "s367" {
		t.Fatalf("second link = %+v, want 115 password", links[1])
	}
}

func TestExtractMediaMetadataFromShortDramaDirectory(t *testing.T) {
	text := `名称：2026年6月9日 短剧更新目录3

描述：目录：
1.白绫三尺惜红颜（63集）吴竹照＆觅七
2.嫡女归京，我被狼群宠上天（73集）Ai短剧

阿里：https://www.alipan.com/s/TTXfbCYaCgk
夸克：https://pan.quark.cn/s/689d3b4512f2
百度：https://pan.baidu.com/s/1eKVTQkEETVy1hIYS8YA5-A?pwd=tjyd

📁 大小：N
🏷 标签：#短剧 #最新短剧 #合集 #擦边短剧 #短剧榜 #热力榜`

	links := NewExtractor().Extract(text)
	if len(links) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(links), links)
	}
	for _, link := range links {
		if link.MediaTitle != "2026年6月9日 短剧更新目录3" || link.MediaCategory != "短剧" {
			t.Fatalf("link = %+v, want directory metadata", link)
		}
		if link.MediaSize != "" {
			t.Fatalf("media size = %q, want invalid size ignored", link.MediaSize)
		}
		if link.MediaTags != "短剧 最新短剧 合集 擦边短剧 短剧榜 热力榜" {
			t.Fatalf("tags = %q", link.MediaTags)
		}
	}
	if links[2].Type != "baidu" || links[2].Password != "tjyd" {
		t.Fatalf("baidu link = %+v, want password", links[2])
	}
}

func TestExtractMediaMetadataFromUpdatedEpisodeTitle(t *testing.T) {
	text := `名称：迷墙 (2026) 更至06集 [4K][剧情][郭京飞/任素汐]

描述：倒霉透顶的小夫妻。

链接：https://pan.xunlei.com/s/VOuXNeFlYfJVnesX3zRR8IRiA1?pwd=3ypd#

📁 大小：NG
🏷 标签：#迷墙 #剧集 #4K #剧情 #郭京飞 #任素汐 #xunlei`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	link := links[0]
	if link.MediaTitle != "迷墙" || link.MediaYear != "2026" || link.MediaEpisode != "更新06集" || link.MediaQuality != "4K" {
		t.Fatalf("metadata = %+v, want updated episode metadata", link)
	}
	if link.MediaSize != "" {
		t.Fatalf("media size = %q, want invalid size ignored", link.MediaSize)
	}
}

func TestExtractMediaMetadataFromOneMessageMultipleTianyiTitles(t *testing.T) {
	text := `日剧分享六
麻烦一族.Involvement in Family Affairs.(2022) {tmdb-158896}
链接：https://cloud.189.cn/t/q2yUJjR7Bjqm（访问码：5zk9）
罗布奥特曼.Ultraman R／B.(2018) {tmdb-81959}
链接：https://cloud.189.cn/t/2Q3Aban67fii（访问码：xmt5）
恋爱何必认真？.What Do You Really Do About Love？.(2022) {tmdb-194854}
链接：https://cloud.189.cn/t/aEnIBjaUbUVj（访问码：1wbv）

标签  #剧集 #合集 #刮销 #4k

大小：1t`

	links := NewExtractor().Extract(text)
	if len(links) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(links), links)
	}
	want := []struct {
		title string
		year  string
		tmdb  string
		pass  string
	}{
		{"麻烦一族.Involvement in Family Affairs.", "2022", "158896", "5zk9"},
		{"罗布奥特曼.Ultraman R／B.", "2018", "81959", "xmt5"},
		{"恋爱何必认真？.What Do You Really Do About Love？.", "2022", "194854", "1wbv"},
	}
	for i, item := range want {
		if links[i].MediaTitle != item.title || links[i].MediaYear != item.year || links[i].MediaTMDBID != item.tmdb || links[i].Password != item.pass {
			t.Fatalf("link %d = %+v, want %+v", i, links[i], item)
		}
		if links[i].MediaSize != "1t" || links[i].MediaTags != "剧集 合集 刮销 4k" {
			t.Fatalf("shared metadata for link %d = %+v", i, links[i])
		}
	}
}

func TestExtractMediaMetadataFromBracketQualityAndSeasonRanges(t *testing.T) {
	text := `名称：厂区日志（2026）【4K.HDR.50fps】【更12集】【剧情/喜剧】【王宁/尹贝希】
.
描述：在大城市工作的王美琳和唐甜。
.
链接：https://pan.quark.cn/s/096c12ad4222
.
📁 大小：NG
🏷 标签：#国剧 #剧情 #喜剧 #厂区日志 #4K #HDR #50fps`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	link := links[0]
	if link.MediaTitle != "厂区日志" || link.MediaYear != "2026" || link.MediaEpisode != "更新12集" {
		t.Fatalf("metadata = %+v, want title/year/update episode", link)
	}
	if link.MediaQuality != "4K HDR 50fps" {
		t.Fatalf("quality = %q", link.MediaQuality)
	}
}

func TestExtractMediaMetadataFromStructuredTVWithBracketQuality(t *testing.T) {
	text := `📺 电视剧：莫离 (2026) S01E01
🍿 TMDB ID: 292696
⭐️ 评分: 0.0
🎭 题材: 剧情
📂 地区: 大陆
🎞️ 质量: [4K] [HDR10]
💾 大小: 2.33 GB
🔗 链接: https://115.com/s/swszh9233xj?password=q8e8`

	links := NewExtractor().Extract(text)
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	link := links[0]
	if link.MediaTitle != "莫离" || link.MediaYear != "2026" || link.MediaSeason != "S01" || link.MediaEpisode != "E01" {
		t.Fatalf("title/sequence metadata = %+v", link)
	}
	if link.MediaQuality != "[4K] [HDR10]" || link.MediaSize != "2.33 GB" || link.MediaTMDBID != "292696" {
		t.Fatalf("structured metadata = %+v", link)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
