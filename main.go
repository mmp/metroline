package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
)

type VATSIMState struct {
	Pilots      []Pilot      `json:"pilots"`
	Controllers []Controller `json:"controllers"`
	ATIS        []ATIS       `json:"atis"`
}

type Pilot struct {
	CID         int     `json:"cid"`
	Name        string  `json:"name"`
	Callsign    string  `json:"callsign"`
	Server      string  `json:"server"`
	PilotRating int     `json:"pilot_rating"`
	Latitude    float32 `json:"latitude"`
	Longitude   float32 `json:"longitude"`
	Altitude    int     `json:"altitude"`
	Groundspeed int     `json:"groundspeed"`
	Transponder string  `json:"transponder"`
	Heading     int     `json:"heading"`
	QNH_Hg      float32 `json:"qnh_i_hg"`
	QNH_mb      float32 `json:"qnh_mb"`
	FlightPlan  struct {
		Rules               string    `json:"flight_rules"`
		Aircraft            string    `json:"aircraft"`
		AircraftFAA         string    `json:"aircraft_faa"`
		AircraftShort       string    `json:"aircraft_short"`
		Departure           string    `json:"departure"`
		Arrival             string    `json:"arrival"`
		Alternate           string    `json:"alternate"`
		CruiseTAS           string    `json:"cruise_tas"`
		Altitude            string    `json:"altitude"`
		DepartureTime       string    `json:"deptime"`
		EnrouteTime         string    `json:"enroute_time"`
		FuelTime            string    `json:"fuel_time"`
		Remarks             string    `json:"remarks"`
		Route               string    `json:"route"`
		RevisionID          int       `json:"revision_id"`
		AssignedTransponder string    `json:"assigned_transponder"`
		Logon               time.Time `json:"logon_time"`
		LastUpdate          time.Time `json:"last_updated"`
	} `json:"flight_plan"`
}

type Controller struct {
	CID        int       `json:"cid"`
	Name       string    `json:"name"`
	Callsign   string    `json:"callsign"`
	Frequency  string    `json:"frequency"`
	Facility   int       `json:"facility"`
	Rating     int       `json:"rating"`
	Server     string    `json:"server"`
	Range      int       `json:"visual_range"`
	ATIS       []string  `json:"text_atis"`
	Logon      time.Time `json:"logon_time"`
	LastUpdate time.Time `json:"last_updated"`
}

type ATIS struct {
	CID        int       `json:"cid"`
	Name       string    `json:"name"`
	Callsign   string    `json:"callsign"`
	Frequency  string    `json:"frequency"`
	Facility   int       `json:"facility"`
	Rating     int       `json:"rating"`
	Server     string    `json:"server"`
	Range      int       `json:"visual_range"`
	ATISCode   string    `json:"atis_code"`
	ATIS       []string  `json:"text_atis"`
	Logon      time.Time `json:"logon_time"`
	LastUpdate time.Time `json:"last_updated"`
}

