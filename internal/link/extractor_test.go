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
	if len(byType["url"]) != 3 {
		t.Fatalf("fallback url count = %d, want 3 telegram links: %+v", len(byType["url"]), byType["url"])
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

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
