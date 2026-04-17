package branch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	BaseURL     = "https://www.branch.vote"
	BuildID     = "ZUGd1QmJpeUpqZeIAr5IP"
	Org         = "branch"
	Lang        = "en"
	HTTPTimeout = 30 * time.Second
)

type Client struct {
	http    *http.Client
	baseURL string
	buildID string
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: HTTPTimeout,
		},
		baseURL: BaseURL,
		buildID: BuildID,
	}
}

func (c *Client) dataURL(path string, params url.Values) string {
	if params == nil {
		params = url.Values{}
	}
	params.Set("org", Org)
	params.Set("lng", Lang)
	return fmt.Sprintf("%s/_next/data/%s/%s?%s", c.baseURL, c.buildID, path, params.Encode())
}

func (c *Client) fetchJSON(dataURL string, target interface{}) error {
	req, err := http.NewRequest("GET", dataURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "branch-pol-mcp/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		PageProps json.RawMessage `json:"pageProps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if err := json.Unmarshal(wrapper.PageProps, target); err != nil {
		return fmt.Errorf("decoding page props: %w", err)
	}

	return nil
}

type State struct {
	ID             string `json:"_id"`
	Type           string `json:"type"`
	MatchName      string `json:"matchName"`
	Name           string `json:"name"`
	LongName       string `json:"longName"`
	ShortName      string `json:"shortName"`
	Parent         string `json:"parent"`
	ParentID       string `json:"parentId"`
	Population     int    `json:"population"`
	PopulationType string `json:"populationType"`
}

type ImpactIssue struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	How  string `json:"how"`
}

type Office struct {
	ID               string        `json:"_id"`
	Key              string        `json:"key"`
	Name             string        `json:"name"`
	DescriptionShort string        `json:"descriptionShort"`
	DescriptionLong  string        `json:"descriptionLong"`
	ImpactIssues     []ImpactIssue `json:"impactIssues"`
	DistrictTypes    []string      `json:"districtTypes"`
	BallotRanking    int           `json:"ballotRanking"`
	VoterVoice       string        `json:"voterVoice"`
}

type CandidateSummary struct {
	NumCandidates             int      `json:"numCandidates"`
	NumCandidatesNotQualified int      `json:"numCandidatesNotQualified"`
	Names                     []string `json:"names"`
	Keys                      []string `json:"keys"`
	IDs                       []string `json:"ids"`
	Photos                    []string `json:"photos"`
	Parties                   []string `json:"parties"`
}

type Race struct {
	ID               string           `json:"_id"`
	RaceKey          string           `json:"raceKey"`
	Election         json.RawMessage  `json:"election"`
	Office           json.RawMessage  `json:"office"`
	District         json.RawMessage  `json:"district"`
	DistrictType     string           `json:"districtType"`
	Party            string           `json:"party"`
	Cities           []string         `json:"cities"`
	Counties         []string         `json:"counties"`
	OfficeName       string           `json:"officeName"`
	LongName         string           `json:"longName"`
	DescriptionShort string           `json:"descriptionShort"`
	DescriptionLong  string           `json:"descriptionLong"`
	ImpactIssues     []ImpactIssue    `json:"impactIssues"`
	MaxChoices       int              `json:"maxChoices"`
	Retention        bool             `json:"retention"`
	Uncontested      bool             `json:"uncontested"`
	BallotOrder      int              `json:"ballotOrder"`
	CandidateSummary CandidateSummary `json:"candidateSummary"`
	Candidates       []Candidate      `json:"candidates"`
	CoverageStatus   string           `json:"coverageStatus"`
	CoverageProgress json.RawMessage  `json:"coverageProgress"`
	PriorityLevel    string           `json:"priorityLevel"`
	ExpectedReaders  int              `json:"expectedReaders"`
}

type Candidate struct {
	ID            string  `json:"_id"`
	Name          string  `json:"name"`
	Official      string  `json:"official"`
	Election      string  `json:"election"`
	Party         string  `json:"party"`
	RaceKey       string  `json:"raceKey"`
	RaceID        string  `json:"raceId"`
	Office        string  `json:"office"`
	DistrictType  string  `json:"districtType"`
	Qualified     string  `json:"qualified"`
	Withdrawn     bool    `json:"withdrawn"`
	Incumbent     bool    `json:"incumbent"`
	Status        string  `json:"status"`
	Progress      float64 `json:"progress"`
	PhotoPathFace string  `json:"photoPathFace"`
	BallotOrder   int     `json:"ballotOrder"`
}

