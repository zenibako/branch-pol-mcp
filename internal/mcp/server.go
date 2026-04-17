package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/anomalyco/branch-pol-mcp/internal/branch"
	mcpgolang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type Server struct {
	client *branch.Client
	server *mcpgolang.Server
}

func NewServer() *Server {
	client := branch.NewClient()
	transport := stdio.NewStdioServerTransport()
	s := mcpgolang.NewServer(transport,
		mcpgolang.WithName("branch-pol-mcp"),
		mcpgolang.WithVersion("1.0.0"),
		mcpgolang.WithInstructions(
			`Branch Politics MCP Server - Research local elections and candidates using branch.vote data.

This server provides tools to:
1. Look up elections and candidates by address
2. Browse elections by state
3. Research individual candidates (bios, issues, endorsements)
4. Build a virtual ballot with your choices

Supported states: AZ, GA, KY, MT, NC, ND, NY, OH, PA, TX, WA, WI`,
		),
	)

	server := &Server{
		client: client,
		server: s,
	}

	server.registerTools()
	return server
}

func (s *Server) Run() error {
	return s.server.Serve()
}

func mustRegister(s *mcpgolang.Server, name, description string, handler any) {
	if err := s.RegisterTool(name, description, handler); err != nil {
		log.Fatalf("failed to register tool %s: %v", name, err)
	}
}

