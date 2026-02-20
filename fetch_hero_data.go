//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	openDotaBase = "https://api.opendota.com/api"
	heroesFile   = "data/heroes.json"
	matchupsFile = "data/matchups_all.json"
	requestDelay = 250 * time.Millisecond // ~4 req/s, well within 60/min limit
)

type Hero struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	LocalizedName string   `json:"localized_name"`
	PrimaryAttr   string   `json:"primary_attr"`
	AttackType    string   `json:"attack_type"`
	Roles         []string `json:"roles"`
}

type MatchupEntry struct {
	HeroID      int `json:"hero_id"`
	GamesPlayed int `json:"games_played"`
	Wins        int `json:"wins"`
}

func fetchJSON(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func main() {
	fmt.Println("=== Dota 2 Hero Data Fetcher ===")
	fmt.Printf("Source: %s\n\n", openDotaBase)

	// Step 1: Fetch all heroes
	fmt.Print("Fetching hero list... ")
	var heroes []Hero
	if err := fetchJSON(openDotaBase+"/heroes", &heroes); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d heroes)\n", len(heroes))

	// Save heroes.json
	heroesJSON, _ := json.MarshalIndent(heroes, "", "  ")
	if err := os.WriteFile(heroesFile, heroesJSON, 0644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", heroesFile, err)
		os.Exit(1)
	}
	fmt.Printf("Saved %s\n\n", heroesFile)

	// Step 2: Fetch matchups for each hero
	fmt.Printf("Fetching matchups for %d heroes (this takes ~%d seconds)...\n",
		len(heroes), len(heroes)*int(requestDelay/time.Second+1))

	allMatchups := make(map[int][]MatchupEntry)

	for i, hero := range heroes {
		fmt.Printf("  [%3d/%d] %s (ID: %d)... ", i+1, len(heroes), hero.LocalizedName, hero.ID)

		var matchups []MatchupEntry
		url := fmt.Sprintf("%s/heroes/%d/matchups", openDotaBase, hero.ID)
		if err := fetchJSON(url, &matchups); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			// Continue with other heroes instead of failing entirely
			time.Sleep(requestDelay)
			continue
		}

		allMatchups[hero.ID] = matchups
		fmt.Printf("OK (%d matchups)\n", len(matchups))

		time.Sleep(requestDelay)
	}

	// Save matchups_all.json
	matchupsJSON, _ := json.MarshalIndent(allMatchups, "", "  ")
	if err := os.WriteFile(matchupsFile, matchupsJSON, 0644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", matchupsFile, err)
		os.Exit(1)
	}

	fmt.Printf("\nSaved %s\n", matchupsFile)
	fmt.Printf("\n=== Done! %d heroes, %d matchup sets ===\n", len(heroes), len(allMatchups))
	fmt.Println("Run 'go build' to embed the data into the binary.")
}
