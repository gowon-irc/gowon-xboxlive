package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

const (
	historyDifferenceNanoSeconds = 2628000000000000
)

var userNotFoundErr = errors.New("user not found")

type XBLXuidSearch struct {
	People []struct {
		Xuid     string `json:"xuid"`
		Gamertag string `json:"gamertag"`
	}
}

type XBLTitleHistory struct {
	Xuid   string `json:"xuid"`
	Titles []struct {
		TitleID     string `json:"titleId"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Achievement struct {
			CurrentAchievements int `json:"currentAchievements"`
			TotalAchievements   int `json:"totalAchievements"`
			CurrentGamerscore   int `json:"currentGamerscore"`
			TotalGamerscore     int `json:"totalGamerscore"`
			ProgressPercentage  int `json:"progressPercentage"`
		} `json:"achievement"`
		TitleHistory struct {
			LastTimePlayed time.Time `json:"lastTimePlayed"`
		} `json:"titleHistory"`
	} `json:"titles"`
}

func (xblth *XBLTitleHistory) Names() (out []string) {
	for _, t := range xblth.Titles {
		if time.Since(t.TitleHistory.LastTimePlayed) < historyDifferenceNanoSeconds {
			out = append(out, t.Name)
		}
	}

	return out
}

func xblGetXuid(client *req.Client, user string) (string, string, error) {
	result := &XBLXuidSearch{}

	url := fmt.Sprintf("https://xbl.io/api/v2/search/%s", user)
	_, err := client.R().
		SetSuccessResult(&result).
		Get(url)

	if err != nil {
		return "", "", err
	}

	if len(result.People) == 0 {
		return "", "", userNotFoundErr
	}

	return result.People[0].Xuid, result.People[0].Gamertag, nil
}

func colourList(in []string) (out []string) {
	out = []string{}

	colours := []string{"green", "red", "blue", "orange", "magenta", "cyan", "yellow"}
	cl := len(colours)

	for n, i := range in {
		c := colours[n%cl]
		o := fmt.Sprintf("{%s}%s{clear}", c, i)
		out = append(out, o)
	}

	return out
}

func xblLastGame(client *req.Client, gamerTag, xuid string) (string, error) {
	result := XBLTitleHistory{}

	url := fmt.Sprintf("https://xbl.io/api/v2/player/titleHistory/%s", xuid)
	_, err := client.R().
		SetSuccessResult(&result).
		Get(url)

	if err != nil {
		return "", err
	}

	if len(result.Titles) == 0 {
		return "user has no recently played xboxlive games", nil
	}

	cl := colourList(result.Names())

	return fmt.Sprintf("%s's played xbox live games: %s", gamerTag, strings.Join(cl, ", ")), nil
}

func xblLastAchievement(client *req.Client, gamerTag, xuid string) (string, error) {
	return xuid, nil
}