func (s *Server) registerTools() {
	type EmptyArgs struct{}

	mustRegister(s.server, "list_states", "List all states supported by Branch.vote with their state codes", func(args EmptyArgs) (*mcpgolang.ToolResponse, error) {
		type stateInfo struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}
		var states []stateInfo
		codes := make([]string, 0, len(branch.SupportedStates))
		for code := range branch.SupportedStates {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			states = append(states, stateInfo{Code: strings.ToUpper(code), Name: branch.SupportedStates[code]})
		}
		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(mustJSON(states))), nil
	})

	type ListElectionsArgs struct {
		StateCode string `json:"state_code" jsonschema:"required,2-letter state code (e.g. GA, TX)"`
	}

	mustRegister(s.server, "list_elections", "List available elections for a state, including election dates, types, and number of races/candidates", func(args ListElectionsArgs) (*mcpgolang.ToolResponse, error) {
		stateCode := strings.ToLower(args.StateCode)
		if _, ok := branch.SupportedStates[stateCode]; !ok {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("State '%s' is not supported. Supported states: %s", args.StateCode, formatSupportedStates()))), nil
		}

		page, err := s.client.GetStateElections(stateCode)
		if err != nil {
			return nil, fmt.Errorf("fetching elections for state %s: %w", stateCode, err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Elections in %s\n\n", branch.SupportedStates[stateCode]))

		if page.SelectedElection.Key != "" {
			e := page.SelectedElection
			sb.WriteString(fmt.Sprintf("## Current Election: %s\n", e.Name))
			sb.WriteString(fmt.Sprintf("- **Date**: %s\n", branch.FormatDate(e.Date)))
			sb.WriteString(fmt.Sprintf("- **Type**: %s\n", formatElectionType(e.ElectionType)))
			sb.WriteString(fmt.Sprintf("- **Status**: %s\n", e.Status))
			sb.WriteString(fmt.Sprintf("- **Races**: %d\n", e.NumRaces))
			sb.WriteString(fmt.Sprintf("- **Candidates**: %d\n", e.NumCandidates))
			sb.WriteString(fmt.Sprintf("- **Parties**: %s\n", formatParties(e.PartiesPresent)))
			if e.EarlyVotingStart != "" {
				sb.WriteString(fmt.Sprintf("- **Early voting starts**: %s\n", branch.FormatDate(e.EarlyVotingStart)))
			}
			if e.EarlyVotingEnd != "" {
				sb.WriteString(fmt.Sprintf("- **Early voting ends**: %s\n", branch.FormatDate(e.EarlyVotingEnd)))
			}
			if e.AbsenteeEnd != "" {
				sb.WriteString(fmt.Sprintf("- **Absentee deadline**: %s\n", branch.FormatDate(e.AbsenteeEnd)))
			}
			if e.VoterRegistrationEnd != "" {
				sb.WriteString(fmt.Sprintf("- **Registration deadline**: %s\n", branch.FormatDate(e.VoterRegistrationEnd)))
			}
			sb.WriteString(fmt.Sprintf("- **Election key**: `%s`\n\n", e.Key))
		}

		if len(page.PreviousElections) > 0 {
			sb.WriteString("## Previous Elections\n\n")
			for _, e := range page.PreviousElections {
				sb.WriteString(fmt.Sprintf("- **%s** (Date: %s, Key: `%s`)\n", e.Name, branch.FormatDate(e.Date), e.Key))
			}
			sb.WriteString("\n")
		}

		if len(page.SelectedElection.CitiesPresent) > 0 {
			sb.WriteString(fmt.Sprintf("\n**Cities with election data**: %d cities available. Use `lookup_elections_by_city` to browse by city.\n", len(page.SelectedElection.CitiesPresent)))
		}

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type LookupElectionsByCityArgs struct {
		StateCode   string `json:"state_code" jsonschema:"required,2-letter state code (e.g. GA)"`
		ElectionKey string `json:"election_key" jsonschema:"required,election key from list_elections (e.g. 2026-georgia-primary-election)"`
		CityName    string `json:"city_name" jsonschema:"required,city name to search for (e.g. Atlanta)"`
	}

	mustRegister(s.server, "lookup_elections_by_city", "Find elections and races for a specific city within a state", func(args LookupElectionsByCityArgs) (*mcpgolang.ToolResponse, error) {
		stateCode := strings.ToLower(args.StateCode)
		if _, ok := branch.SupportedStates[stateCode]; !ok {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("State '%s' is not supported.", args.StateCode))), nil
		}

		statePage, err := s.client.GetStateElections(stateCode)
		if err != nil {
			return nil, fmt.Errorf("fetching state page: %w", err)
		}

		var matchName string
		var cityName string
		lower := strings.ToLower(args.CityName)
		for _, c := range statePage.SelectedElection.CitiesPresent {
			if strings.Contains(strings.ToLower(c.Name), lower) || strings.Contains(strings.ToLower(c.MatchName), lower) {
				matchName = c.MatchName
				cityName = c.Name
				break
			}
		}
		if matchName == "" {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("City '%s' not found. Try using `list_elections` to see available cities.", args.CityName))), nil
		}

		cityPage, err := s.client.GetCityElections(stateCode, args.ElectionKey, matchName)
		if err != nil {
			return nil, fmt.Errorf("fetching city elections: %w", err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Elections in %s, %s\n\n", cityName, branch.SupportedStates[stateCode]))
		sb.WriteString(fmt.Sprintf("Election: %s (Date: %s)\n\n", cityPage.SelectedElection.Name, branch.FormatDate(cityPage.SelectedElection.Date)))

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type LookupBallotArgs struct {
		StateCode   string `json:"state_code" jsonschema:"required,2-letter state code (e.g. GA)"`
		ElectionKey string `json:"election_key" jsonschema:"required,election key from list_elections (e.g. 2026-georgia-primary-election)"`
		Street      string `json:"street" jsonschema:"required,street address (e.g. 730 Peachtree St NE)"`
		PlaceID     string `json:"place_id" jsonschema:"Google Place ID for the address (from Google Maps geocoding). If unknown, use lookup_ballot_by_address instead.)"`
		Party       string `json:"party" jsonschema:"Party preference for primary elections: D (Democrat), R (Republican), or N (Nonpartisan/No preference)"`
	}

	mustRegister(s.server, "lookup_ballot", "Look up your personalized ballot based on your address using a Google Place ID. Returns all races that will appear on your ballot for the given election.", func(args LookupBallotArgs) (*mcpgolang.ToolResponse, error) {
		stateCode := strings.ToLower(args.StateCode)
		if _, ok := branch.SupportedStates[stateCode]; !ok {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("State '%s' is not supported. Supported states: %s", args.StateCode, formatSupportedStates()))), nil
		}

		ballot, err := s.client.GetBallot(args.Street, args.PlaceID)
		if err != nil {
			return nil, fmt.Errorf("looking up ballot: %w", err)
		}

		var sb strings.Builder
		sb.WriteString("# Your Ballot\n\n")
		sb.WriteString(fmt.Sprintf("**Address**: %s\n", ballot.Street))
		sb.WriteString(fmt.Sprintf("**Election**: %s (Date: %s)\n\n", ballot.SelectedElection.Name, branch.FormatDate(ballot.SelectedElection.Date)))

		if len(ballot.Districts) > 0 {
			sb.WriteString("## Your Districts\n\n")
			for _, d := range ballot.Districts {
				sb.WriteString(fmt.Sprintf("- %s (%s)\n", d.LongName, d.Type))
			}
			sb.WriteString("\n")
		}

		if len(ballot.Elections) > 0 {
			sb.WriteString("## Available Elections\n\n")
			for _, e := range ballot.Elections {
				sb.WriteString(fmt.Sprintf("- **%s** (Date: %s, Type: %s, Key: `%s`)\n", e.Name, branch.FormatDate(e.Date), formatElectionType(e.ElectionType), e.Key))
			}
		}

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type LookupBallotByAddressArgs struct {
		StateCode   string `json:"state_code" jsonschema:"required,2-letter state code (e.g. GA)"`
		ElectionKey string `json:"election_key" jsonschema:"required,election key from list_elections (e.g. 2026-georgia-primary-election)"`
		Address     string `json:"address" jsonschema:"required,full street address (e.g. 730 Peachtree St NE, Atlanta, GA 30308)"`
		Party       string `json:"party" jsonschema:"Party preference for primary elections: D (Democrat), R (Republican), or N (Nonpartisan)"`
	}

	mustRegister(s.server, "lookup_ballot_by_address", "Look up your personalized ballot using a street address. Use this if you don't have a Google Place ID. Returns all races and candidates matching your address and party preference.", func(args LookupBallotByAddressArgs) (*mcpgolang.ToolResponse, error) {
		stateCode := strings.ToLower(args.StateCode)
		if _, ok := branch.SupportedStates[stateCode]; !ok {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("State '%s' is not supported.", args.StateCode))), nil
		}

		allRaces, err := s.client.GetAllRaces(stateCode, args.ElectionKey)
		if err != nil {
			return nil, fmt.Errorf("fetching races: %w", err)
		}

		party := strings.ToUpper(args.Party)
		if party == "" {
			party = "N"
		}

		var filteredRaces []branch.Race
		for _, race := range allRaces {
			if len(race.CandidateSummary.Names) == 0 {
				continue
			}
			if party != "N" && party != "" && race.Party != "" && race.Party != party {
				continue
			}
			filteredRaces = append(filteredRaces, race)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Ballot for %s\n\n", args.Address))
		if party != "" && party != "N" {
			sb.WriteString(fmt.Sprintf("**Party filter**: %s primary\n\n", branch.PartyName(party)))
		}

		sb.WriteString(fmt.Sprintf("**Showing %d races**\n\n", len(filteredRaces)))

		for i, race := range filteredRaces {
			sb.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, race.LongName))
			sb.WriteString(fmt.Sprintf("- **Office**: %s\n", race.OfficeName))
			if race.DescriptionShort != "" {
				sb.WriteString(fmt.Sprintf("- **Description**: %s\n", race.DescriptionShort))
			}
			sb.WriteString(fmt.Sprintf("- **Race key**: `%s`\n", race.RaceKey))
			sb.WriteString(fmt.Sprintf("- **Candidates**: %d\n", race.CandidateSummary.NumCandidates))

			for j, name := range race.CandidateSummary.Names {
				party := ""
				if j < len(race.CandidateSummary.Parties) {
					party = fmt.Sprintf(" (%s)", branch.PartyName(race.CandidateSummary.Parties[j]))
				}
				key := ""
				if j < len(race.CandidateSummary.Keys) {
					key = fmt.Sprintf(" [key: `%s`]", race.CandidateSummary.Keys[j])
				}
				sb.WriteString(fmt.Sprintf("  - %s%s%s\n", name, party, key))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("\n*Use `research_candidate` to get detailed information about a specific candidate, or `list_race_details` to see all candidates in a specific race.*\n")

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type ListRaceCandidatesArgs struct {
		StateCode   string `json:"state_code" jsonschema:"required,2-letter state code (e.g. GA)"`
		ElectionKey string `json:"election_key" jsonschema:"required,election key (e.g. 2026-georgia-primary-election)"`
		Party       string `json:"party" jsonschema:"Party filter for primary elections: D (Democrat), R (Republican), N (Nonpartisan). Leave empty for all."`
		Page        int    `json:"page" jsonschema:"Page number (0-based) for paginated results. Each page has 10 races."`
		Search      string `json:"search" jsonschema:"Optional search term to filter races by office name (e.g. 'Sheriff', 'Mayor', 'School Board')"`
	}

	mustRegister(s.server, "list_race_candidates", "List races and their candidates for an election. Returns paginated results with candidate summaries. Use search to filter by office name.", func(args ListRaceCandidatesArgs) (*mcpgolang.ToolResponse, error) {
		stateCode := strings.ToLower(args.StateCode)
		if _, ok := branch.SupportedStates[stateCode]; !ok {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("State '%s' is not supported.", args.StateCode))), nil
		}

		if args.Page < 0 {
			args.Page = 0
		}

		result, err := s.client.GetRaces(stateCode, args.ElectionKey, args.Page)
		if err != nil {
			return nil, fmt.Errorf("fetching races: %w", err)
		}

		party := strings.ToUpper(args.Party)
		var filteredRaces []branch.Race
		for _, race := range result.Races {
			if party != "" && party != "N" && race.Party != "" && race.Party != party {
				continue
			}
			if args.Search != "" {
				lower := strings.ToLower(args.Search)
				if !strings.Contains(strings.ToLower(race.OfficeName), lower) && !strings.Contains(strings.ToLower(race.LongName), lower) {
					continue
				}
			}
			filteredRaces = append(filteredRaces, race)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Races in %s (Page %d, Total: %d)\n\n", args.ElectionKey, args.Page, result.RacesTotal))

		for i, race := range filteredRaces {
			sb.WriteString(fmt.Sprintf("## %d. %s\n\n", (args.Page*10)+i+1, race.LongName))
			sb.WriteString(fmt.Sprintf("- **Office**: %s\n", race.OfficeName))
			if race.DescriptionShort != "" {
				sb.WriteString(fmt.Sprintf("- **What it does**: %s\n", race.DescriptionShort))
			}
			if len(race.ImpactIssues) > 0 {
				sb.WriteString("- **Impact areas**: ")
				issues := make([]string, 0, len(race.ImpactIssues))
				for _, iss := range race.ImpactIssues {
					issues = append(issues, iss.Name)
				}
				sb.WriteString(strings.Join(issues, ", ") + "\n")
			}
			sb.WriteString(fmt.Sprintf("- **Candidates**: %d\n", race.CandidateSummary.NumCandidates))
			for j, name := range race.CandidateSummary.Names {
				partyStr := ""
				if j < len(race.CandidateSummary.Parties) {
					partyStr = fmt.Sprintf(" (%s)", branch.PartyName(race.CandidateSummary.Parties[j]))
				}
				key := ""
				if j < len(race.CandidateSummary.Keys) {
					key = fmt.Sprintf(" [key: `%s`]", race.CandidateSummary.Keys[j])
				}
				sb.WriteString(fmt.Sprintf("  - %s%s%s\n", name, partyStr, key))
			}
			sb.WriteString(fmt.Sprintf("- **Race key**: `%s`\n\n", race.RaceKey))
		}

		totalPages := (result.RacesTotal + 9) / 10
		sb.WriteString(fmt.Sprintf("\n---\n*Page %d of %d (showing %d of %d total races)*\n", args.Page+1, totalPages, len(filteredRaces), result.RacesTotal))
		if args.Page+1 < totalPages {
			sb.WriteString(fmt.Sprintf("*Use page=%d to see more races.*\n", args.Page+1))
		}

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type ResearchCandidateArgs struct {
		RaceKey       string `json:"race_key" jsonschema:"required,Race key (from list_race_candidates or lookup_ballot_by_address)"`
		CandidateSlug string `json:"candidate_slug" jsonschema:"required,Candidate slug/key (from list_race_candidates, e.g. 'daryl-l.-edwards')"`
	}

	mustRegister(s.server, "research_candidate", "Get detailed information about a specific candidate including their bio, positions on issues, endorsements, and contact information.", func(args ResearchCandidateArgs) (*mcpgolang.ToolResponse, error) {
		race, err := s.client.GetRace(args.RaceKey)
		if err != nil {
			return nil, fmt.Errorf("fetching race: %w", err)
		}

		candidate, err := s.client.GetCandidate(args.RaceKey, args.CandidateSlug)
		if err != nil {
			return nil, fmt.Errorf("fetching candidate: %w", err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", candidate.Name))

		party := branch.PartyName(candidate.Party)
		sb.WriteString(fmt.Sprintf("## Office: %s\n", race.LongName))
		sb.WriteString(fmt.Sprintf("- **Party**: %s\n", party))
		sb.WriteString(fmt.Sprintf("- **Status**: %s\n", candidate.Status))
		if candidate.Incumbent {
			sb.WriteString("- **Incumbent**: Yes\n")
		}
		if candidate.Withdrawn {
			sb.WriteString("- **Withdrawn**: Yes (may still appear on ballot)\n")
		}
		if candidate.Qualified != "" {
			sb.WriteString(fmt.Sprintf("- **Qualified**: %s\n", candidate.Qualified))
		}
		sb.WriteString(fmt.Sprintf("- **Profile completeness**: %.0f%%\n\n", candidate.Progress*100))

		if len(candidate.Bios) > 0 {
			sb.WriteString("## Biography\n\n")
			for _, bio := range candidate.Bios {
				if bio.Text != "" {
					label := "Background"
					switch bio.Type {
					case "personal":
						label = "Personal Background"
					case "political":
						label = "Political Background"
					case "professional":
						label = "Professional Background"
					}
					sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", label, bio.Text))
				}
			}
		}

		if len(candidate.Issues) > 0 {
			sb.WriteString("## Positions on Issues\n\n")
			for _, issue := range candidate.Issues {
				sb.WriteString(fmt.Sprintf("### %s\n", issue.Title))
				if issue.Text != "" {
					sb.WriteString(fmt.Sprintf("%s\n\n", issue.Text))
				}
				if issue.MissingData != "" && issue.MissingData != "no-response" {
					sb.WriteString(fmt.Sprintf("*%s*\n\n", issue.MissingData))
				}
				if issue.IsTopPriority {
					sb.WriteString("**Marked as top priority**\n\n")
				}
			}
		}

		if len(candidate.Contact) > 0 {
			sb.WriteString("## Contact Information\n\n")
			for _, c := range candidate.Contact {
				visible := ""
				if c.Visibility != "" && c.Visibility != "public" {
					visible = fmt.Sprintf(" (%s)", c.Visibility)
				}
				sb.WriteString(fmt.Sprintf("- **%s**: %s%s\n", c.Method, c.Value, visible))
			}
			sb.WriteString("\n")
		}

		if len(candidate.Links) > 0 {
			sb.WriteString("## Links\n\n")
			for _, link := range candidate.Links {
				sb.WriteString(fmt.Sprintf("- [%s](%s) (%s)\n", link.Title, link.URL, link.MediaType))
			}
			sb.WriteString("\n")
		}

		if candidate.References.Checked && len(candidate.References.Categories) > 0 {
			sb.WriteString("## Sources & References\n\n")
			for _, cat := range candidate.References.Categories {
				sb.WriteString(fmt.Sprintf("### %s\n", cat.Type))
				for _, src := range cat.Sources {
					sb.WriteString(fmt.Sprintf("- [%s](%s)\n", src.Title, src.URL))
				}
				if cat.Missing {
					sb.WriteString("*No sources found for this category.*\n")
				}
				sb.WriteString("\n")
			}
		}

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type ListRaceDetailsArgs struct {
		RaceKey string `json:"race_key" jsonschema:"required,Race key (from list_race_candidates, e.g. '2026-georgia-primary-election-ga-state-county-commissioner-ga-appling-county-district-3-r')"`
	}

	mustRegister(s.server, "list_race_details", "Get detailed information about a specific race including all candidates, impact areas, and what the office does.", func(args ListRaceDetailsArgs) (*mcpgolang.ToolResponse, error) {
		race, err := s.client.GetRace(args.RaceKey)
		if err != nil {
			return nil, fmt.Errorf("fetching race: %w", err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", race.LongName))

		sb.WriteString("## About This Office\n\n")
		if race.DescriptionShort != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", race.DescriptionShort))
		}
		if race.DescriptionLong != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", race.DescriptionLong))
		}

		sb.WriteString(fmt.Sprintf("- **Party**: %s\n", branch.PartyName(race.Party)))
		sb.WriteString(fmt.Sprintf("- **District type**: %s\n", race.DistrictType))
		sb.WriteString(fmt.Sprintf("- **Max choices**: %d\n", race.MaxChoices))
		if race.Retention {
			sb.WriteString("- **Retention election**: Yes (vote yes/no to retain)\n")
		}
		if race.Uncontested {
			sb.WriteString("- **Uncontested**: Yes\n")
		}
		sb.WriteString("\n")

		if len(race.ImpactIssues) > 0 {
			sb.WriteString("## How This Office Impacts You\n\n")
			for _, issue := range race.ImpactIssues {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", issue.Name, issue.How))
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("## Candidates (%d running)\n\n", len(race.Candidates)))
		for _, c := range race.Candidates {
			party := branch.PartyName(c.Party)
			incumbentStr := ""
			if c.Incumbent {
				incumbentStr = " (Incumbent)"
			}
			withdrawn := ""
			if c.Withdrawn {
				withdrawn = " [WITHDRAWN]"
			}
			progress := fmt.Sprintf("%.0f%%", c.Progress*100)
			sb.WriteString(fmt.Sprintf("- **%s** (%s)%s%s - Profile: %s [key: `%s`]\n", c.Name, party, incumbentStr, withdrawn, progress, c.Official))
		}
		sb.WriteString(fmt.Sprintf("\n*Use `research_candidate` with race_key=`%s` and candidate_slug to get detailed information about a candidate.*\n", race.RaceKey))

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})

	type BallotChoice struct {
		RaceKey       string `json:"race_key"`
		OfficeName    string `json:"office_name"`
		CandidateKey  string `json:"candidate_key"`
		CandidateName string `json:"candidate_name"`
		Party         string `json:"party"`
	}

	type FillBallotArgs struct {
		StateCode   string         `json:"state_code" jsonschema:"required,2-letter state code (e.g. GA)"`
		ElectionKey string         `json:"election_key" jsonschema:"required,election key (e.g. 2026-georgia-primary-election)"`
		Party       string         `json:"party" jsonschema:"Your party preference for primary elections: D (Democrat), R (Republican), N (Nonpartisan)"`
		Choices     []BallotChoice `json:"choices" jsonschema:"required,List of your candidate choices. Each choice must include race_key, candidate_key (the candidate slug), and candidate_name."`
	}

	mustRegister(s.server, "fill_ballot", "Create a virtual ballot by selecting your preferred candidates for each race. Returns a formatted ballot with your choices. This ballot is local-only and not saved to Branch.vote — it is for your personal reference only. You must still cast your official vote at your polling place or via absentee ballot.", func(args FillBallotArgs) (*mcpgolang.ToolResponse, error) {
		stateCode := strings.ToLower(args.StateCode)
		if _, ok := branch.SupportedStates[stateCode]; !ok {
			return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("State '%s' is not supported.", args.StateCode))), nil
		}

		var sb strings.Builder
		sb.WriteString("# Your Virtual Ballot\n\n")
		sb.WriteString("> **Note**: This ballot is local to this conversation and is not saved to or synced with Branch.vote. It is for your personal reference only — you must still cast your official vote at your polling place or via absentee ballot.\n\n")
		sb.WriteString(fmt.Sprintf("**Election**: %s\n", args.ElectionKey))
		sb.WriteString(fmt.Sprintf("**State**: %s\n", strings.ToUpper(stateCode)))
		if args.Party != "" {
			sb.WriteString(fmt.Sprintf("**Party**: %s\n", branch.PartyName(args.Party)))
		}
		sb.WriteString("\n---\n\n")

		sb.WriteString("| # | Office | Your Choice | Party |\n")
		sb.WriteString("|---|--------|-------------|-------|\n")

		for i, choice := range args.Choices {
			party := branch.PartyName(choice.Party)
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, choice.OfficeName, choice.CandidateName, party))
		}

		sb.WriteString("\n---\n\n")
		sb.WriteString(fmt.Sprintf("**Total choices: %d**\n\n", len(args.Choices)))
		sb.WriteString("*This is a virtual ballot for your reference only. It is not stored on Branch.vote and does not constitute an official vote. You must cast your official vote at your polling place or via absentee ballot.*\n")

		return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(sb.String())), nil
	})
}

func mustJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(b)
}

func formatElectionType(t string) string {
	switch t {
	case "primary":
		return "Primary"
	case "general":
		return "General"
	case "general-runoff":
		return "General Runoff"
	case "special":
		return "Special"
	default:
		return t
	}
}

func formatParties(parties []string) string {
	if len(parties) == 0 {
		return "N/A"
	}
	names := make([]string, 0, len(parties))
	for _, p := range parties {
		names = append(names, branch.PartyName(p))
	}
	return strings.Join(names, ", ")
}

func formatSupportedStates() string {
	codes := make([]string, 0, len(branch.SupportedStates))
	for code := range branch.SupportedStates {
		codes = append(codes, strings.ToUpper(code))
	}
	sort.Strings(codes)
	return strings.Join(codes, ", ")
}
