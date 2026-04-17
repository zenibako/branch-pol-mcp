# Branch Politics MCP Server

A Model Context Protocol (MCP) server that provides access to local election data from [Branch Politics](https://www.branch.vote). Research candidates, explore elections, and build virtual ballots.

## Installation

```bash
go build -o branch-pol-mcp ./cmd/main.go
```

## Configuration

Add to your MCP client configuration (e.g., Claude Desktop, Cursor):

```json
{
  "mcpServers": {
    "branch-politics": {
      "command": "/path/to/branch-pol-mcp",
      "args": []
    }
  }
}
```

## Tools

### `list_states`
List all states supported by Branch.vote with their state codes.

**Supported states:** AZ, GA, KY, MT, NC, ND, NY, OH, PA, TX, WA, WI

### `list_elections`
List available elections for a state, including dates, types, and candidate counts.

**Parameters:**
- `state_code` (required): 2-letter state code (e.g., "GA", "TX")

### `lookup_elections_by_city`
Find elections and races for a specific city within a state.

**Parameters:**
- `state_code` (required): 2-letter state code
- `election_key` (required): Election key from `list_elections`
- `city_name` (required): City name to search for

### `lookup_ballot`
Look up your personalized ballot using a Google Place ID.

**Parameters:**
- `state_code` (required): 2-letter state code
- `election_key` (required): Election key from `list_elections`
- `street` (required): Street address
- `place_id`: Google Place ID (optional)
- `party`: Party filter for primaries ("D", "R", "N")

### `lookup_ballot_by_address`
Look up ballot information using a street address (no Google Place ID needed). Returns all races and candidates for the state election.

**Parameters:**
- `state_code` (required): 2-letter state code
- `election_key` (required): Election key from `list_elections`
- `address` (required): Full street address
- `party`: Party filter for primaries

### `list_race_candidates`
List races and their candidates for an election with pagination and search.

**Parameters:**
- `state_code` (required): 2-letter state code
- `election_key` (required): Election key
- `party`: Party filter ("D", "R", "N")
- `page`: Page number (0-based, 10 races per page)
- `search`: Filter by office name (e.g., "Sheriff", "Mayor")

### `list_race_details`
Get detailed information about a specific race including office description, impact areas, and all candidates.

**Parameters:**
- `race_key` (required): Race key from `list_race_candidates` or `lookup_ballot_by_address`

### `research_candidate`
Get detailed candidate information including biography, positions on issues, endorsements, and contact info.

**Parameters:**
- `race_key` (required): Race key
- `candidate_slug` (required): Candidate slug from race listings

### `fill_ballot`
Create a virtual ballot with your candidate selections.

**Parameters:**
- `state_code` (required): 2-letter state code
- `election_key` (required): Election key
- `party`: Your party preference
- `choices` (required): Array of selections with `race_key`, `office_name`, `candidate_key`, `candidate_name`, `party`

## Example Workflow

1. **Find your state's elections:**
   ```
   list_elections(state_code="GA")
   ```

2. **Browse races in the election:**
   ```
   list_race_candidates(state_code="GA", election_key="2026-georgia-primary-election", search="Sheriff")
   ```

3. **Get details on a specific race:**
   ```
   list_race_details(race_key="2026-georgia-primary-election-ga-state-sheriff-ga-fulton-county-d")
   ```

4. **Research a candidate:**
   ```
   research_candidate(race_key="2026-georgia-primary-election-ga-state-sheriff-ga-fulton-county-d", candidate_slug="john-smith")
   ```

5. **Fill your virtual ballot:**
   ```
   fill_ballot(state_code="GA", election_key="2026-georgia-primary-election", choices=[...])
   ```

## Data Source

All election data is sourced from [Branch Politics](https://www.branch.vote), a free, nonpartisan website that helps voters make informed decisions about local elections.

## ⚠️ Important Disclaimers

**UNOFFICIAL SOURCE**: Branch.vote is not an official public resource. Information such as election dates, voting locations, and ballot items may not be 100% accurate. Always double-check any information against official sources like your state's Secretary of State website or your official sample ballot.

**NOT A BALLOT**: The `fill_ballot` tool creates a local virtual ballot for personal reference only. It is not saved to Branch.vote, does not constitute an official vote, and is not transmitted anywhere. You must cast your official vote at your polling place or via absentee ballot.

**INTELLECTUAL PROPERTY**: Branch.vote's Content and Marks are the property of Branch Chat PBC. This tool accesses publicly available pages for **personal, non-commercial use** only. You may not use this data to create databases, compilations, or commercial products without written permission from Branch Chat PBC.

**NO SCRAPING WARRANTY**: This server relies on Branch.vote's publicly accessible Next.js data endpoints, which may change structure or become unavailable without notice. The server may break if the site is restructured.

**ACCURACY**: Per Branch.vote's Terms of Use: *"We cannot guarantee that the information on our website, such as election dates, voting locations, and items on the ballot, is 100% accurate."* Verify all information with official election authorities before making voting decisions.

For the complete terms, see [branch.vote/termsofuse](https://www.branch.vote/termsofuse).

## License

MIT