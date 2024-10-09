package rutube

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorilla/feeds"
	"github.com/kiberdruzhinnik/go-rss-bridge/pkg/utils"
)

const RUTUBE_SITE = "https://rutube.ru"

var VALID_USERNAME_PATTERN = []*unicode.RangeTable{
	unicode.Letter,
	unicode.Digit,
	{R16: []unicode.Range16{{'_', '_', 1}}},
	{R16: []unicode.Range16{{'-', '-', 1}}},
	{R16: []unicode.Range16{{'.', '.', 1}}},
}

var VALID_SKIP_BEFORE_PATTERN = []*unicode.RangeTable{
	unicode.Digit,
}

type RutubeVideo struct {
	Title string
	URL   string
	Date  time.Time
}

type RutubeVideos struct {
	Results []RutubeVideo
}

type Video struct {
	VideoURL      string `json:"video_url"`
	Title         string `json:"title"`
	PublicationTS string `json:"publication_ts"`
}

type Results struct {
	Results []Video `json:"results"`
}

type Data struct {
	Data Results `json:"data"`
}

type Videos map[string]Data

type Queries struct {
	Queries Videos `json:"queries"`
}

type API struct {
	API Queries `json:"api"`
}

func indexAt(s, sep string, n int) int {
	idx := strings.Index(s[n:], sep)
	if idx > -1 {
		idx += n
	}
	return idx
}

func stripBetweenTokens(input, startToken, endToken string) string {
	idxStart := indexAt(input, startToken, 0)
	if idxStart == -1 {
		log.Printf("Not found startToken %s\n", startToken)
		return ""
	}
	idxEnd := indexAt(input, endToken, idxStart)
	if idxEnd == -1 {
		log.Printf("Not found endToken %s\n", endToken)
		return ""
	}
	return input[idxStart+len(startToken) : idxEnd]
}

func parseTime(input string) (time.Time, error) {
	layout := "2006-01-02T15:04:05"
	t, err := time.Parse(layout, input)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func fixJsonEscapes(jsonString string) string {
	re := regexp.MustCompile(`\\x([0-9a-fA-F]{2})`)
	fixedJson := re.ReplaceAllStringFunc(jsonString, func(match string) string {
		hexCode := match[2:]
		intCode, _ := strconv.ParseInt(hexCode, 16, 32)
		return string(rune(intCode))
	})
	return fixedJson
}

func GetLatestVideosByUsername(username string) (RutubeVideos, error) {
	url := fmt.Sprintf("%s/channel/%s/", RUTUBE_SITE, username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err)
		return RutubeVideos{}, err
	}

	req.Header.Set("User-Agent", utils.USER_AGENT)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return RutubeVideos{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return RutubeVideos{}, err
	}

	parsed := stripBetweenTokens(string(body), "window.reduxState = {", "};")
	parsed = fmt.Sprintf("{%s}", parsed)
	parsed = fixJsonEscapes(parsed)

	var api API
	err = json.Unmarshal([]byte(parsed), &api)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return RutubeVideos{}, err
	}

	key := fmt.Sprintf("videos(%s)", username)

	var rutubeVideos RutubeVideos

	if videoData, ok := api.API.Queries[key]; ok {
		for _, video := range videoData.Data.Results {
			date, err := parseTime(video.PublicationTS)
			if err != nil {
				log.Println(err)
				continue
			}
			rutubeVideo := RutubeVideo{Title: video.Title, URL: video.VideoURL, Date: date}
			rutubeVideos.Results = append(rutubeVideos.Results, rutubeVideo)
		}
	} else {
		fmt.Println("Key not found:", key)
	}

	return rutubeVideos, nil
}

func GetFeed(username string, skipBefore int) (string, error) {
	videos, err := GetLatestVideosByUsername(username)
	if err != nil {
		return "", err
	}

	if len(videos.Results) == 0 {
		return "", fmt.Errorf("no videos")
	}

	feed := &feeds.Feed{
		Title: fmt.Sprintf("Rutube @%s", username),
		Link: &feeds.Link{
			Href: fmt.Sprintf("https://rutube.ru/channel/%s/", username),
		},
		Description: fmt.Sprintf("Лента RSS Rutube @%s", username),
	}

	seenSet := mapset.NewSet[string]()
	for _, entry := range videos.Results {
		if skipBefore > int(entry.Date.Unix()) {
			continue
		}

		if seenSet.Contains(entry.URL) {
			continue
		}

		seenSet.Add(entry.URL)
		feed.Items = append(feed.Items, &feeds.Item{
			Title:   entry.Title,
			Link:    &feeds.Link{Href: entry.URL},
			Created: entry.Date,
			Id:      entry.URL,
		})

	}

	return feed.ToRss()
}
