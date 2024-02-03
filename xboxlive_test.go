package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/imroc/req/v3"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func openTestFile(t *testing.T, endpoint, filename string) []byte {
	fp := filepath.Join("testdata", endpoint, filename)
	out, err := os.ReadFile(fp)

	if err != nil {
		t.Fatalf("failed to read test file: %s", err)
	}

	return out
}

func TestColourList(t *testing.T) {
	cases := map[string]struct {
		in       []string
		expected []string
	}{
		"empty list": {
			in:       []string{},
			expected: []string{},
		},
		"single item": {
			in:       []string{"a"},
			expected: []string{"{green}a{clear}"},
		},
		"two items": {
			in:       []string{"a", "b"},
			expected: []string{"{green}a{clear}", "{red}b{clear}"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := colourList(tc.in)

			assert.Equal(t, tc.expected, out)
		})
	}
}

func TestXBLTitleHistoryRecentNames(t *testing.T) {
	cases := map[string]struct {
		deltas   []time.Duration
		expected []string
	}{
		"no titles": {
			deltas:   []time.Duration{},
			expected: []string{},
		},
		"one newer": {
			deltas:   []time.Duration{0},
			expected: []string{"0"},
		},
		"one older": {
			deltas:   []time.Duration{-(historyDifference + 10)},
			expected: []string{},
		},
		"one newer, one older": {
			deltas:   []time.Duration{-(historyDifference + 10), 0},
			expected: []string{"1"},
		},
	}

	now := time.Now()

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblth := XBLTitleHistory{
				Xuid:   "test",
				Titles: []XBLTitle{},
			}

			for n, d := range tc.deltas {
				title := XBLTitle{}
				title.Name = fmt.Sprintf("%d", n)
				title.TitleHistory.LastTimePlayed = now.Add(d)
				xblth.Titles = append(xblth.Titles, title)
			}

			out := xblth.RecentNames()

			assert.Equal(t, tc.expected, out)
		})
	}
}

