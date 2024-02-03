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

var (
	userNotFoundErr        = errors.New("user not found")
	userNoAchievementsErr  = errors.New("user has no achievements")
	titleNoAchievementsErr = errors.New("title has no achievements")
)

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

type XBLPlayerAchievements struct {
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
			SourceVersion       int `json:"sourceVersion"`
		} `json:"achievement"`
		TitleHistory struct {
			LastTimePlayed time.Time `json:"lastTimePlayed"`
		} `json:"titleHistory"`
	} `json:"titles"`
}

func (xblpa *XBLPlayerAchievements) NewestTitleID() (string, error) {
	if len(xblpa.Titles) == 0 {
		return "", userNoAchievementsErr
	}

	return xblpa.Titles[0].TitleID, nil
}

type XBLPlayerTitleAchievements struct {
	Achievements []XBLAchievement `json:"achievements"`
	PagingInfo   struct {
		ContinuationToken any `json:"continuationToken"`
		TotalRecords      int `json:"totalRecords"`
	} `json:"pagingInfo"`
}

type XBLAchievement struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	TitleAssociations []struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	} `json:"titleAssociations"`
	ProgressState string `json:"progressState"`
	Progression   struct {
		Requirements []any     `json:"requirements"`
		TimeUnlocked time.Time `json:"timeUnlocked"`
	} `json:"progression"`
	IsSecret          bool   `json:"isSecret"`
	Description       string `json:"description"`
	AchievementType   string `json:"achievementType"`
	ParticipationType string `json:"participationType"`
	Rewards           []struct {
		Name        any    `json:"name"`
		Description any    `json:"description"`
		Value       string `json:"value"`
		Type        string `json:"type"`
		ValueType   string `json:"valueType"`
	} `json:"rewards"`
	EstimatedTime string `json:"estimatedTime"`
	Rarity        struct {
		CurrentCategory   string  `json:"currentCategory"`
		CurrentPercentage float64 `json:"currentPercentage"`
	} `json:"rarity"`
}

func (xblpta *XBLPlayerTitleAchievements) NewestAchievement() (newest XBLAchievement, err error) {
	if len(xblpta.Achievements) == 0 {
		return newest, titleNoAchievementsErr
	}

	newest = XBLAchievement{}

	for _, a := range xblpta.Achievements {
		if a.Progression.TimeUnlocked.After(newest.Progression.TimeUnlocked) {
			newest = a
		}
	}

	return newest, nil
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

	return fmt.Sprintf("%s's recently played xbox live games: %s", gamerTag, strings.Join(cl, ", ")), nil
}

func xblLastAchievement(client *req.Client, gamerTag, xuid string) (string, error) {
	lastAchievementResult := &XBLPlayerAchievements{}

	url := fmt.Sprintf("https://xbl.io/api/v2/achievements/player/%s", xuid)
	_, err := client.R().
		SetSuccessResult(&lastAchievementResult).
		Get(url)

	if err != nil {
		return "", err
	}

	lastAchievementID, err := lastAchievementResult.NewestTitleID()
	if err != nil {
		return "", err
	}

	playerTitleAchievementsResult := &XBLPlayerTitleAchievements{}

	url = fmt.Sprintf("https://xbl.io/api/v2/achievements/player/%s/%s", xuid, lastAchievementID)
	_, err = client.R().
		SetSuccessResult(&playerTitleAchievementsResult).
		Get(url)

	if err != nil {
		return "", err
	}

	lastAchievement, err := playerTitleAchievementsResult.NewestAchievement()
	if err != nil {
		return "", err
	}

	gameName := lastAchievement.TitleAssociations[0].Name
	achievementName := lastAchievement.Name
	achievementDesc := strings.TrimSuffix(lastAchievement.Description, ".")

	return fmt.Sprintf("%s's last xbox live achievement: %s - %s (%s)", gamerTag, gameName, achievementName, achievementDesc), nil
}
