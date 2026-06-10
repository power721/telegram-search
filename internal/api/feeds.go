package api

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg-search/internal/notification"
	"tg-search/internal/resource"
)

const feedMaxLimit = 100

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	BuildDate   string    `xml:"lastBuildDate,omitempty"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link,omitempty"`
	GUID        string `xml:"guid"`
	Description string `xml:"description,omitempty"`
	PubDate     string `xml:"pubDate,omitempty"`
}

func (h handlers) feedLatest(c *gin.Context) {
	items, _, err := h.externalResourceItems(c, "", splitCSV(c.Query("cloud_types")), readFeedLimit(c), 0, false)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	h.writeRSS(c, "tg-search latest resources", "Latest indexed Telegram resources.", items)
}

func (h handlers) feedSearch(c *gin.Context) {
	keyword := strings.TrimSpace(firstQuery(c, "q", "kw", "keyword"))
	items, _, err := h.externalResourceItems(c, keyword, splitCSV(c.Query("cloud_types")), readFeedLimit(c), 0, false)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	title := "tg-search search"
	if keyword != "" {
		title += ": " + keyword
	}
	h.writeRSS(c, title, "Search results from tg-search.", items)
}

func (h handlers) feedSavedSearch(c *gin.Context) {
	if h.deps.SavedSearches == nil || h.deps.Resources == nil {
		errorText(c, http.StatusServiceUnavailable, "saved search feed is unavailable")
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	search, err := h.deps.SavedSearches.FindByID(c.Request.Context(), id)
	if err != nil {
		handleNotFound(c, err)
		return
	}
	result, err := h.deps.Resources.List(c.Request.Context(), resource.Query{
		Keyword:   search.Keyword,
		Type:      search.Filters.Type,
		Category:  search.Filters.Category,
		AccountID: search.Filters.AccountID,
		ChannelID: search.Filters.ChannelID,
		Limit:     readFeedLimit(c),
		MaxLimit:  feedMaxLimit,
		Sort:      "date_desc",
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	items := make([]resource.Item, 0, len(result.Items))
	for _, item := range result.Items {
		if notification.SavedSearchMatchesResource(search, item) {
			items = append(items, item)
		}
	}
	h.writeRSS(c, "tg-search saved search: "+search.Name, "Saved search feed for "+search.Keyword+".", items)
}

func (h handlers) writeRSS(c *gin.Context, title string, description string, items []resource.Item) {
	now := time.Now().UTC()
	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       title,
			Link:        requestBaseURL(c),
			Description: description,
			BuildDate:   now.Format(time.RFC1123Z),
			Items:       rssItems(c, items),
		},
	}
	output, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), output...))
}

func rssItems(c *gin.Context, items []resource.Item) []rssItem {
	out := make([]rssItem, 0, len(items))
	for _, item := range items {
		title := firstNonEmptyString(item.Title, item.Note, item.FileName, item.URL, item.ID)
		out = append(out, rssItem{
			Title:       title,
			Link:        rssItemLink(c, item),
			GUID:        item.ID,
			Description: rssDescription(item),
			PubDate:     item.Datetime.Format(time.RFC1123Z),
		})
	}
	return out
}

func rssItemLink(c *gin.Context, item resource.Item) string {
	if item.URL != "" {
		return item.URL
	}
	if item.TelegramFileID > 0 {
		return requestBaseURL(c) + "/v/" + strconv.FormatInt(item.TelegramFileID, 10)
	}
	return requestBaseURL(c)
}

func rssDescription(item resource.Item) string {
	parts := []string{}
	if item.Type != "" {
		parts = append(parts, "Type: "+item.Type)
	}
	if item.Category != "" {
		parts = append(parts, "Category: "+item.Category)
	}
	if item.ChannelTitle != "" {
		parts = append(parts, "Source: "+item.ChannelTitle)
	}
	if item.TelegramMessageID > 0 {
		parts = append(parts, "Telegram message: "+strconv.FormatInt(item.TelegramMessageID, 10))
	}
	return strings.Join(parts, "\n")
}

func readFeedLimit(c *gin.Context) int {
	limit, ok := optionalQueryNonNegativeInt(c, "limit")
	if !ok || limit <= 0 {
		return 50
	}
	if limit > feedMaxLimit {
		return feedMaxLimit
	}
	return limit
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.Split(forwarded, ",")[0]
	}
	return scheme + "://" + c.Request.Host
}