type CandidateFull struct {
	ID            string        `json:"_id"`
	Name          string        `json:"name"`
	Official      string        `json:"official"`
	Election      string        `json:"election"`
	Party         string        `json:"party"`
	RaceKey       string        `json:"raceKey"`
	RaceID        string        `json:"raceId"`
	Office        string        `json:"office"`
	DistrictType  string        `json:"districtType"`
	Qualified     string        `json:"qualified"`
	Withdrawn     bool          `json:"withdrawn"`
	Incumbent     bool          `json:"incumbent"`
	Status        string        `json:"status"`
	Progress      float64       `json:"progress"`
	PhotoPathFace string        `json:"photoPathFace"`
	BallotOrder   int           `json:"ballotOrder"`
	Bios          []Bio         `json:"bios"`
	Issues        []Issue       `json:"issues"`
	Contact       []ContactInfo `json:"contact"`
	Links         []Link        `json:"links"`
	References    References    `json:"references"`
}

type Bio struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Complete bool   `json:"complete"`
}

type Issue struct {
	Key           string `json:"key"`
	Title         string `json:"title"`
	Text          string `json:"text"`
	Complete      bool   `json:"complete"`
	MissingData   string `json:"missingData"`
	IsTopPriority bool   `json:"isTopPriority"`
}

type ContactInfo struct {
	Method     string `json:"method"`
	Value      string `json:"value"`
	Visibility string `json:"visibility"`
}

type Link struct {
	MediaType string `json:"mediaType"`
	Title     string `json:"title"`
	URL       string `json:"url"`
}

type References struct {
	Checked      bool          `json:"checked"`
	TotalSources int           `json:"totalSources"`
	Categories   []RefCategory `json:"categories"`
}

type RefCategory struct {
	Type    string      `json:"type"`
	Sources []RefSource `json:"sources"`
	Missing bool        `json:"missing"`
}

type RefSource struct {
	MediaType string `json:"mediaType"`
	Title     string `json:"title"`
	URL       string `json:"url"`
}

type Election struct {
	ID                     string        `json:"_id"`
	Name                   string        `json:"name"`
	Key                    string        `json:"key"`
	Status                 string        `json:"status"`
	Active                 bool          `json:"active"`
	Date                   string        `json:"date"`
	Year                   int           `json:"year"`
	ElectionType           string        `json:"electionType"`
	PrimaryMode            string        `json:"primaryMode"`
	RunoffMode             string        `json:"runoffMode"`
	NonpartisanPrimaryMode string        `json:"nonpartisanPrimaryMode"`
	Partisan               bool          `json:"partisan"`
	StateCode              string        `json:"stateCode"`
	NumRaces               int           `json:"numRaces"`
	NumCandidates          int           `json:"numCandidates"`
	NumMeasures            int           `json:"numMeasures"`
	NumProfiles            int           `json:"numProfiles"`
	PartiesPresent         []string      `json:"partiesPresent"`
	OfficesPresent         []string      `json:"officesPresent"`
	CitiesPresent          []CityPresent `json:"citiesPresent"`
	AbsenteeEnd            string        `json:"absenteeEnd"`
	EarlyVotingEnd         string        `json:"earlyVotingEnd"`
	EarlyVotingStart       string        `json:"earlyVotingStart"`
	VoterRegistrationEnd   string        `json:"voterRegistrationEnd"`
}

type CityPresent struct {
	ID                   string `json:"_id"`
	Parent               string `json:"parent"`
	Type                 string `json:"type"`
	Name                 string `json:"name"`
	MatchName            string `json:"matchName"`
	TranslationAvailable bool   `json:"translationAvailable"`
}

type StateElectionsPage struct {
	SelectedElection  Election   `json:"selectedElection"`
	PreviousElections []Election `json:"previousElections"`
	State             State      `json:"state"`
}

type RacesPage struct {
	Races      []Race `json:"races"`
	RacesPage  string `json:"racesPage"`
	RacesTotal int    `json:"racesTotal"`
}

type CityElectionsPage struct {
	SelectedElection Election        `json:"selectedElection"`
	State            State           `json:"state"`
	City             json.RawMessage `json:"city"`
}

type RacePage struct {
	Race Race `json:"race"`
}

type CandidatePage struct {
	Candidate CandidateFull `json:"candidate"`
}

