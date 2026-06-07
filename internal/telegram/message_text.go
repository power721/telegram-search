package telegram

import (
	"strings"

	"github.com/gotd/td/tg"
)

func IndexedMessageText(message *tg.Message) (string, []string) {
	if message == nil {
		return "", nil
	}
	urls := messageURLs(message)
	text := message.Message
	for _, rawURL := range urls {
		if strings.Contains(text, rawURL) {
			continue
		}
		if text != "" {
			text += "\n"
		}
		text += rawURL
	}
	return text, urls
}

func messageURLs(message *tg.Message) []string {
	if message == nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	appendURL := func(rawURL string) {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return
		}
		if _, ok := seen[rawURL]; ok {
			return
		}
		seen[rawURL] = struct{}{}
		out = append(out, rawURL)
	}

	if entities, ok := message.GetEntities(); ok {
		for _, entity := range entities {
			switch e := entity.(type) {
			case *tg.MessageEntityTextURL:
				appendURL(e.URL)
			}
		}
	}

	if markup, ok := message.GetReplyMarkup(); ok {
		appendReplyMarkupURLs(markup, appendURL)
	}

	if media, ok := message.GetMedia(); ok {
		appendMediaURLs(media, appendURL)
	}

	return out
}

func appendReplyMarkupURLs(markup tg.ReplyMarkupClass, appendURL func(string)) {
	switch m := markup.(type) {
	case *tg.ReplyInlineMarkup:
		for _, row := range m.Rows {
			for _, button := range row.Buttons {
				appendButtonURL(button, appendURL)
			}
		}
	}
}

func appendButtonURL(button tg.KeyboardButtonClass, appendURL func(string)) {
	switch b := button.(type) {
	case *tg.KeyboardButtonURL:
		appendURL(b.URL)
	case *tg.KeyboardButtonURLAuth:
		appendURL(b.URL)
	case *tg.KeyboardButtonWebView:
		appendURL(b.URL)
	case *tg.KeyboardButtonSimpleWebView:
		appendURL(b.URL)
	}
}

func appendMediaURLs(media tg.MessageMediaClass, appendURL func(string)) {
	webPageMedia, ok := media.(*tg.MessageMediaWebPage)
	if !ok {
		return
	}
	webPage, ok := webPageMedia.Webpage.(*tg.WebPage)
	if !ok {
		return
	}
	appendURL(webPage.URL)
}
