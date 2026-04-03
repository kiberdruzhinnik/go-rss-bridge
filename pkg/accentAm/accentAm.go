package accentAm

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorilla/feeds"
	"github.com/kiberdruzhinnik/go-rss-bridge/pkg/utils"
	// mapset "github.com/deckarep/golang-set/v2"
	// "github.com/gorilla/feeds"
)

// https://accent-am.ru/funds/aktsent-5-fond-nedvizhimosti

var VALID_FUND_PATTERN = []*unicode.RangeTable{
	unicode.Letter,
	unicode.Digit,
	{R16: []unicode.Range16{{'_', '_', 1}}},
	{R16: []unicode.Range16{{'-', '-', 1}}},
	{R16: []unicode.Range16{{'.', '.', 1}}},
}

const BASE_URL = "https://accent-am.ru"

type Message struct {
	Title        string
	URL          string
	DateRaw      string
	Date         time.Time
	Availability string
}

func parseDate(s string) (time.Time, error) {
	layout := "02.01.2006, 15:04"
	return time.Parse(layout, strings.TrimSpace(s))
}

func GetLatestMessagesByFundName(fundName string) ([]Message, error) {
	url := fmt.Sprintf("%s/funds/%s", BASE_URL, fundName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err)
		return []Message{}, err
	}

	req.Header.Set("User-Agent", utils.USER_AGENT)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return []Message{}, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Println(err)
		return []Message{}, err
	}

	var results []Message

	// 1. Find tab button "Сообщения"
	doc.Find(".fund-documents__list__links a").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(strings.TrimSpace(s.Text()), "Сообщения") {
			tabID, exists := s.Attr("aria-controls")
			if !exists {
				return
			}

			// 2. Find corresponding tab panel
			panel := doc.Find(fmt.Sprintf("#%s", tabID))

			// 3. Find "За все года" tab
			panel.Find(".fund-documents__content").Each(func(_ int, content *goquery.Selection) {

				var allTabID string

				content.Find("[role='tab']").Each(func(_ int, tab *goquery.Selection) {
					if strings.Contains(tab.Text(), "За все года") {
						id, ok := tab.Attr("aria-controls")
						if ok {
							allTabID = id
						}
					}
				})

				if allTabID == "" {
					return
				}

				// 4. Find content for "За все года"
				allPanel := content.Find(fmt.Sprintf("#%s", allTabID))

				allPanel.Find("ul.fund-documents__documents li").Each(func(_ int, li *goquery.Selection) {

					a := li.Find("a.document-item")
					if a.Length() == 0 {
						return // skip header row
					}

					title := strings.TrimSpace(a.Find(".document-item__title").Text())
					url, _ := a.Attr("href")

					availability := strings.TrimSpace(
						a.Find(".document-item__info__text span:last-child").Text(),
					)

					dateStr := strings.TrimSpace(
						a.Find(".document-item__info__date span:last-child").Text(),
					)

					parsedDate, err := parseDate(dateStr)
					if err != nil {
						log.Printf("failed to parse date: %s (%v)", dateStr, err)
					}

					results = append(results, Message{
						Title:        title,
						URL:          fmt.Sprintf("%s/%s", BASE_URL, url),
						DateRaw:      dateStr,
						Date:         parsedDate,
						Availability: availability,
					})
				})
			})
		}
	})
	return results, nil
}

func GetFeed(fundName string) (string, error) {
	results, err := GetLatestMessagesByFundName(fundName)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no messages")
	}

	feed := &feeds.Feed{
		Title: fmt.Sprintf("Фонд акцент @%s", fundName),
		Link: &feeds.Link{
			Href: fmt.Sprintf("%s/funds/%s", BASE_URL, fundName),
		},
		Description: fmt.Sprintf("Сообщения фонда %s", fundName),
	}

	seenSet := mapset.NewSet[string]()
	for _, entry := range results {

		url := entry.URL

		if seenSet.Contains(url) {
			continue
		}

		seenSet.Add(url)
		feed.Items = append(feed.Items, &feeds.Item{
			Title:   entry.Title,
			Link:    &feeds.Link{Href: url},
			Created: entry.Date,
			Id:      url,
		})

	}

	return feed.ToRss()
}