func (c *Client) GetStateElections(stateCode string) (*StateElectionsPage, error) {
	params := url.Values{}
	params.Set("stateCode", stateCode)
	var result struct {
		SelectedElection  json.RawMessage `json:"selectedElection"`
		PreviousElections json.RawMessage `json:"previousElections"`
		State             json.RawMessage `json:"state"`
	}
	err := c.fetchJSON(c.dataURL(fmt.Sprintf("elections/state/%s.json", stateCode), params), &result)
	if err != nil {
		return nil, err
	}

	page := &StateElectionsPage{}

	if len(result.SelectedElection) > 0 {
		if err := json.Unmarshal(result.SelectedElection, &page.SelectedElection); err != nil {
			page.SelectedElection = Election{}
		}
	}

	if len(result.PreviousElections) > 0 {
		if err := json.Unmarshal(result.PreviousElections, &page.PreviousElections); err != nil {
			page.PreviousElections = []Election{}
		}
	}

	if len(result.State) > 0 {
		if err := json.Unmarshal(result.State, &page.State); err != nil {
			page.State = State{}
		}
	}

	return page, nil
}

func (c *Client) GetRaces(stateCode, electionKey string, page int) (*RacesPage, error) {
	params := url.Values{}
	params.Set("stateCode", stateCode)
	params.Set("racesPage", strconv.Itoa(page))
	var result RacesPage
	err := c.fetchJSON(c.dataURL(fmt.Sprintf("elections/state/%s/%s/races.json", stateCode, electionKey), params), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAllRaces(stateCode, electionKey string) ([]Race, error) {
	var allRaces []Race
	page := 0
	for {
		result, err := c.GetRaces(stateCode, electionKey, page)
		if err != nil {
			return nil, err
		}
		if len(result.Races) == 0 {
			break
		}
		allRaces = append(allRaces, result.Races...)
		if len(allRaces) >= result.RacesTotal {
			break
		}
		page++
	}
	return allRaces, nil
}

func (c *Client) GetRace(raceKey string) (*Race, error) {
	params := url.Values{}
	var result RacePage
	err := c.fetchJSON(c.dataURL(fmt.Sprintf("races/%s.json", raceKey), params), &result)
	if err != nil {
		return nil, err
	}
	return &result.Race, nil
}

func (c *Client) GetCandidate(raceKey, candidateSlug string) (*CandidateFull, error) {
	params := url.Values{}
	var result CandidatePage
	err := c.fetchJSON(c.dataURL(fmt.Sprintf("races/%s/candidates/%s.json", raceKey, candidateSlug), params), &result)
	if err != nil {
		return nil, err
	}
	return &result.Candidate, nil
}

func (c *Client) GetCityElections(stateCode, electionKey, cityMatchName string) (*CityElectionsPage, error) {
	params := url.Values{}
	params.Set("stateCode", stateCode)
	var result CityElectionsPage
	err := c.fetchJSON(c.dataURL(fmt.Sprintf("elections/state/%s/%s/city/%s.json", stateCode, electionKey, cityMatchName), params), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type BallotDistrict struct {
	ID        string `json:"_id"`
	Type      string `json:"type"`
	MatchName string `json:"matchName"`
	Name      string `json:"name"`
	LongName  string `json:"longName"`
	ShortName string `json:"shortName"`
	Parent    string `json:"parent"`
}

type BallotPage struct {
	Elections        []Election       `json:"elections"`
	Districts        []BallotDistrict `json:"districts"`
	SelectedElection Election         `json:"selectedElection"`
	Street           string           `json:"street"`
}

func (c *Client) GetBallot(street, placeID string) (*BallotPage, error) {
	params := url.Values{}
	params.Set("street", street)
	params.Set("placeId", placeID)
	var result struct {
		Elections        json.RawMessage `json:"elections"`
		Districts        json.RawMessage `json:"districts"`
		SelectedElection json.RawMessage `json:"selectedElection"`
		Street           string          `json:"street"`
	}
	err := c.fetchJSON(c.dataURL("ballot.json", params), &result)
	if err != nil {
		return nil, err
	}

	page := &BallotPage{Street: result.Street}

	if len(result.Elections) > 0 {
		json.Unmarshal(result.Elections, &page.Elections)
	}
	if len(result.Districts) > 0 {
		json.Unmarshal(result.Districts, &page.Districts)
	}
	if len(result.SelectedElection) > 0 {
		json.Unmarshal(result.SelectedElection, &page.SelectedElection)
	}

	return page, nil
}

var SupportedStates = map[string]string{
	"az": "Arizona",
	"ga": "Georgia",
	"ky": "Kentucky",
	"mt": "Montana",
	"nc": "North Carolina",
	"nd": "North Dakota",
	"ny": "New York",
	"oh": "Ohio",
	"pa": "Pennsylvania",
	"tx": "Texas",
	"wa": "Washington",
	"wi": "Wisconsin",
}

func PartyName(code string) string {
	switch strings.ToUpper(code) {
	case "D":
		return "Democrat"
	case "R":
		return "Republican"
	case "N":
		return "Nonpartisan"
	case "G":
		return "Green"
	case "L":
		return "Libertarian"
	default:
		return code
	}
}

func FormatDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("January 2, 2006")
}
