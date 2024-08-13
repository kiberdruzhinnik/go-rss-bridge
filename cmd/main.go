package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kiberdruzhinnik/go-rss-bridge/pkg/utils"
	"github.com/kiberdruzhinnik/go-rss-bridge/pkg/vkvideo"
)

// VK init
var VK_TOKEN vkvideo.VkApiToken

func prepareVkToken() {
	for {
		log.Println("Trying to obtain VK Video anonymous token")
		vkToken, err := vkvideo.GetToken()
		if err != nil {
			log.Println(err)
			time.Sleep(10 * time.Second)
			continue
		}
		log.Println("Got VK Video anonymous token")
		VK_TOKEN = vkToken
		return
	}
}

func init() {
	prepareVkToken()
}

func vkVideoRoute(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	username = utils.StringsAllowlist(username, vkvideo.VALID_USERNAME_PATTERN)

	skipBeforeStr := strings.TrimSpace(c.Query("skip_before"))
	skipBeforeStr = utils.StringsAllowlist(skipBeforeStr, vkvideo.VALID_SKIP_BEFORE_PATTERN)
	skipBefore := -1
	if skipBeforeStr != "" {
		parsed, err := strconv.Atoi(skipBeforeStr)
		if err != nil {
			log.Printf("Got enormous int in skip_before = %s, defaulting to -1\n", skipBeforeStr)
			parsed = -1
		}
		skipBefore = parsed
	}

	if username == "" {
		c.String(http.StatusBadRequest, "error")
		return
	}

	timeNow := time.Now().Unix()

	if timeNow > (int64(VK_TOKEN.Expiration) - 3600) {
		vkToken, err := vkvideo.GetToken()
		if err != nil {
			log.Println(err)
		}
		VK_TOKEN = vkToken
		c.String(http.StatusBadRequest, "error")
		return
	}

	feed, err := vkvideo.GetFeed(username, VK_TOKEN, skipBefore)
	if err != nil {
		log.Println(err)
		c.String(http.StatusBadRequest, "error")
		return
	}

	c.String(http.StatusOK, feed)
}

func main() {
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/vkvideo/:username", vkVideoRoute)

	log.Fatal(router.Run(":8080"))
}
