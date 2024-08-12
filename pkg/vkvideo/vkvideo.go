package vkvideo

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"unicode"

	"github.com/gorilla/feeds"
	"github.com/kiberdruzhinnik/go-rss-bridge/pkg/utils"
)

const VK_LOGIN_API = "https://login.vk.com"
const VK_API = "https://api.vk.com"

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

type VkApi struct {
	Token VkApiToken
}

type VkApiToken struct {
	Token      string
	Expiration int
}

type VkApiTokenJSON struct {
	Type string `json:"type"`
	Data struct {
		AccessToken string `json:"access_token"`
		Expires     int    `json:"expires"`
	} `json:"data"`
}

func GetToken() (VkApiToken, error) {
	apiUrl := VK_LOGIN_API

	params := map[string]string{
		"client_secret": "QbYic1K3lEV5kTGiqlq2",
		"client_id":     "6287487",
		"scopes":        "audio_anonymous,video_anonymous,photos_anonymous,profile_anonymous",
		"version":       "1",
		"app_id":        "6287487",
		"act":           "get_anonym_token",
	}

	req, err := http.NewRequest("POST", apiUrl, nil)
	if err != nil {
		log.Println(err)
		return VkApiToken{}, err
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
		return VkApiToken{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return VkApiToken{}, err
	}

	var j VkApiTokenJSON

	err = json.Unmarshal(body, &j)
	if err != nil {
		return VkApiToken{}, err
	}

	if j.Type != "okay" {
		return VkApiToken{}, fmt.Errorf("%v", j)
	}

	token := VkApiToken{
		Token:      j.Data.AccessToken,
		Expiration: j.Data.Expires,
	}
	return token, nil
}

type VkVideoSectionJSON struct {
	Response struct {
		Videos []struct {
			Description string `json:"description"`
			Image       []struct {
				URL string `json:"url"`
			} `json:"image"`
			Title  string `json:"title"`
			Player string `json:"player"`
			Date   int64  `json:"date"`
		} `json:"videos"`
	} `json:"response"`
}

type VkVideoJSON struct {
	Response struct {
		Videos []struct {
			Date        int    `json:"date"`
			Description string `json:"description"`
			ID          int    `json:"id"`
			OwnerID     int    `json:"owner_id"`
			Title       string `json:"title"`
		} `json:"videos"`
	} `json:"response"`
}

func GetLatestVideosByUsername(token VkApiToken, username string) (VkVideoJSON, error) {
	apiUrl := fmt.Sprintf("%s/method/catalog.getVideo", VK_API)
	params := map[string]string{
		"v":            "5.241",
		"client_id":    "6287487",
		"access_token": token.Token,
		"url":          fmt.Sprintf("https://vk.com/video/@%s", username),
		"need_blocks":  "1",
		"owner_id":     "0",
	}

	req, err := http.NewRequest("POST", apiUrl, nil)
	if err != nil {
		log.Println(err)
		return VkVideoJSON{}, err
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
		return VkVideoJSON{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return VkVideoJSON{}, err
	}

	var j VkVideoJSON
	err = json.Unmarshal(body, &j)
	if err != nil {
		return VkVideoJSON{}, err
	}

	return j, nil
}

func GetFeed(username string, token VkApiToken, skipBefore int) (string, error) {
	videos, err := GetLatestVideosByUsername(token, username)
	if err != nil {
		return "", err
	}

	if len(videos.Response.Videos) == 0 {
		return "", fmt.Errorf("no videos")
	}

	feed := &feeds.Feed{
		Title: fmt.Sprintf("VK Video @%s", username),
		Link: &feeds.Link{
			Href: fmt.Sprintf("https://vk.com/video/@%s", username),
		},
	}

	for _, entry := range videos.Response.Videos {
		// skip enabled
		if skipBefore > -1 {
			if entry.Date > skipBefore {
				feed.Items = append(feed.Items, &feeds.Item{
					Title:       entry.Title,
					Link:        &feeds.Link{Href: fmt.Sprintf("https://vk.com/video%d_%d", entry.OwnerID, entry.ID)},
					Description: entry.Description,
					Created:     time.Unix(int64(entry.Date), 0),
				})
			}
		} else {
			feed.Items = append(feed.Items, &feeds.Item{
				Title:       entry.Title,
				Link:        &feeds.Link{Href: fmt.Sprintf("https://vk.com/video%d_%d", entry.OwnerID, entry.ID)},
				Description: entry.Description,
				Created:     time.Unix(int64(entry.Date), 0),
			})
		}

	}

	return feed.ToAtom()
}
