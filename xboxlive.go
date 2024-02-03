package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

const (
	historyDifference = time.Hour * 24 * 30
)

var (
	userNotFoundErr        = errors.New("user not found")
	userNoTitlesErr        = errors.New("user has no achievements")
	titleNoAchievementsErr = errors.New("title has no achievements")
)

func colourString(in, colour string) string {
	return fmt.Sprintf("{%s}%s{clear}", colour, in)
}

func colourList(in []string) (out []string) {
	out = []string{}

	colours := []string{"green", "red", "blue", "orange", "magenta", "cyan", "yellow"}
	cl := len(colours)

	for n, i := range in {
		c := colours[n%cl]
		o := colourString(i, c)
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
	Xuid   string     `json:"xuid"`
	Titles []XBLTitle `json:"titles"`
}

type XBLTitle struct {
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
}

func (xblth *XBLTitleHistory) RecentNames() (out []string) {
	out = []string{}

	for _, t := range xblth.Titles {
		if time.Since(t.TitleHistory.LastTimePlayed) < historyDifference {
			out = append(out, t.Name)
		}
	}

	return out
}

func (xblpa *XBLTitleHistory) FirstTitleID() (string, error) {
	if len(xblpa.Titles) == 0 {
		return "", userNoTitlesErr
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

type XBLPlayerSummary struct {
	People []XBLPlayer `json:"people"`
}

type XBLPlayer struct {
	Xuid               string `json:"xuid"`
	DisplayName        string `json:"displayName"`
	Gamertag           string `json:"gamertag"`
	GamerScore         string `json:"gamerScore"`
	XboxOneRep         string `json:"xboxOneRep"`
	PresenceState      string `json:"presenceState"`
	PresenceText       string `json:"presenceText"`
	IsBroadcasting     bool   `json:"isBroadcasting"`
	MultiplayerSummary struct {
		InMultiplayerSession int `json:"InMultiplayerSession"`
		InParty              int `json:"InParty"`
	} `json:"multiplayerSummary"`
}

func (xblp *XBLPlayerSummary) Summary() string {
	var sb strings.Builder

	w := func(in, colour string) {
		s := colourString(in, colour)
		sb.WriteString(s)
	}

	p := xblp.People[0]

	w(p.Gamertag, "cyan")

	sb.WriteString(" | ")

	w(p.GamerScore, "yellow")

	sb.WriteString(" | ")

	stateColour := func(s string) string {
		if s == "Online" {
			return "green"
		}
		return "red"
	}

	w(p.PresenceState, stateColour(p.PresenceState))

	if p.PresenceState == "Online" {
		sb.WriteString(" | ")
		sb.WriteString(p.PresenceText)
	}

	return sb.String()
}

func xblGetXuid(client *req.Client, user string) (string, string, error) {
	result := &XBLXuidSearch{}

	_, err := client.R().
		SetPathParam("user", user).
		SetSuccessResult(&result).
		Get("https://xbl.io/api/v2/search/{user}")

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

	_, err := client.R().
		SetPathParam("xuid", xuid).
		SetSuccessResult(&result).
		Get("https://xbl.io/api/v2/player/titleHistory/{xuid}")

	if err != nil {
		return "", err
	}

	recent := result.RecentNames()

	if len(recent) == 0 {
		return fmt.Sprintf("%s has no recently played xboxlive games", gamerTag), nil
	}

	cl := colourList(recent)

	return fmt.Sprintf("%s's recently played xbox live games: %s", gamerTag, strings.Join(cl, ", ")), nil
}

func xblLastAchievement(client *req.Client, gamerTag, xuid string) (string, error) {
	lastAchievementResult := &XBLTitleHistory{}

	_, err := client.R().
		SetPathParam("xuid", xuid).
		SetSuccessResult(&lastAchievementResult).
		Get("https://xbl.io/api/v2/achievements/player/{xuid}")

	if err != nil {
		return "", err
	}

	lastAchievementID, err := lastAchievementResult.FirstTitleID()

	if lastAchievementID == "" {
		return fmt.Sprintf("%s has not played any games", gamerTag), nil
	}

	if err != nil {
		return "", err
	}

	playerTitleAchievementsResult := &XBLPlayerTitleAchievements{}

	_, err = client.R().
		SetPathParam("xuid", xuid).
		SetPathParam("id", lastAchievementID).
		SetSuccessResult(&playerTitleAchievementsResult).
		Get("https://xbl.io/api/v2/achievements/player/{xuid}/{id}")

	if err != nil {
		return "", err
	}

	lastAchievement, err := playerTitleAchievementsResult.NewestAchievement()

	if errors.Is(err, titleNoAchievementsErr) {
		return fmt.Sprintf("%s has no achievements", gamerTag), err
	}

	if err != nil {
		return "", err
	}

	gameName := lastAchievement.TitleAssociations[0].Name
	achievementName := lastAchievement.Name
	achievementDesc := strings.TrimSuffix(lastAchievement.Description, ".")

	return fmt.Sprintf("%s's last xbox live achievement: %s - %s (%s)", gamerTag, gameName, achievementName, achievementDesc), nil
}

func xblPlayerSummary(client *req.Client, gamerTag, xuid string) (string, error) {
	playerSummary := &XBLPlayerSummary{}

	_, err := client.R().
		SetPathParam("xuid", xuid).
		SetSuccessResult(&playerSummary).
		Get("https://xbl.io/api/v2/player/summary/{xuid}")

	if err != nil {
		return "", err
	}

	return playerSummary.Summary(), nil
}