func TestXBLTitleHistoryFirstTitleID(t *testing.T) {
	cases := map[string]struct {
		count    int
		expected string
		err      error
	}{
		"no titles": {
			count:    0,
			expected: "",
			err:      userNoTitlesErr,
		},
		"one title": {
			count:    1,
			expected: "0",
			err:      nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblth := XBLTitleHistory{
				Xuid:   "test",
				Titles: []XBLTitle{},
			}

			for i := 0; i < tc.count; i++ {
				title := XBLTitle{}
				title.TitleID = fmt.Sprintf("%d", i)
				xblth.Titles = append(xblth.Titles, title)
			}

			out, err := xblth.FirstTitleID()

			assert.Equal(t, tc.expected, out)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}

func TestXBLPlayerTitleAchievementsNewestAchievement(t *testing.T) {
	cases := map[string]struct {
		xblptafn string
		xblafn   string
		err      error
	}{
		"no unlocked achievements": {
			xblptafn: "no_unlocked_achievements.json",
			xblafn:   "default.json",
			err:      nil,
		},
		"unlocked achievements": {
			xblptafn: "unlocked_achievements.json",
			xblafn:   "unlocked.json",
			err:      nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblptajson := openTestFile(t, "XBLPlayerTitleAchievements", tc.xblptafn)
			xblpta := XBLPlayerTitleAchievements{}
			err := json.Unmarshal(xblptajson, &xblpta)
			assert.Nil(t, err)

			xblajson := openTestFile(t, "XBLAchievement", tc.xblafn)
			xbla := XBLAchievement{}
			err = json.Unmarshal(xblajson, &xbla)
			assert.Nil(t, err)

			out, err := xblpta.NewestAchievement()
			assert.Equal(t, xbla, out)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}

func TestXBLPlayerSummary(t *testing.T) {
	cases := map[string]struct {
		xblpsfn  string
		expected string
	}{
		"player online": {
			xblpsfn:  "player_online.json",
			expected: "{cyan}player{clear} | {yellow}3225{clear} | {green}Online{clear} | Persona 3 Reload",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblpsjson := openTestFile(t, "XBLPlayerSummary", tc.xblpsfn)
			xblps := XBLPlayerSummary{}
			err := json.Unmarshal(xblpsjson, &xblps)
			assert.Nil(t, err)

			out := xblps.Summary()
			assert.Equal(t, tc.expected, out)
		})
	}
}

func TestXblGetXuid(t *testing.T) {
	cases := map[string]struct {
		xblxs    string
		xuid     string
		gamerTag string
		err      error
	}{
		"user exists": {
			xblxs:    "user_exists.json",
			xuid:     "2533274798129181",
			gamerTag: "xTACTICSx",
			err:      nil,
		},
		"user doesn't exist": {
			xblxs:    "user_doesnt_exist.json",
			xuid:     "",
			gamerTag: "",
			err:      userNotFoundErr,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblxsjson := openTestFile(t, "XBLXuidSearch", tc.xblxs)

			client := req.C()
			httpmock.ActivateNonDefault(client.GetClient())
			httpmock.RegisterResponder("GET", "https://xbl.io/api/v2/search/test", func(request *http.Request) (*http.Response, error) {
				resp := httpmock.NewBytesResponse(http.StatusOK, xblxsjson)
				return resp, nil
			})

			xuid, gamerTag, err := xblGetXuid(client, "test")
			assert.Equal(t, tc.xuid, xuid)
			assert.Equal(t, tc.gamerTag, gamerTag)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}

func TestXblLastGame(t *testing.T) {
	cases := map[string]struct {
		xblxs string
		msg   string
		err   error
	}{
		"no titles": {
			xblxs: "no_titles.json",
			msg:   "test has no recently played xboxlive games",
			err:   nil,
		},
		"recent titles": {
			xblxs: "recent_titles.json",
			msg:   "test's recently played xbox live games: {green}Persona 3 Reload{clear}",
			err:   nil,
		},
		"no recent titles": {
			xblxs: "no_recent_titles.json",
			msg:   "test has no recently played xboxlive games",
			err:   nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblxsjson := openTestFile(t, "XBLTitleHistory", tc.xblxs)

			client := req.C()
			httpmock.ActivateNonDefault(client.GetClient())
			httpmock.RegisterResponder("GET", "https://xbl.io/api/v2/player/titleHistory/test", func(request *http.Request) (*http.Response, error) {
				resp := httpmock.NewBytesResponse(http.StatusOK, xblxsjson)
				return resp, nil
			})

			out, err := xblLastGame(client, "test", "test")
			assert.Equal(t, tc.msg, out)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}

func TestXblLastAchievement(t *testing.T) {
	cases := map[string]struct {
		xblth   string
		xblpta  string
		titleId string
		msg     string
		err     error
	}{
		"has achievements": {
			xblth:   "has_achievements.json",
			xblpta:  "has_achievements.json",
			titleId: "1670311038",
			msg:     "test's last xbox live achievement: Persona 3 Reload - Back on Track (Defeated the Priestess)",
			err:     nil,
		},
		"no titles": {
			xblth:   "no_titles.json",
			xblpta:  "empty.json",
			titleId: "none",
			msg:     "test has not played any games",
			err:     nil,
		},
		"title has no achievements": {
			xblth:   "title_no_achievements.json",
			xblpta:  "title_no_achievements.json",
			titleId: "1632510060",
			msg:     "test has no achievements",
			err:     titleNoAchievementsErr,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblthjson := openTestFile(t, "XBLTitleHistory", tc.xblth)
			xblptajson := openTestFile(t, "XBLPlayerTitleAchievements", tc.xblpta)

			client := req.C()
			httpmock.ActivateNonDefault(client.GetClient())
			httpmock.RegisterResponder("GET", "https://xbl.io/api/v2/achievements/player/test", func(request *http.Request) (*http.Response, error) {
				resp := httpmock.NewBytesResponse(http.StatusOK, xblthjson)
				return resp, nil
			})
			httpmock.RegisterResponder("GET", fmt.Sprintf("https://xbl.io/api/v2/achievements/player/test/%s", tc.titleId), func(request *http.Request) (*http.Response, error) {
				resp := httpmock.NewBytesResponse(http.StatusOK, xblptajson)
				return resp, nil
			})

			out, err := xblLastAchievement(client, "test", "test")
			assert.Equal(t, tc.msg, out)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}

func TestXblPlayerSummary(t *testing.T) {
	cases := map[string]struct {
		xblpsfn  string
		expected string
		err      error
	}{
		"player online": {
			xblpsfn:  "player_online.json",
			expected: "{cyan}player{clear} | {yellow}3225{clear} | {green}Online{clear} | Persona 3 Reload",
			err:      nil,
		},
		"player offline": {
			xblpsfn:  "player_offline.json",
			expected: "{cyan}graffsu7{clear} | {yellow}2466{clear} | {red}Offline{clear}",
			err:      nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xblpsjson := openTestFile(t, "XBLPlayerSummary", tc.xblpsfn)

			client := req.C()
			httpmock.ActivateNonDefault(client.GetClient())
			httpmock.RegisterResponder("GET", "https://xbl.io/api/v2/player/summary/test", func(request *http.Request) (*http.Response, error) {
				resp := httpmock.NewBytesResponse(http.StatusOK, xblpsjson)
				return resp, nil
			})

			out, err := xblPlayerSummary(client, "test", "test")
			assert.Equal(t, tc.expected, out)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}
