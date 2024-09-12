package dzen

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
	"unicode"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/gorilla/feeds"
	"github.com/kiberdruzhinnik/go-rss-bridge/pkg/utils"
)

// https://dzen.ru/api/v3/launcher/more?channel_name=konstantinromanov

const DZEN_API = "https://dzen.ru/api/v3"

type DzenVideosJSON struct {
	Items []struct {
		Title                 string `json:"title"`
		Text                  string `json:"text"`
		ExtLink               string `json:"ext_link"`
		PublicationDateString string `json:"publication_date"`
		PublicationDate       int
	} `json:"items"`
}

var VALID_USERNAME_PATTERN = []*unicode.RangeTable{
	unicode.Letter,
	unicode.Digit,
	{R16: []unicode.Range16{{'_', '_', 1}}},
	{R16: []unicode.Range16{{'-', '-', 1}}},
	{R16: []unicode.Range16{{'.', '.', 1}}},
}

func GetLatestVideosByUsername(username string) (DzenVideosJSON, error) {
	apiUrl := fmt.Sprintf("%s/launcher/more", DZEN_API)
	params := map[string]string{
		"channel_name": username,
	}

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		log.Println(err)
		return DzenVideosJSON{}, err
	}

	req.Header.Set("User-Agent", utils.USER_AGENT)

	q := req.URL.Query()
	for key, value := range params {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return DzenVideosJSON{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return DzenVideosJSON{}, err
	}

	var j DzenVideosJSON
	err = json.Unmarshal(body, &j)
	if err != nil {
		return DzenVideosJSON{}, err
	}

	return j, nil
}

func GetFeed(username string, skipBefore int) (string, error) {
	videos, err := GetLatestVideosByUsername(username)
	if err != nil {
		return "", err
	}

	if len(videos.Items) == 0 {
		return "", fmt.Errorf("no videos")
	}

	feed := &feeds.Feed{
		Title: fmt.Sprintf("Dzen Video @%s", username),
		Link: &feeds.Link{
			Href: fmt.Sprintf("https://dzen.ru/%s", username),
		},
		Description: fmt.Sprintf("Лента RSS Dzen %s", username),
	}

	seenSet := mapset.NewSet[string]()
	for _, entry := range videos.Items {

		publicationDate, err := strconv.Atoi(entry.PublicationDateString)
		if err != nil {
			log.Println(err)
			continue
		}

		if skipBefore > publicationDate {
			continue
		}
		videoUrl := entry.ExtLink

		if seenSet.Contains(videoUrl) {
			continue
		}

		seenSet.Add(videoUrl)
		feed.Items = append(feed.Items, &feeds.Item{
			Title:   entry.Title,
			Link:    &feeds.Link{Href: videoUrl},
			Created: time.Unix(int64(publicationDate), 0),
			Id:      videoUrl,
		})

	}

	return feed.ToRss()
}
