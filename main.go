package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

// convert -resize 20x20 ~/Downloads/ZNY-Mediakit/ZNY-transparent-black-1000x1000px.png zny.png
//
//go:embed zny.png
var znyPNG []byte

type Airport struct {
	Name     string
	Location [2]float32
}

func (a Airport) DistanceTo(p [2]float32) float32 {
	return NMDistance2LL(a.Location, p)
}

type MajorAirport struct {
	Airport
	Satellites []Airport
}

func main() {
	state, err := FetchVATSIMState()
	if err != nil {
		panic(err)
	}

	// Filter down to the traffic we're interested in reporting.
	// Majors and their sats.
	n90 := []MajorAirport{
		MajorAirport{
			Airport: Airport{
				Name:     "KJFK",
				Location: [2]float32{-73.780968, 40.641766},
			},
			Satellites: []Airport{
				Airport{Name: "KFRG", Location: [2]float32{-73.4134208, 40.7292742}},
				Airport{Name: "KISP", Location: [2]float32{-73.1006651, 40.7961357}},
				Airport{Name: "KOXC", Location: [2]float32{-73.1351825, 41.4782806}},
				Airport{Name: "KFOK", Location: [2]float32{-72.6318119, 40.8436186}},
				Airport{Name: "KBDR", Location: [2]float32{-73.1261758, 41.1634808}},
				Airport{Name: "KHVN", Location: [2]float32{-72.8877292, 41.2637247}},
			},
		},
		MajorAirport{
			Airport: Airport{
				Name:     "KLGA",
				Location: [2]float32{-73.87261, 40.77724},
			},
			Satellites: []Airport{
				Airport{Name: "KDXR", Location: [2]float32{-73.4821894, 41.3715344}},
				Airport{Name: "KHPN", Location: [2]float32{-73.7075661, 41.0669531}},
			},
		},
		MajorAirport{
			Airport: Airport{
				Name:     "KEWR",
				Location: [2]float32{-74.174538, 40.689491},
			},
			Satellites: []Airport{
				Airport{Name: "KTEB", Location: [2]float32{-74.0608333, 40.8501111}},
				Airport{Name: "KCDW", Location: [2]float32{-74.2813503, 40.8752247}},
				Airport{Name: "KMMU", Location: [2]float32{-74.4148886, 40.7993383}},
			},
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
			h, m := int(s.Hours()), int(s.Minutes())-60*int(s.Hours())
			fmt.Printf("%-9s %s (%d:%02d) | font=Monaco | href=https://nyartcc.org/controller/%d\n",
				ctrl.Callsign, ctrl.Name, h, m, ctrl.CID)
		}
	}

	// Traffic
	fmt.Printf("---\n")
	for _, major := range n90 {
		fmt.Printf("%s %2d🛫 %2d🛬 | font=Monaco | href=https://vatsim-radar.com/airport/%s\n", major.Name,
			n90dep[major.Name], n90arr[major.Name], major.Name)
	}
}

func CountTraffic(state *VATSIMState, airports []MajorAirport) (map[string]int, map[string]int, int) {
	major := func(ap string) *Airport { // return corresponding major
		for _, major := range airports {
			if major.Name == ap {
				return &major.Airport
			}
			for _, sat := range major.Satellites {
				if sat.Name == ap {
					return &major.Airport
				}
			}
		}
		return nil
	}

	dep, arr := make(map[string]int), make(map[string]int)
	count := 0
	for _, pilot := range state.Pilots {
		pilotLoc := [2]float32{pilot.Longitude, pilot.Latitude}

		// If it's >500nm from the first major (whatever it is), don't
		// consider it further.
		if airports[0].DistanceTo(pilotLoc) > 500 {
			continue
		}

		// Count departures that are within 10 miles of the departure field
		if major := major(pilot.FlightPlan.Departure); major != nil && major.DistanceTo(pilotLoc) < 30 {
			dep[major.Name] = dep[major.Name] + 1
			count++
		} else if pilot.Groundspeed < 20 && pilot.FlightPlan.Departure == "" {
		loop:
			// Look for aircraft without a flight plan on the ground at one of the airports.
			for _, major := range airports {
				if major.DistanceTo(pilotLoc) < 3 {
					dep[major.Name] = dep[major.Name] + 1
					count++
					break
				}

				for _, sat := range major.Satellites {
					if sat.DistanceTo(pilotLoc) < 3 {
						dep[major.Name] = dep[major.Name] + 1
						count++
						break loop
					}
				}
			}
		}

		// Arrivals within 300 miles but must also be moving
		if major := major(pilot.FlightPlan.Arrival); major != nil && major.DistanceTo(pilotLoc) < 300 && pilot.Groundspeed > 20 {
			arr[major.Name] = arr[major.Name] + 1
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