func FetchVATSIMState() (*VATSIMState, error) {
	st, err := FetchURL("https://status.vatsim.net/status.json")
	if err != nil {
		return nil, err
	}

	var status struct {
		Data struct {
			V3           []string `json:"v3"`
			Transceivers []string `json:"transceivers"`
		} `json:"data"`
		Metar []string `json:"metar"`
	}
	if err := json.Unmarshal(st, &status); err != nil {
		return nil, err
	}
	if len(status.Data.V3) != 1 || len(status.Metar) != 1 {
		return nil, fmt.Errorf("Unexpected response format: %s -> %+v\n", string(st), status)
	}

	var state VATSIMState
	st, err = FetchURL(status.Data.V3[0])
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(st, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

//go:embed ZNY-positions.json
var znyPositionConfigJSON []byte

type Position struct {
	Name       string  `json:"callsign"` // e.g. JFK_TWR
	Descriptor string  `json:"name"`
	Callsign   string  `json:"radioName"` // e.g., Kennedy Tower
	Frequency  float64 `json:"frequency"`
	ERAM       struct {
		SectorId string `json:"sectorId"`
	} `json:"eramConfiguration"`
	STARS struct {
		Subset   int    `json:"subset"`
		SectorId string `json:"sectorId"`
	} `json:"starsConfiguration"`
}

type MajorAirport struct {
	Satellites []string
	Location   [2]float32
}

// convert -resize 20x20 ~/Downloads/ZNY-Mediakit/ZNY-transparent-black-1000x1000px.png zny.png
//
//go:embed zny.png
var znyPNG []byte

func main() {
	state, err := FetchVATSIMState()
	if err != nil {
		panic(err)
	}

	// Filter down to the traffic we're interested in reporting.
	// Majors and their sats.
	n90 := map[string]MajorAirport{
		"KJFK": MajorAirport{
			Satellites: []string{"KFRG", "KISP", "KOXC", "KFOK", "KBDR", "KHVN"},
			Location:   [2]float32{-73.780968, 40.641766},
		},
		"KLGA": MajorAirport{
			Satellites: []string{"KDXR", "KHPN"},
			Location:   [2]float32{-73.87261, 40.77724},
		},
		"KEWR": MajorAirport{
			Satellites: []string{"KTEB", "KCDW", "KMMU"},
			Location:   [2]float32{-74.174538, 40.689491},
		},
	}
	n90dep, n90arr, n90count := CountTraffic(state, n90)

	// Get the online controllers
	var znyPositions []Position
	dec := json.NewDecoder(bytes.NewReader(znyPositionConfigJSON))
	if err := dec.Decode(&znyPositions); err != nil {
		panic(err)
	}
	online := ActiveControllers(state, znyPositions)
	ctr := slices.ContainsFunc(online, func(c Controller) bool { return strings.HasSuffix(c.Callsign, "_CTR") })

	// Print it out, per https://github.com/matryer/xbar-plugins/blob/main/CONTRIBUTING.md
	fmt.Printf("%d", len(online))
	if ctr {
		fmt.Printf("*")
	}
	fmt.Printf(":headphones: %d :airplane: | templateImage=%s", n90count, Base64(znyPNG))
	fmt.Printf("\n")

	// Controllers
	if len(online) > 0 {
		fmt.Printf("---\n")

		for _, ctrl := range online {
			s := time.Since(ctrl.Logon)
			online := ""
			if s.Hours() > 0 {
				online = fmt.Sprintf("%d:", int(s.Hours()))
			}
			online += fmt.Sprintf("%02d", int(s.Minutes())-60*int(s.Hours()))

			fmt.Printf("%s - %s (%s) | font=Monaco | href=https://nyartcc.org/controller/%d\n", ctrl.Callsign, ctrl.Name, online, ctrl.CID)
		}
	}

	// Traffic
	fmt.Printf("---\n")
	for _, major := range slices.Sorted(maps.Keys(n90)) {
		fmt.Printf("%s %2d🛫 %2d🛬 | font=Monaco | href=https://vatsim-radar.com/airport/%s\n", major, n90dep[major], n90arr[major], major)
	}
}

func CountTraffic(state *VATSIMState, airports map[string]MajorAirport) (map[string]int, map[string]int, int) {
	major := func(ap string) (string, [2]float32) { // return corresponding major
		if info, ok := airports[ap]; ok { // it is a major
			return ap, info.Location
		}
		for major, info := range airports {
			if slices.Contains(info.Satellites, ap) { // satellite
				return major, info.Location
			}
		}
		return "", [2]float32{} // n/a
	}

	dep, arr := make(map[string]int), make(map[string]int)
	count := 0
	for _, pilot := range state.Pilots {
		pilotLoc := [2]float32{pilot.Longitude, pilot.Latitude}
		// Count departures that are within 30 miles of the center
		if major, loc := major(pilot.FlightPlan.Departure); major != "" && NMDistance2LL(loc, pilotLoc) < 30 {
			dep[major] = dep[major] + 1
			count++
		}
		// Arrivals within 300 miles but must also be moving
		if major, loc := major(pilot.FlightPlan.Arrival); major != "" && NMDistance2LL(loc, pilotLoc) < 300 && pilot.Groundspeed > 20 {
			arr[major] = arr[major] + 1
			count++
		}
	}

	return dep, arr, count
}

func ActiveControllers(state *VATSIMState, positions []Position) []Controller {
	var online []Controller
	for _, ctrl := range state.Controllers {
		if slices.ContainsFunc(positions, func(p Position) bool { return p.Name == ctrl.Callsign }) {
			online = append(online, ctrl)
		}
	}

	slices.SortFunc(online, func(a, b Controller) int { return strings.Compare(a.Callsign, b.Callsign) })

	return online
}

func FetchURL(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var text []byte
	if text, err = io.ReadAll(response.Body); err != nil {
		return nil, err
	}

	return text, nil
}

func Base64(b []byte) string {
	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	if _, err := io.Copy(enc, bytes.NewReader(b)); err != nil {
		panic(err)
	}
	enc.Close()
	return buf.String()
}

// NMDistance2ll returns the distance in nautical miles between two
// provided lat-long coordinates.
func NMDistance2LL(a [2]float32, b [2]float32) float32 {
	// https://www.movable-type.co.uk/scripts/latlong.html
	const R = 6371000 // metres
	rad := func(d float64) float64 { return float64(d) / 180 * math.Pi }
	lat1, lon1 := rad(float64(a[1])), rad(float64(a[0]))
	lat2, lon2 := rad(float64(b[1])), rad(float64(b[0]))
	dlat, dlon := lat2-lat1, lon2-lon1

	sqr := func(x float64) float64 { return x * x }

	x := sqr(math.Sin(dlat/2)) + math.Cos(lat1)*math.Cos(lat2)*sqr(math.Sin(dlon/2))
	c := 2 * math.Atan2(math.Sqrt(x), math.Sqrt(1-x))
	dm := R * c // in metres

	return float32(dm * 0.000539957)
}
