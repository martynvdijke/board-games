// Package bgg parses BoardGameGeek's read-only XML API2 and exposes a small
// rate-limited client.
package bgg

import (
	"encoding/xml"
	"strconv"
	"strings"
)

type Collection struct {
	XMLName xml.Name         `xml:"items"`
	Items   []CollectionItem `xml:"item"`
}

type CollectionItem struct {
	ObjectType    string           `xml:"objecttype,attr"`
	ObjectID      int              `xml:"objectid,attr"`
	Subtype       string           `xml:"subtype,attr"`
	Name          string           `xml:"name"`
	YearPublished int              `xml:"yearpublished"`
	Image         string           `xml:"image"`
	Thumbnail     string           `xml:"thumbnail"`
	Stats         CollectionStats  `xml:"stats"`
	Status        CollectionStatus `xml:"status"`
	NumPlays      int              `xml:"numplays"`
}

type CollectionStatus struct {
	Own          string `xml:"own,attr"`
	PrevOwned    string `xml:"prevowned,attr"`
	ForTrade     string `xml:"fortrade,attr"`
	Want         string `xml:"want,attr"`
	WantToPlay   string `xml:"wanttoplay,attr"`
	WantToBuy    string `xml:"wanttobuy,attr"`
	Wishlist     string `xml:"wishlist,attr"`
	Preordered   string `xml:"preordered,attr"`
	LastModified string `xml:"lastmodified,attr"`
}

func (s CollectionStatus) IsOwned() bool      { return s.Own == "1" }
func (s CollectionStatus) IsWishlisted() bool { return s.Wishlist == "1" || s.WantToBuy == "1" }
func (s CollectionStatus) IsWantToPlay() bool { return s.WantToPlay == "1" }
func (s CollectionStatus) IsPreordered() bool { return s.Preordered == "1" }
func (s CollectionStatus) IsForTrade() bool   { return s.ForTrade == "1" }

type CollectionStats struct {
	MinPlayers  int       `xml:"minplayers,attr"`
	MaxPlayers  int       `xml:"maxplayers,attr"`
	MinPlaytime int       `xml:"minplaytime,attr"`
	MaxPlaytime int       `xml:"maxplaytime,attr"`
	PlayingTime int       `xml:"playingtime,attr"`
	MinAge      int       `xml:"minage,attr"`
	Rating      BGGRating `xml:"rating"`
}

type BGGRating struct {
	Value        string  `xml:"value,attr"`
	Average      float64 `xml:"average,attr"`
	BayesAverage float64 `xml:"bayesaverage,attr"`
	Ranks        []Rank  `xml:"ranks>rank"`
}

func (r BGGRating) Personal() *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(r.Value), 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

type Rank struct {
	Type  string `xml:"type,attr"`
	ID    string `xml:"id,attr"`
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (r Rank) Number() *int {
	v, err := strconv.Atoi(strings.TrimSpace(r.Value))
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

type Things struct {
	XMLName xml.Name `xml:"items"`
	Items   []Thing  `xml:"item"`
}

type Thing struct {
	ID            int         `xml:"id,attr"`
	Type          string      `xml:"type,attr"`
	Thumbnail     string      `xml:"thumbnail"`
	Image         string      `xml:"image"`
	Names         []ThingName `xml:"name"`
	Description   string      `xml:"description"`
	YearPublished IntValue    `xml:"yearpublished"`
	MinPlayers    IntValue    `xml:"minplayers"`
	MaxPlayers    IntValue    `xml:"maxplayers"`
	MinPlaytime   IntValue    `xml:"minplaytime"`
	MaxPlaytime   IntValue    `xml:"maxplaytime"`
	PlayingTime   IntValue    `xml:"playingtime"`
	MinAge        IntValue    `xml:"minage"`
	Statistics    ThingStats  `xml:"statistics"`
	Links         []Link      `xml:"link"`
	Polls         []Poll      `xml:"poll"`
}

type ThingName struct {
	Type      string `xml:"type,attr"`
	SortIndex int    `xml:"sortindex,attr"`
	Value     string `xml:"value,attr"`
}

type IntValue struct {
	Value int `xml:"value,attr"`
}

type FloatValue struct {
	Value float64 `xml:"value,attr"`
}

type ThingStats struct {
	AverageWeight FloatValue `xml:"averageweight"`
	Ratings       struct {
		UsersRated   IntValue   `xml:"usersrated"`
		Average      FloatValue `xml:"average"`
		BayesAverage FloatValue `xml:"bayesaverage"`
		Ranks        []Rank     `xml:"ranks>rank"`
	} `xml:"ratings"`
}

type Link struct {
	Type  string `xml:"type,attr"`
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

type Poll struct {
	Name    string        `xml:"name,attr"`
	Results []PollResults `xml:"results"`
}

type PollResults struct {
	NumPlayers string       `xml:"numplayers,attr"`
	Results    []PollResult `xml:"result"`
}

type PollResult struct {
	Value    string `xml:"value,attr"`
	NumVotes string `xml:"numvotes,attr"`
}

func (t Thing) PrimaryName() string {
	for _, n := range t.Names {
		if n.Type == "primary" && n.Value != "" {
			return n.Value
		}
	}
	if len(t.Names) > 0 {
		return t.Names[0].Value
	}
	return ""
}

func (t Thing) IsExpansion() bool { return strings.Contains(t.Type, "expansion") }

func (t Thing) Rank() *int {
	for _, r := range t.Statistics.Ratings.Ranks {
		if r.Name == "boardgame" || r.Type == "subtype" {
			return r.Number()
		}
	}
	return nil
}

func (t Thing) LinksOfType(kind string) []string {
	var out []string
	for _, l := range t.Links {
		if l.Type == kind && strings.TrimSpace(l.Value) != "" {
			out = append(out, strings.TrimSpace(l.Value))
		}
	}
	return out
}

type Plays struct {
	XMLName xml.Name `xml:"plays"`
	Total   int      `xml:"total,attr"`
	Page    int      `xml:"page,attr"`
	Plays   []Play   `xml:"play"`
}

type Play struct {
	ID         int          `xml:"id,attr"`
	Date       string       `xml:"date,attr"`
	Quantity   int          `xml:"quantity,attr"`
	Length     int          `xml:"length,attr"`
	Incomplete int          `xml:"incomplete,attr"`
	Location   string       `xml:"location,attr"`
	Item       PlayItem     `xml:"item"`
	Players    []PlayPlayer `xml:"players>player"`
	Comments   string       `xml:"comments"`
}

type PlayItem struct {
	Name       string `xml:"name,attr"`
	ObjectType string `xml:"objecttype,attr"`
	ObjectID   int    `xml:"objectid,attr"`
}

type PlayPlayer struct {
	Name          string `xml:"name,attr"`
	Username      string `xml:"username,attr"`
	Score         string `xml:"score,attr"`
	Win           string `xml:"win,attr"`
	New           string `xml:"new,attr"`
	Rating        string `xml:"rating,attr"`
	StartPosition string `xml:"startposition,attr"`
	Color         string `xml:"color,attr"`
}

func (p PlayPlayer) IsWinner() bool { return p.Win == "1" }
func (p PlayPlayer) IsNew() bool    { return p.New == "1" }
