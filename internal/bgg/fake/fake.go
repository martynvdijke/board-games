// Package fakebgg provides a small, deterministic BoardGameGeek XML API2
// server for local development and integration tests.
package fakebgg

import (
	"fmt"
	"net/http"
	"strings"
)

const collectionXML = `<?xml version="1.0" encoding="utf-8"?>
<items total="3" termsofuse="https://boardgamegeek.com/xmlapi/termsofuse">
  <item objecttype="thing" objectid="13" subtype="boardgame">
    <name>Catan</name><yearpublished>1995</yearpublished>
    <image>https://example.invalid/catan.jpg</image><thumbnail>https://example.invalid/catan.jpg</thumbnail>
    <stats minplayers="3" maxplayers="4" minplaytime="60" maxplaytime="120" playingtime="120" minage="10"><rating value="8"/></stats>
    <status own="1" prevowned="0" fortrade="0" want="0" wanttoplay="0" wanttobuy="0" wishlist="0" preordered="0" lastmodified="2026-01-01"/>
    <numplays>2</numplays>
  </item>
  <item objecttype="thing" objectid="266192" subtype="boardgame">
    <name>Wingspan</name><yearpublished>2019</yearpublished>
    <image>https://example.invalid/wingspan.jpg</image><thumbnail>https://example.invalid/wingspan.jpg</thumbnail>
    <stats minplayers="1" maxplayers="5" minplaytime="40" maxplaytime="70" playingtime="70" minage="10"><rating value="9"/></stats>
    <status own="1" prevowned="0" fortrade="0" want="0" wanttoplay="0" wanttobuy="0" wishlist="0" preordered="0" lastmodified="2026-01-01"/>
    <numplays>1</numplays>
  </item>
  <item objecttype="thing" objectid="163412" subtype="boardgame">
    <name>Patchwork</name><yearpublished>2014</yearpublished>
    <image>https://example.invalid/patchwork.jpg</image><thumbnail>https://example.invalid/patchwork.jpg</thumbnail>
    <stats minplayers="2" maxplayers="2" minplaytime="15" maxplaytime="30" playingtime="30" minage="8"><rating value="0"/></stats>
    <status own="1" prevowned="0" fortrade="0" want="0" wanttoplay="0" wanttobuy="0" wishlist="0" preordered="0" lastmodified="2026-01-01"/>
    <numplays>0</numplays>
  </item>
</items>`

const thingXML = `<?xml version="1.0" encoding="utf-8"?>
<items termsofuse="https://boardgamegeek.com/xmlapi/termsofuse">
%s
</items>`

func thing(id, name, desc, year, minP, maxP, minT, maxT, age string, rating, weight float64, rank int, cats, mechs, designers, publishers []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<item type="boardgame" id="%s"><thumbnail>https://example.invalid/%s.jpg</thumbnail><image>https://example.invalid/%s.jpg</image><name type="primary" sortindex="1" value="%s"/><description>%s</description><yearpublished value="%s"/><minplayers value="%s"/><maxplayers value="%s"/><minplaytime value="%s"/><maxplaytime value="%s"/><minage value="%s"/>`, id, id, id, name, desc, year, minP, maxP, minT, maxT, age)
	b.WriteString(`<statistics><ratings><usersrated value="100"/><average value="`)
	fmt.Fprintf(&b, `%.1f`, rating)
	b.WriteString(`"/><bayesaverage value="`)
	fmt.Fprintf(&b, `%.1f`, rating)
	b.WriteString(`"/><ranks><rank type="subtype" id="1" name="boardgame" friendlyname="Board Game Rank" value="`)
	fmt.Fprintf(&b, `%d`, rank)
	b.WriteString(`"/></ranks></ratings><averageweight value="`)
	fmt.Fprintf(&b, `%.1f`, weight)
	b.WriteString(`"/></statistics>`)
	for _, c := range cats {
		fmt.Fprintf(&b, `<link type="boardgamecategory" id="1" value="%s"/>`, c)
	}
	for _, m := range mechs {
		fmt.Fprintf(&b, `<link type="boardgamemechanic" id="2" value="%s"/>`, m)
	}
	for _, d := range designers {
		fmt.Fprintf(&b, `<link type="boardgamedesigner" id="3" value="%s"/>`, d)
	}
	for _, p := range publishers {
		fmt.Fprintf(&b, `<link type="boardgamepublisher" id="4" value="%s"/>`, p)
	}
	b.WriteString(`</item>`)
	return b.String()
}

func things() string {
	return thing("13", "Catan", "Trade, build, settle. A classic of resource management and friendly negotiation.", "1995", "3", "4", "60", "120", "10", 7.1, 2.3, 500, []string{"Negotiation", "Economic"}, []string{"Trading", "Dice Rolling"}, []string{"Klaus Teuber"}, []string{"KOSMOS"}) +
		thing("266192", "Wingspan", "A relaxing engine builder about attracting birds to your wildlife preserve.", "2019", "1", "5", "40", "70", "10", 8.1, 2.4, 28, []string{"Animals", "Card Game"}, []string{"Hand Management", "Set Collection"}, []string{"Elizabeth Hargrave"}, []string{"Stonemaier Games"}) +
		thing("163412", "Patchwork", "Two-player quilting with a brutal little economy.", "2014", "2", "2", "15", "30", "8", 7.5, 1.6, 96, []string{"Abstract Strategy", "Puzzle"}, []string{"Tile Placement", "Time Track"}, []string{"Uwe Rosenberg"}, []string{"Lookout Games"})
}

func plays(page string) string {
	if page == "2" {
		return `<?xml version="1.0" encoding="utf-8"?><plays username="alexplays" userid="1" total="3" page="2"><play id="9003" date="2026-02-14" quantity="1" length="30" location="Home" incomplete="0" nowinstats="0"><item name="Patchwork" objecttype="thing" objectid="163412"/><players><player username="alexplays" userid="1" name="Alex" startposition="1" color="blue" score="22" new="1" rating="8" win="1"/><player username="friend" userid="2" name="Sam" startposition="2" color="red" score="18" new="1" rating="7" win="0"/></players></play></plays>`
	}
	return `<?xml version="1.0" encoding="utf-8"?><plays username="alexplays" userid="1" total="3" page="1"><play id="9001" date="2026-03-02" quantity="1" length="75" location="Home" incomplete="0" nowinstats="0"><item name="Wingspan" objecttype="thing" objectid="266192"/><players><player username="alexplays" userid="1" name="Alex" startposition="1" color="blue" score="82" new="0" rating="9" win="1"/><player username="friend" userid="2" name="Sam" startposition="2" color="red" score="76" new="0" rating="8" win="0"/></players></play><play id="9002" date="2026-02-01" quantity="2" length="90" location="Pub" incomplete="0" nowinstats="0"><item name="Catan" objecttype="thing" objectid="13"/><players><player username="alexplays" userid="1" name="Alex" startposition="1" color="white" score="10" new="0" rating="8" win="0"/><player username="friend" userid="2" name="Sam" startposition="2" color="orange" score="10" new="0" rating="8" win="1"/></players></play></plays>`
}

// Handler returns the fake XML API2 handler.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /xmlapi2/collection", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") == "unknown" {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><items total="0"></items>`))
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(collectionXML))
	})
	mux.HandleFunc("GET /xmlapi2/thing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, thingXML, things())
	})
	mux.HandleFunc("GET /xmlapi2/plays", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(plays(r.URL.Query().Get("page"))))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("fake BoardGameGeek XML API2\n"))
	})
	return mux
}
