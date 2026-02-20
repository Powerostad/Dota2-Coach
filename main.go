package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go/v3"
)

//go:embed data/heroes.json
var heroesJSON []byte

//go:embed data/matchups_all.json
var matchupsJSON []byte

// ---------------------------------------------------------------------------
// GSI Payload Structs
// ---------------------------------------------------------------------------

type GSIPayload struct {
	Provider  *Provider  `json:"provider,omitempty"`
	Map       *MapState  `json:"map,omitempty"`
	Player    *Player    `json:"player,omitempty"`
	Hero      *Hero      `json:"hero,omitempty"`
	Abilities *Abilities `json:"abilities,omitempty"`
	Items     *Items     `json:"items,omitempty"`
	Buildings *Buildings `json:"buildings,omitempty"`
	Draft     *Draft     `json:"draft,omitempty"`
}

type Provider struct {
	Name      string `json:"name"`
	AppID     int    `json:"appid"`
	Version   int    `json:"version"`
	Timestamp int64  `json:"timestamp"`
}

type MapState struct {
	Name                 string `json:"name"`
	MatchID              string `json:"matchid"`
	GameTime             int    `json:"game_time"`
	ClockTime            int    `json:"clock_time"`
	Daytime              bool   `json:"daytime"`
	NightstalkerNight    bool   `json:"nightstalker_night"`
	GameState            string `json:"game_state"`
	Paused               bool   `json:"paused"`
	WinTeam              string `json:"win_team"`
	CustomGameName       string `json:"customgamename"`
	WardPurchaseCooldown int    `json:"ward_purchase_cooldown"`
	RoshanState          string `json:"roshan_state"`
	RoshanStateEndSeconds int   `json:"roshan_state_end_seconds"`
	RadiantScore         int    `json:"radiant_score"`
	DireScore            int    `json:"dire_score"`
}

type Player struct {
	SteamID             string `json:"steamid"`
	Name                string `json:"name"`
	Activity            string `json:"activity"`
	Kills               int    `json:"kills"`
	Deaths              int    `json:"deaths"`
	Assists             int    `json:"assists"`
	LastHits            int    `json:"last_hits"`
	Denies              int    `json:"denies"`
	KillStreak          int    `json:"kill_streak"`
	CommandsIssued      int    `json:"commands_issued"`
	TeamName            string `json:"team_name"`
	Gold                int    `json:"gold"`
	GoldReliable        int    `json:"gold_reliable"`
	GoldUnreliable      int    `json:"gold_unreliable"`
	GoldFromHeroKills   int    `json:"gold_from_hero_kills"`
	GoldFromCreepKills  int    `json:"gold_from_creep_kills"`
	GoldFromIncome      int    `json:"gold_from_income"`
	GoldFromShared      int    `json:"gold_from_shared"`
	GPM                 int    `json:"gpm"`
	XPM                 int    `json:"xpm"`
	NetWorth            int    `json:"net_worth"`
	HeroDamage          int    `json:"hero_damage"`
	TowerDamage         int    `json:"tower_damage"`
	GoldLostToDeath     int    `json:"gold_lost_to_death"`
	GoldSpentOnBuybacks int    `json:"gold_spent_on_buybacks"`
	WardsPurchased      int    `json:"wards_purchased"`
	WardsPlaced         int    `json:"wards_placed"`
	WardsDestroyed      int    `json:"wards_destroyed"`
	CampsStacked        int    `json:"camps_stacked"`
}

type Hero struct {
	XPos            int    `json:"xpos"`
	YPos            int    `json:"ypos"`
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Level           int    `json:"level"`
	XP              int    `json:"xp"`
	Alive           bool   `json:"alive"`
	RespawnSeconds  int    `json:"respawn_seconds"`
	BuybackCost     int    `json:"buyback_cost"`
	BuybackCooldown int    `json:"buyback_cooldown"`
	Health          int    `json:"health"`
	MaxHealth       int    `json:"max_health"`
	HealthPercent   int    `json:"health_percent"`
	Mana            int    `json:"mana"`
	MaxMana         int    `json:"max_mana"`
	ManaPercent     int    `json:"mana_percent"`
	Silenced        bool   `json:"silenced"`
	Stunned         bool   `json:"stunned"`
	Disarmed        bool   `json:"disarmed"`
	MagicImmune     bool   `json:"magicimmune"`
	Hexed           bool   `json:"hexed"`
	Muted           bool   `json:"muted"`
	Break           bool   `json:"break"`
	AghanimsScepter bool   `json:"aghanims_scepter"`
	AghanimsShard   bool   `json:"aghanims_shard"`
	Smoked          bool   `json:"smoked"`
	HasDebuff       bool   `json:"has_debuff"`
}

// ---------------------------------------------------------------------------
// Ability GSI Structs
// ---------------------------------------------------------------------------

type Ability struct {
	Name           string `json:"name"`
	Level          int    `json:"level"`
	CanCast        bool   `json:"can_cast"`
	Passive        bool   `json:"passive"`
	AbilityActive  bool   `json:"ability_active"`
	Cooldown       int    `json:"cooldown"`
	Ultimate       bool   `json:"ultimate"`
	Charges        int    `json:"charges"`
	MaxCharges     int    `json:"max_charges"`
	ChargeCooldown int    `json:"charge_cooldown"`
}

type Abilities struct {
	Ability0 *Ability `json:"ability0,omitempty"`
	Ability1 *Ability `json:"ability1,omitempty"`
	Ability2 *Ability `json:"ability2,omitempty"`
	Ability3 *Ability `json:"ability3,omitempty"`
	Ability4 *Ability `json:"ability4,omitempty"`
	Ability5 *Ability `json:"ability5,omitempty"`
}

func (a *Abilities) All() []*Ability {
	if a == nil {
		return nil
	}
	return []*Ability{a.Ability0, a.Ability1, a.Ability2, a.Ability3, a.Ability4, a.Ability5}
}

func (a *Abilities) UltReady() (ready bool, level int) {
	if a == nil {
		return false, 0
	}
	for _, ab := range a.All() {
		if ab != nil && ab.Ultimate && ab.Level > 0 {
			return ab.CanCast && ab.Cooldown == 0, ab.Level
		}
	}
	return false, 0
}

// ---------------------------------------------------------------------------
// Item GSI Structs
// ---------------------------------------------------------------------------

type Item struct {
	Name         string `json:"name"`
	Purchaser    int    `json:"purchaser"`
	CanCast      bool   `json:"can_cast"`
	Cooldown     int    `json:"cooldown"`
	Passive      bool   `json:"passive"`
	ItemCharges  int    `json:"item_charges"`
	ContainsRune string `json:"contains_rune"`
	Charges      int    `json:"charges"`
}

type Items struct {
	Slot0     *Item `json:"slot0,omitempty"`
	Slot1     *Item `json:"slot1,omitempty"`
	Slot2     *Item `json:"slot2,omitempty"`
	Slot3     *Item `json:"slot3,omitempty"`
	Slot4     *Item `json:"slot4,omitempty"`
	Slot5     *Item `json:"slot5,omitempty"`
	Stash0    *Item `json:"stash0,omitempty"`
	Stash1    *Item `json:"stash1,omitempty"`
	Stash2    *Item `json:"stash2,omitempty"`
	Stash3    *Item `json:"stash3,omitempty"`
	Stash4    *Item `json:"stash4,omitempty"`
	Stash5    *Item `json:"stash5,omitempty"`
	Teleport0 *Item `json:"teleport0,omitempty"`
	Neutral0  *Item `json:"neutral0,omitempty"`
}

func (it *Items) InventoryNames() []string {
	if it == nil {
		return nil
	}
	var names []string
	for _, item := range []*Item{it.Slot0, it.Slot1, it.Slot2, it.Slot3, it.Slot4, it.Slot5} {
		if item != nil && item.Name != "" && item.Name != "empty" {
			names = append(names, item.Name)
		}
	}
	return names
}

func (it *Items) HasItem(name string) bool {
	if it == nil {
		return false
	}
	for _, item := range []*Item{it.Slot0, it.Slot1, it.Slot2, it.Slot3, it.Slot4, it.Slot5, it.Neutral0} {
		if item != nil && item.Name == name {
			return true
		}
	}
	return false
}

func (it *Items) HasTP() bool {
	if it == nil {
		return false
	}
	return it.Teleport0 != nil && it.Teleport0.Name != "" && it.Teleport0.Name != "empty"
}

// ---------------------------------------------------------------------------
// Building GSI Structs
// ---------------------------------------------------------------------------

type BuildingState struct {
	Health    int `json:"health"`
	MaxHealth int `json:"max_health"`
}

type TeamBuildings map[string]BuildingState

type Buildings struct {
	Radiant TeamBuildings `json:"radiant"`
	Dire    TeamBuildings `json:"dire"`
}

func towersDown(tb TeamBuildings) int {
	if tb == nil {
		return 0
	}
	count := 0
	for name, b := range tb {
		if strings.Contains(name, "tower") && b.Health == 0 && b.MaxHealth > 0 {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Draft GSI Structs
// ---------------------------------------------------------------------------

type DraftTeam struct {
	HomeTeam   bool   `json:"home_team"`
	Pick0ID    int    `json:"pick0_id"`
	Pick0Class string `json:"pick0_class"`
	Pick1ID    int    `json:"pick1_id"`
	Pick1Class string `json:"pick1_class"`
	Pick2ID    int    `json:"pick2_id"`
	Pick2Class string `json:"pick2_class"`
	Pick3ID    int    `json:"pick3_id"`
	Pick3Class string `json:"pick3_class"`
	Pick4ID    int    `json:"pick4_id"`
	Pick4Class string `json:"pick4_class"`
	Ban0ID     int    `json:"ban0_id"`
	Ban0Class  string `json:"ban0_class"`
	Ban1ID     int    `json:"ban1_id"`
	Ban1Class  string `json:"ban1_class"`
	Ban2ID     int    `json:"ban2_id"`
	Ban2Class  string `json:"ban2_class"`
	Ban3ID     int    `json:"ban3_id"`
	Ban3Class  string `json:"ban3_class"`
	Ban4ID     int    `json:"ban4_id"`
	Ban4Class  string `json:"ban4_class"`
	Ban5ID     int    `json:"ban5_id"`
	Ban5Class  string `json:"ban5_class"`
}

func (dt *DraftTeam) Picks() []int {
	var picks []int
	for _, id := range []int{dt.Pick0ID, dt.Pick1ID, dt.Pick2ID, dt.Pick3ID, dt.Pick4ID} {
		if id != 0 {
			picks = append(picks, id)
		}
	}
	return picks
}

func (dt *DraftTeam) Bans() []int {
	var bans []int
	for _, id := range []int{dt.Ban0ID, dt.Ban1ID, dt.Ban2ID, dt.Ban3ID, dt.Ban4ID, dt.Ban5ID} {
		if id != 0 {
			bans = append(bans, id)
		}
	}
	return bans
}

type Draft struct {
	ActiveTeam              int       `json:"activeteam"`
	Pick                    bool      `json:"pick"`
	ActiveTeamTimeRemaining float64   `json:"activeteam_time_remaining"`
	RadiantBonusTime        float64   `json:"radiant_bonus_time"`
	DireBonusTime           float64   `json:"dire_bonus_time"`
	Team2                   DraftTeam `json:"team2"` // Radiant
	Team3                   DraftTeam `json:"team3"` // Dire
}

// ---------------------------------------------------------------------------
// Hero Database (loaded from embedded OpenDota data)
// ---------------------------------------------------------------------------

type HeroData struct {
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

type HeroDatabase struct {
	Heroes       map[int]HeroData
	HeroesByName map[string]int
	Matchups     map[int]map[int]MatchupEntry // matchups[heroA][heroB]
	HeroList     []HeroData
}

func NewHeroDatabase() *HeroDatabase {
	db := &HeroDatabase{
		Heroes:       make(map[int]HeroData),
		HeroesByName: make(map[string]int),
		Matchups:     make(map[int]map[int]MatchupEntry),
	}

	// Parse heroes
	var heroes []HeroData
	if err := json.Unmarshal(heroesJSON, &heroes); err != nil {
		log.Printf("[HeroDB] Failed to parse heroes.json: %v", err)
		return db
	}
	db.HeroList = heroes
	for _, h := range heroes {
		db.Heroes[h.ID] = h
		db.HeroesByName[h.Name] = h.ID
	}

	// Parse matchups: JSON is map[string][]MatchupEntry (string keys because JSON)
	var rawMatchups map[string][]MatchupEntry
	if err := json.Unmarshal(matchupsJSON, &rawMatchups); err != nil {
		log.Printf("[HeroDB] Failed to parse matchups_all.json: %v", err)
		return db
	}
	for idStr, entries := range rawMatchups {
		var heroID int
		fmt.Sscanf(idStr, "%d", &heroID)
		if heroID == 0 {
			continue
		}
		db.Matchups[heroID] = make(map[int]MatchupEntry)
		for _, e := range entries {
			db.Matchups[heroID][e.HeroID] = e
		}
	}

	log.Printf("[HeroDB] Loaded %d heroes, %d matchup sets", len(db.Heroes), len(db.Matchups))
	return db
}

// ---------------------------------------------------------------------------
// Draft State (merges GSI data + manual input)
// ---------------------------------------------------------------------------

type DraftState struct {
	mu             sync.RWMutex
	Phase          string `json:"phase"`           // "none", "hero_selection", "strategy", "in_progress"
	PlayerTeam     string `json:"player_team"`     // "radiant" or "dire"
	RolePreference string `json:"role_preference"` // "carry", "mid", "offlane", "soft_support", "hard_support"
	AllyPicks      []int  `json:"ally_picks"`
	EnemyPicks     []int  `json:"enemy_picks"`
	AllBans        []int  `json:"all_bans"`
	UserPick       int    `json:"user_pick"`
}

func NewDraftState() *DraftState {
	return &DraftState{
		Phase:      "none",
		AllyPicks:  []int{},
		EnemyPicks: []int{},
		AllBans:    []int{},
	}
}

func (ds *DraftState) Snapshot() DraftState {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	cp := *ds
	cp.AllyPicks = append([]int{}, ds.AllyPicks...)
	cp.EnemyPicks = append([]int{}, ds.EnemyPicks...)
	cp.AllBans = append([]int{}, ds.AllBans...)
	return cp
}

func (ds *DraftState) Reset() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.Phase = "none"
	ds.RolePreference = ""
	ds.AllyPicks = []int{}
	ds.EnemyPicks = []int{}
	ds.AllBans = []int{}
	ds.UserPick = 0
}

func (ds *DraftState) AddPick(heroID int, isAlly bool) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if isAlly {
		if !containsInt(ds.AllyPicks, heroID) {
			ds.AllyPicks = append(ds.AllyPicks, heroID)
		}
	} else {
		if !containsInt(ds.EnemyPicks, heroID) {
			ds.EnemyPicks = append(ds.EnemyPicks, heroID)
		}
	}
}

func (ds *DraftState) RemovePick(heroID int, isAlly bool) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if isAlly {
		ds.AllyPicks = removeInt(ds.AllyPicks, heroID)
	} else {
		ds.EnemyPicks = removeInt(ds.EnemyPicks, heroID)
	}
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func removeInt(s []int, v int) []int {
	result := make([]int, 0, len(s))
	for _, x := range s {
		if x != v {
			result = append(result, x)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Hero Suggestion Algorithm
// ---------------------------------------------------------------------------

type HeroSuggestion struct {
	HeroID        int      `json:"hero_id"`
	LocalizedName string   `json:"localized_name"`
	Name          string   `json:"name"`
	Score         float64  `json:"score"`
	Reasons       []string `json:"reasons"`
	PrimaryAttr   string   `json:"primary_attr"`
	AttackType    string   `json:"attack_type"`
	Roles         []string `json:"roles"`
}

// roleMatchesPreference checks if a hero's roles match the user's role preference.
func roleMatchesPreference(roles []string, pref string) bool {
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[strings.ToLower(r)] = true
	}
	switch pref {
	case "carry":
		return roleSet["carry"]
	case "mid":
		return roleSet["nuker"] || roleSet["carry"]
	case "offlane":
		return roleSet["initiator"] || roleSet["durable"] || roleSet["disabler"]
	case "soft_support":
		return roleSet["support"] || roleSet["disabler"] || roleSet["initiator"]
	case "hard_support":
		return roleSet["support"]
	}
	return false
}

// coveredRoles returns a set of role categories covered by picked heroes.
func (db *HeroDatabase) coveredRoles(picks []int) map[string]bool {
	covered := make(map[string]bool)
	for _, id := range picks {
		hero, ok := db.Heroes[id]
		if !ok {
			continue
		}
		for _, r := range hero.Roles {
			covered[strings.ToLower(r)] = true
		}
	}
	return covered
}

func (db *HeroDatabase) SuggestHeroes(state *DraftState, limit int) []HeroSuggestion {
	snap := state.Snapshot()

	// Build exclusion set
	excluded := make(map[int]bool)
	for _, id := range snap.AllyPicks {
		excluded[id] = true
	}
	for _, id := range snap.EnemyPicks {
		excluded[id] = true
	}
	for _, id := range snap.AllBans {
		excluded[id] = true
	}
	if snap.UserPick != 0 {
		excluded[snap.UserPick] = true
	}

	allyCoveredRoles := db.coveredRoles(snap.AllyPicks)

	var suggestions []HeroSuggestion

	for _, hero := range db.HeroList {
		if excluded[hero.ID] {
			continue
		}

		var counterScore, roleScore, synergyScore, baseScore float64
		var reasons []string

		// --- Counter Score (weight: 0.40) ---
		if len(snap.EnemyPicks) > 0 {
			var totalWR float64
			var count int
			bestCounter := ""
			bestCounterWR := 0.0

			for _, enemyID := range snap.EnemyPicks {
				matchup, ok := db.Matchups[hero.ID][enemyID]
				if !ok || matchup.GamesPlayed < 50 {
					continue
				}
				wr := float64(matchup.Wins) / float64(matchup.GamesPlayed)
				totalWR += wr
				count++

				if wr > bestCounterWR {
					bestCounterWR = wr
					if enemy, ok := db.Heroes[enemyID]; ok {
						bestCounter = enemy.LocalizedName
					}
				}
			}

			if count > 0 {
				// Normalize: 0.50 WR = neutral (0.5 score), 0.55+ = good, 0.45- = bad
				avgWR := totalWR / float64(count)
				counterScore = math.Min(1.0, math.Max(0.0, (avgWR-0.35)/0.30))

				if bestCounterWR > 0.52 && bestCounter != "" {
					reasons = append(reasons, fmt.Sprintf("Strong vs %s (%.0f%%)", bestCounter, bestCounterWR*100))
				}
			}
		}

		// --- Role Preference Score (weight: 0.25) ---
		if snap.RolePreference != "" {
			if roleMatchesPreference(hero.Roles, snap.RolePreference) {
				roleScore = 1.0
				reasons = append(reasons, fmt.Sprintf("Fits %s role", snap.RolePreference))
			}
		}

		// --- Synergy / Role Coverage Score (weight: 0.25) ---
		if len(snap.AllyPicks) > 0 {
			heroRoles := make(map[string]bool)
			for _, r := range hero.Roles {
				heroRoles[strings.ToLower(r)] = true
			}
			// Score higher if hero fills uncovered roles
			importantRoles := []string{"carry", "support", "initiator", "durable", "nuker", "disabler", "pusher"}
			uncoveredFilled := 0
			for _, role := range importantRoles {
				if !allyCoveredRoles[role] && heroRoles[role] {
					uncoveredFilled++
				}
			}
			if uncoveredFilled > 0 {
				synergyScore = math.Min(1.0, float64(uncoveredFilled)/3.0)
				reasons = append(reasons, "Fills missing roles")
			} else {
				synergyScore = 0.3 // neutral if all roles already covered
			}
		}

		// --- Base Win Rate Score (weight: 0.10) ---
		if matchups, ok := db.Matchups[hero.ID]; ok {
			var totalWins, totalGames int
			for _, m := range matchups {
				totalWins += m.Wins
				totalGames += m.GamesPlayed
			}
			if totalGames > 0 {
				overallWR := float64(totalWins) / float64(totalGames)
				baseScore = math.Min(1.0, math.Max(0.0, (overallWR-0.40)/0.20))
			}
		}

		// --- Weighted Total ---
		hasEnemies := len(snap.EnemyPicks) > 0
		hasRole := snap.RolePreference != ""
		hasAllies := len(snap.AllyPicks) > 0

		var totalScore float64
		var totalWeight float64

		if hasEnemies {
			totalScore += counterScore * 0.40
			totalWeight += 0.40
		}
		if hasRole {
			totalScore += roleScore * 0.25
			totalWeight += 0.25
		}
		if hasAllies {
			totalScore += synergyScore * 0.25
			totalWeight += 0.25
		}
		totalScore += baseScore * 0.10
		totalWeight += 0.10

		// Normalize to 0-1 range
		if totalWeight > 0 {
			totalScore /= totalWeight
		}

		if len(reasons) == 0 {
			reasons = append(reasons, fmt.Sprintf("Win rate: %.0f%%", baseScore*20+40))
		}

		suggestions = append(suggestions, HeroSuggestion{
			HeroID:        hero.ID,
			LocalizedName: hero.LocalizedName,
			Name:          hero.Name,
			Score:         totalScore,
			Reasons:       reasons,
			PrimaryAttr:   hero.PrimaryAttr,
			AttackType:    hero.AttackType,
			Roles:         hero.Roles,
		})
	}

	// Sort by score descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	return suggestions
}

// ---------------------------------------------------------------------------
// Thread-Safe State Store
// ---------------------------------------------------------------------------

type GameStateStore struct {
	mu      sync.RWMutex
	current *GSIPayload
}

func (s *GameStateStore) Update(payload *GSIPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = payload
}

func (s *GameStateStore) Get() *GSIPayload {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// ---------------------------------------------------------------------------
// SSE Broker — Channel-Based Multi-Client Broadcaster
// ---------------------------------------------------------------------------

type SSEMessage struct {
	Event string // empty = default "message" event
	Data  string
}

type SSEBroker struct {
	clients    map[chan SSEMessage]bool
	register   chan chan SSEMessage
	unregister chan chan SSEMessage
	broadcast  chan SSEMessage
}

func NewSSEBroker() *SSEBroker {
	b := &SSEBroker{
		clients:    make(map[chan SSEMessage]bool),
		register:   make(chan chan SSEMessage),
		unregister: make(chan chan SSEMessage),
		broadcast:  make(chan SSEMessage, 8),
	}
	go b.run()
	return b
}

func (b *SSEBroker) Send(data string) {
	b.broadcast <- SSEMessage{Data: data}
}

func (b *SSEBroker) SendEvent(event, data string) {
	b.broadcast <- SSEMessage{Event: event, Data: data}
}

func (b *SSEBroker) run() {
	for {
		select {
		case client := <-b.register:
			b.clients[client] = true
			log.Printf("[SSE] Client connected (total: %d)", len(b.clients))
		case client := <-b.unregister:
			if _, ok := b.clients[client]; ok {
				delete(b.clients, client)
				close(client)
				log.Printf("[SSE] Client disconnected (total: %d)", len(b.clients))
			}
		case msg := <-b.broadcast:
			for client := range b.clients {
				select {
				case client <- msg:
				default:
					// Drop message for slow clients
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AI Client — OpenAI Wrapper
// ---------------------------------------------------------------------------

type AIClient struct {
	client  *openai.Client
	enabled bool
}

func NewAIClient() *AIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Println("[AI] No OPENAI_API_KEY set — AI coaching disabled, using rule-based tips only")
		return &AIClient{enabled: false}
	}
	client := openai.NewClient()
	log.Println("[AI] OpenAI client initialized (model: gpt-4o-mini)")
	return &AIClient{client: &client, enabled: true}
}

func (ai *AIClient) GetCoachingTip(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !ai.enabled {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	completion, err := ai.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:     "gpt-4o-mini",
		MaxTokens: openai.Int(80),
	})
	if err != nil {
		return "", err
	}
	if len(completion.Choices) > 0 {
		return completion.Choices[0].Message.Content, nil
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Coach Engine — Real-Time AI Coaching
// ---------------------------------------------------------------------------

const coachSystemPrompt = `You are a concise Dota 2 coach providing real-time advice. You see ONLY the player's own data (not enemies' items or health).

Rules:
- Give exactly ONE tip in 1-2 sentences max
- Be specific and actionable — reference actual numbers from the data (gold amounts, timings, item names)
- Sound like a knowledgeable friend, not a robot
- Never start with "Tip:" or "Coach:" — just say the advice directly
- Prioritize: survival > objectives > farming > optimization
- Adapt to game phase: early (0-15min) focus on laning and farming, mid (15-30min) focus on objectives and teamfights, late (30+min) focus on buyback management and highground
- If the player just died, briefly mention what to think about while dead (buyback? farming plan after respawn?)
- If a major item was just acquired, mention how it changes their play style`

type CoachSnapshot struct {
	HeroName      string
	HeroLocalName string
	HeroLevel     int
	ClockTime     int
	GamePhase     string // "early", "mid", "late"
	IsDaytime     bool
	HealthPct     int
	ManaPct       int
	Alive         bool
	RespawnSec    int
	BuybackCost   int
	BuybackReady  bool
	Gold          int
	GoldReliable  int
	NetWorth      int
	GPM           int
	XPM           int
	Kills         int
	Deaths        int
	Assists       int
	LastHits      int
	Denies        int
	ItemNames     []string
	HasTP         bool
	HasAegis      bool
	NeutralItem   string
	UltReady      bool
	UltLevel      int
	AllyTowersDown  int
	EnemyTowersDown int
	RoshanState   string
	TeamScore     int
	EnemyScore    int
	PlayerTeam    string
	HasScepter    bool
	HasShard      bool
}

func buildSnapshot(payload *GSIPayload, heroDB *HeroDatabase, playerTeam string) *CoachSnapshot {
	if payload == nil || payload.Map == nil || payload.Hero == nil {
		return nil
	}
	gs := payload.Map.GameState
	if gs != "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS" && gs != "DOTA_GAMERULES_STATE_PRE_GAME" {
		return nil
	}

	snap := &CoachSnapshot{
		ClockTime:  payload.Map.ClockTime,
		IsDaytime:  payload.Map.Daytime,
		RoshanState: payload.Map.RoshanState,
		PlayerTeam: playerTeam,
	}

	// Game phase
	if snap.ClockTime < 900 {
		snap.GamePhase = "early"
	} else if snap.ClockTime < 1800 {
		snap.GamePhase = "mid"
	} else {
		snap.GamePhase = "late"
	}

	// Hero
	h := payload.Hero
	snap.HeroName = h.Name
	snap.HeroLevel = h.Level
	snap.HealthPct = h.HealthPercent
	snap.ManaPct = h.ManaPercent
	snap.Alive = h.Alive
	snap.RespawnSec = h.RespawnSeconds
	snap.BuybackCost = h.BuybackCost
	snap.BuybackReady = h.BuybackCooldown == 0
	snap.HasScepter = h.AghanimsScepter
	snap.HasShard = h.AghanimsShard

	if hero, ok := heroDB.Heroes[h.ID]; ok {
		snap.HeroLocalName = hero.LocalizedName
	} else {
		snap.HeroLocalName = formatHeroNameGo(h.Name)
	}

	// Player
	if p := payload.Player; p != nil {
		snap.Gold = p.Gold
		snap.GoldReliable = p.GoldReliable
		snap.NetWorth = p.NetWorth
		snap.GPM = p.GPM
		snap.XPM = p.XPM
		snap.Kills = p.Kills
		snap.Deaths = p.Deaths
		snap.Assists = p.Assists
		snap.LastHits = p.LastHits
		snap.Denies = p.Denies
	}

	// Items
	if it := payload.Items; it != nil {
		snap.ItemNames = it.InventoryNames()
		snap.HasTP = it.HasTP()
		snap.HasAegis = it.HasItem("item_aegis")
		if it.Neutral0 != nil && it.Neutral0.Name != "" && it.Neutral0.Name != "empty" {
			snap.NeutralItem = it.Neutral0.Name
		}
	}

	// Abilities
	if ab := payload.Abilities; ab != nil {
		snap.UltReady, snap.UltLevel = ab.UltReady()
	}

	// Scores
	if playerTeam == "radiant" {
		snap.TeamScore = payload.Map.RadiantScore
		snap.EnemyScore = payload.Map.DireScore
	} else {
		snap.TeamScore = payload.Map.DireScore
		snap.EnemyScore = payload.Map.RadiantScore
	}

	// Buildings
	if b := payload.Buildings; b != nil {
		if playerTeam == "radiant" {
			snap.AllyTowersDown = towersDown(b.Radiant)
			snap.EnemyTowersDown = towersDown(b.Dire)
		} else {
			snap.AllyTowersDown = towersDown(b.Dire)
			snap.EnemyTowersDown = towersDown(b.Radiant)
		}
	}

	return snap
}

func formatHeroNameGo(name string) string {
	cleaned := strings.TrimPrefix(name, "npc_dota_hero_")
	words := strings.Split(cleaned, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// --- Game Events ---

type GameEvent struct {
	Type    string
	Details string
}

func detectEvents(prev, curr *CoachSnapshot) []GameEvent {
	if prev == nil {
		return nil
	}
	var events []GameEvent

	if prev.Alive && !curr.Alive {
		events = append(events, GameEvent{Type: "death", Details: fmt.Sprintf("Died at %d:%02d", curr.ClockTime/60, curr.ClockTime%60)})
	}
	if !prev.Alive && curr.Alive {
		events = append(events, GameEvent{Type: "respawn", Details: "Respawned"})
	}
	if curr.Kills > prev.Kills {
		events = append(events, GameEvent{Type: "kill", Details: fmt.Sprintf("Got a kill (%d total)", curr.Kills)})
	}
	if curr.HeroLevel > prev.HeroLevel {
		events = append(events, GameEvent{Type: "level_up", Details: fmt.Sprintf("Reached level %d", curr.HeroLevel)})
	}
	if !prev.UltReady && curr.UltReady {
		events = append(events, GameEvent{Type: "ult_ready", Details: "Ultimate is now ready"})
	}
	if prev.IsDaytime && !curr.IsDaytime {
		events = append(events, GameEvent{Type: "night_falling", Details: "Night has fallen"})
	}
	if prev.RoshanState != curr.RoshanState && curr.RoshanState != "" {
		events = append(events, GameEvent{Type: "roshan_change", Details: fmt.Sprintf("Roshan: %s", curr.RoshanState)})
	}
	if curr.EnemyTowersDown > prev.EnemyTowersDown {
		events = append(events, GameEvent{Type: "tower_destroyed", Details: "Enemy tower destroyed"})
	}
	if curr.AllyTowersDown > prev.AllyTowersDown {
		events = append(events, GameEvent{Type: "tower_destroyed", Details: "Ally tower destroyed"})
	}

	// New items
	prevItems := make(map[string]bool)
	for _, n := range prev.ItemNames {
		prevItems[n] = true
	}
	for _, n := range curr.ItemNames {
		if !prevItems[n] {
			events = append(events, GameEvent{Type: "item_acquired", Details: cleanItemName(n)})
		}
	}

	return events
}

func cleanItemName(name string) string {
	return strings.ReplaceAll(strings.TrimPrefix(name, "item_"), "_", " ")
}

// --- Coach Tips ---

type CoachTip struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Category  string    `json:"category"`
	Priority  int       `json:"priority"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// --- Coach Engine ---

type CoachEngine struct {
	aiClient         *AIClient
	heroDB           *HeroDatabase
	broker           *SSEBroker
	mu               sync.Mutex
	prevSnapshot     *CoachSnapshot
	lastAICall       time.Time
	lastTipByType    map[string]time.Time
	activeTips       []CoachTip
	tipHistory       []string
	playerTeam       string
	aiCooldown       time.Duration
	periodicInterval time.Duration
	tipDisplayTime   time.Duration
	tipCounter       int
}

func NewCoachEngine(aiClient *AIClient, heroDB *HeroDatabase, broker *SSEBroker) *CoachEngine {
	return &CoachEngine{
		aiClient:         aiClient,
		heroDB:           heroDB,
		broker:           broker,
		lastTipByType:    make(map[string]time.Time),
		aiCooldown:       15 * time.Second,
		periodicInterval: 45 * time.Second,
		tipDisplayTime:   12 * time.Second,
	}
}

func (ce *CoachEngine) Run(ctx context.Context, stateChan <-chan *GSIPayload) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var latestPayload *GSIPayload

	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-stateChan:
			latestPayload = payload
			// Update player team from payload
			if payload.Player != nil && payload.Player.TeamName != "" {
				ce.mu.Lock()
				ce.playerTeam = strings.ToLower(payload.Player.TeamName)
				ce.mu.Unlock()
			}
		case <-ticker.C:
			if latestPayload == nil {
				continue
			}

			ce.mu.Lock()
			team := ce.playerTeam
			ce.mu.Unlock()

			snapshot := buildSnapshot(latestPayload, ce.heroDB, team)
			if snapshot == nil {
				continue
			}

			ce.mu.Lock()

			// Detect events
			events := detectEvents(ce.prevSnapshot, snapshot)

			// Generate rule-based tips (free, instant)
			ruleTips := ce.generateRuleTips(snapshot, events)
			for _, tip := range ruleTips {
				ce.addTipLocked(tip)
			}

			// Decide whether to call AI
			if ce.shouldCallAI(snapshot, events) {
				snap := *snapshot // copy
				evts := make([]GameEvent, len(events))
				copy(evts, events)
				history := make([]string, len(ce.tipHistory))
				copy(history, ce.tipHistory)
				ce.lastAICall = time.Now()
				ce.mu.Unlock()
				go ce.callAI(ctx, &snap, evts, history)
				ce.mu.Lock()
			}

			// Expire old tips
			ce.expireTipsLocked()

			// Broadcast
			tips := make([]CoachTip, len(ce.activeTips))
			copy(tips, ce.activeTips)

			ce.prevSnapshot = snapshot
			ce.mu.Unlock()

			ce.broadcastTips(tips)
		}
	}
}

func (ce *CoachEngine) shouldCallAI(snap *CoachSnapshot, events []GameEvent) bool {
	if !ce.aiClient.enabled {
		return false
	}
	if time.Since(ce.lastAICall) < ce.aiCooldown {
		return false
	}

	// High priority events — call immediately
	for _, e := range events {
		switch e.Type {
		case "death", "item_acquired", "roshan_change":
			return true
		}
	}

	// Medium priority — 30s cooldown
	if time.Since(ce.lastAICall) >= 30*time.Second {
		for _, e := range events {
			switch e.Type {
			case "level_up", "tower_destroyed", "night_falling":
				return true
			}
		}
	}

	// Periodic
	if time.Since(ce.lastAICall) >= ce.periodicInterval {
		return true
	}

	return false
}

func (ce *CoachEngine) callAI(ctx context.Context, snap *CoachSnapshot, events []GameEvent, history []string) {
	userPrompt := ce.buildUserPrompt(snap, events, history)
	tip, err := ce.aiClient.GetCoachingTip(ctx, coachSystemPrompt, userPrompt)
	if err != nil {
		log.Printf("[AI] Error: %v", err)
		return
	}
	if tip == "" {
		return
	}

	ce.mu.Lock()
	ce.addTipLocked(CoachTip{
		Text:     tip,
		Category: "ai",
		Priority: 2,
		Source:   "ai",
	})
	tips := make([]CoachTip, len(ce.activeTips))
	copy(tips, ce.activeTips)
	ce.mu.Unlock()

	ce.broadcastTips(tips)
}

func (ce *CoachEngine) buildUserPrompt(snap *CoachSnapshot, events []GameEvent, history []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Hero: %s (Level %d)\n", snap.HeroLocalName, snap.HeroLevel))

	min := snap.ClockTime / 60
	sec := snap.ClockTime % 60
	if snap.ClockTime < 0 {
		min = (-snap.ClockTime) / 60
		sec = (-snap.ClockTime) % 60
		sb.WriteString(fmt.Sprintf("Time: -%d:%02d (pre-game)\n", min, sec))
	} else {
		dn := "day"
		if !snap.IsDaytime {
			dn = "night"
		}
		sb.WriteString(fmt.Sprintf("Time: %d:%02d (%s game, %s)\n", min, sec, snap.GamePhase, dn))
	}

	sb.WriteString(fmt.Sprintf("K/D/A: %d/%d/%d | LH/DN: %d/%d\n", snap.Kills, snap.Deaths, snap.Assists, snap.LastHits, snap.Denies))
	sb.WriteString(fmt.Sprintf("Gold: %d (reliable: %d) | GPM: %d | Net Worth: %d\n", snap.Gold, snap.GoldReliable, snap.GPM, snap.NetWorth))
	sb.WriteString(fmt.Sprintf("HP: %d%% | Mana: %d%% | Alive: %v\n", snap.HealthPct, snap.ManaPct, snap.Alive))

	if !snap.Alive {
		sb.WriteString(fmt.Sprintf("Respawn in: %ds | Buyback cost: %d | Buyback ready: %v | Gold: %d\n", snap.RespawnSec, snap.BuybackCost, snap.BuybackReady, snap.Gold))
	}

	if len(snap.ItemNames) > 0 {
		cleaned := make([]string, len(snap.ItemNames))
		for i, n := range snap.ItemNames {
			cleaned[i] = cleanItemName(n)
		}
		sb.WriteString(fmt.Sprintf("Items: %s\n", strings.Join(cleaned, ", ")))
	} else {
		sb.WriteString("Items: none\n")
	}

	sb.WriteString(fmt.Sprintf("Has TP: %v | Ult ready: %v (lv %d)\n", snap.HasTP, snap.UltReady, snap.UltLevel))

	if snap.HasAegis {
		sb.WriteString("Has Aegis!\n")
	}
	if snap.HasScepter {
		sb.WriteString("Has Aghanim's Scepter\n")
	}
	if snap.HasShard {
		sb.WriteString("Has Aghanim's Shard\n")
	}

	sb.WriteString(fmt.Sprintf("Score: Team %d - Enemy %d\n", snap.TeamScore, snap.EnemyScore))

	if snap.RoshanState != "" {
		sb.WriteString(fmt.Sprintf("Roshan: %s\n", snap.RoshanState))
	}

	sb.WriteString(fmt.Sprintf("Towers lost: us %d, them %d\n", snap.AllyTowersDown, snap.EnemyTowersDown))

	if len(events) > 0 {
		sb.WriteString("\nRecent events:\n")
		for _, e := range events {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Type, e.Details))
		}
	}

	if len(history) > 0 {
		start := len(history) - 5
		if start < 0 {
			start = 0
		}
		sb.WriteString(fmt.Sprintf("\nDo NOT repeat these recent tips: %s\n", strings.Join(history[start:], " | ")))
	}

	return sb.String()
}

// --- Rule-Based Tips ---

func (ce *CoachEngine) generateRuleTips(snap *CoachSnapshot, events []GameEvent) []CoachTip {
	var tips []CoachTip

	// No TP scroll
	if !snap.HasTP && snap.Alive && snap.ClockTime > 0 {
		if ce.tipCooldownOK("no_tp", 60*time.Second) {
			tips = append(tips, CoachTip{Text: "No TP scroll! Buy one immediately.", Category: "items", Priority: 1, Source: "rule"})
			ce.lastTipByType["no_tp"] = time.Now()
		}
	}

	// Dying too much early game
	if snap.Deaths >= 3 && snap.ClockTime < 600 && snap.ClockTime > 0 {
		if ce.tipCooldownOK("dying_early", 120*time.Second) {
			tips = append(tips, CoachTip{
				Text:     fmt.Sprintf("%d deaths before 10 min — play safer, stay near tower.", snap.Deaths),
				Category: "survival", Priority: 1, Source: "rule",
			})
			ce.lastTipByType["dying_early"] = time.Now()
		}
	}

	// Buyback unaffordable in late game
	if snap.GamePhase == "late" && snap.Gold < snap.BuybackCost && snap.Alive && snap.BuybackCost > 0 {
		if ce.tipCooldownOK("buyback_poor", 90*time.Second) {
			tips = append(tips, CoachTip{
				Text:     fmt.Sprintf("Buyback costs %dg but you only have %dg — farm safely, don't risk dying.", snap.BuybackCost, snap.Gold),
				Category: "economy", Priority: 2, Source: "rule",
			})
			ce.lastTipByType["buyback_poor"] = time.Now()
		}
	}

	// Ultimate ready reminder
	if snap.UltReady && snap.UltLevel > 0 && snap.Alive {
		if ce.tipCooldownOK("ult_ready", 90*time.Second) {
			tips = append(tips, CoachTip{
				Text:     "Your ultimate is ready — look for a fight or gank opportunity.",
				Category: "abilities", Priority: 2, Source: "rule",
			})
			ce.lastTipByType["ult_ready"] = time.Now()
		}
	}

	// Has Aegis
	if snap.HasAegis {
		if ce.tipCooldownOK("aegis", 120*time.Second) {
			tips = append(tips, CoachTip{
				Text:     "You have Aegis — play aggressive, force fights or push objectives.",
				Category: "items", Priority: 2, Source: "rule",
			})
			ce.lastTipByType["aegis"] = time.Now()
		}
	}

	// Night falling warning (check if close to a 5-min boundary and daytime)
	if snap.IsDaytime && snap.ClockTime > 0 {
		// Night falls at 5:00, 15:00, 25:00... (every 10 min starting at 5)
		// Simplified: warn if clock_time mod 600 is between 270 and 300 (last 30s of day)
		cyclePos := snap.ClockTime % 600
		if cyclePos >= 270 && cyclePos <= 300 {
			if ce.tipCooldownOK("night_warning", 300*time.Second) {
				timeToNight := 300 - cyclePos
				tips = append(tips, CoachTip{
					Text:     fmt.Sprintf("Night falls in ~%ds — careful about reduced vision and ganks.", timeToNight),
					Category: "timing", Priority: 3, Source: "rule",
				})
				ce.lastTipByType["night_warning"] = time.Now()
			}
		}
	}

	// GPM benchmark warning
	if snap.ClockTime > 300 && snap.Alive { // after 5 min
		expectedGPM := 400 // baseline
		if snap.GamePhase == "mid" {
			expectedGPM = 450
		} else if snap.GamePhase == "late" {
			expectedGPM = 500
		}
		if snap.GPM > 0 && snap.GPM < expectedGPM-100 {
			if ce.tipCooldownOK("low_gpm", 120*time.Second) {
				tips = append(tips, CoachTip{
					Text:     fmt.Sprintf("GPM is %d (expected ~%d+) — focus on farming between fights.", snap.GPM, expectedGPM),
					Category: "economy", Priority: 3, Source: "rule",
				})
				ce.lastTipByType["low_gpm"] = time.Now()
			}
		}
	}

	return tips
}

func (ce *CoachEngine) tipCooldownOK(tipType string, cooldown time.Duration) bool {
	last, ok := ce.lastTipByType[tipType]
	if !ok {
		return true
	}
	return time.Since(last) >= cooldown
}

// --- Tip Management ---

func (ce *CoachEngine) addTipLocked(tip CoachTip) {
	ce.tipCounter++
	tip.ID = fmt.Sprintf("%s-%d", tip.Category, ce.tipCounter)
	tip.CreatedAt = time.Now()
	tip.ExpiresAt = time.Now().Add(ce.tipDisplayTime)

	// Dedup
	for _, existing := range ce.activeTips {
		if existing.Text == tip.Text {
			return
		}
	}

	// Max 3 active tips
	if len(ce.activeTips) >= 3 {
		worstIdx := 0
		for i, t := range ce.activeTips {
			if t.Priority > ce.activeTips[worstIdx].Priority {
				worstIdx = i
			}
		}
		if tip.Priority < ce.activeTips[worstIdx].Priority {
			ce.activeTips = append(ce.activeTips[:worstIdx], ce.activeTips[worstIdx+1:]...)
		} else {
			return
		}
	}

	ce.activeTips = append(ce.activeTips, tip)

	// Track for repetition prevention
	ce.tipHistory = append(ce.tipHistory, tip.Text)
	if len(ce.tipHistory) > 20 {
		ce.tipHistory = ce.tipHistory[len(ce.tipHistory)-20:]
	}
}

func (ce *CoachEngine) expireTipsLocked() {
	now := time.Now()
	active := ce.activeTips[:0]
	for _, tip := range ce.activeTips {
		if now.Before(tip.ExpiresAt) {
			active = append(active, tip)
		}
	}
	ce.activeTips = active
}

func (ce *CoachEngine) broadcastTips(tips []CoachTip) {
	data, _ := json.Marshal(tips)
	ce.broker.SendEvent("coach", string(data))
}

// ---------------------------------------------------------------------------
// HTTP Handlers
// ---------------------------------------------------------------------------

func handleGSIPost(store *GameStateStore, broker *SSEBroker, draftState *DraftState, heroDB *HeroDatabase, coachChan chan<- *GSIPayload) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload GSIPayload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		store.Update(&payload)

		// Update draft phase from game state
		if payload.Map != nil {
			draftState.mu.Lock()
			switch payload.Map.GameState {
			case "DOTA_GAMERULES_STATE_HERO_SELECTION":
				draftState.Phase = "hero_selection"
			case "DOTA_GAMERULES_STATE_STRATEGY_TIME":
				draftState.Phase = "strategy"
			case "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS", "DOTA_GAMERULES_STATE_PRE_GAME":
				if draftState.Phase == "hero_selection" || draftState.Phase == "strategy" {
					draftState.Phase = "in_progress"
				}
			}
			draftState.mu.Unlock()
		}

		// Detect player team
		if payload.Player != nil && payload.Player.TeamName != "" {
			draftState.mu.Lock()
			draftState.PlayerTeam = strings.ToLower(payload.Player.TeamName)
			draftState.mu.Unlock()
		}

		// Parse draft data if available (spectator-primary, but try anyway)
		if payload.Draft != nil {
			draftState.mu.Lock()
			if draftState.PlayerTeam == "radiant" {
				for _, id := range payload.Draft.Team2.Picks() {
					if !containsInt(draftState.AllyPicks, id) {
						draftState.AllyPicks = append(draftState.AllyPicks, id)
					}
				}
				for _, id := range payload.Draft.Team3.Picks() {
					if !containsInt(draftState.EnemyPicks, id) {
						draftState.EnemyPicks = append(draftState.EnemyPicks, id)
					}
				}
			} else if draftState.PlayerTeam == "dire" {
				for _, id := range payload.Draft.Team3.Picks() {
					if !containsInt(draftState.AllyPicks, id) {
						draftState.AllyPicks = append(draftState.AllyPicks, id)
					}
				}
				for _, id := range payload.Draft.Team2.Picks() {
					if !containsInt(draftState.EnemyPicks, id) {
						draftState.EnemyPicks = append(draftState.EnemyPicks, id)
					}
				}
			}
			allBans := append(payload.Draft.Team2.Bans(), payload.Draft.Team3.Bans()...)
			for _, id := range allBans {
				if !containsInt(draftState.AllBans, id) {
					draftState.AllBans = append(draftState.AllBans, id)
				}
			}
			draftState.mu.Unlock()
		}

		// Detect user's hero pick
		if payload.Hero != nil && payload.Hero.ID != 0 {
			draftState.mu.Lock()
			draftState.UserPick = payload.Hero.ID
			draftState.mu.Unlock()
		}

		// Broadcast GSI data
		data, err := json.Marshal(payload)
		if err == nil {
			broker.Send(string(data))
		}

		// Feed coach engine (non-blocking)
		select {
		case coachChan <- &payload:
		default:
		}

		// If in draft phase, also broadcast suggestions
		draftState.mu.RLock()
		phase := draftState.Phase
		draftState.mu.RUnlock()

		if phase == "hero_selection" || phase == "strategy" {
			suggestions := heroDB.SuggestHeroes(draftState, 10)
			sugData, _ := json.Marshal(suggestions)
			broker.SendEvent("draft", string(sugData))
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handleDashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

func handleSSE(broker *SSEBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		clientChan := make(chan SSEMessage, 4)
		broker.register <- clientChan

		defer func() {
			broker.unregister <- clientChan
		}()

		rc := http.NewResponseController(w)
		ctx := r.Context()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-clientChan:
				if !ok {
					return
				}
				if msg.Event != "" {
					fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, msg.Data)
				} else {
					fmt.Fprintf(w, "data: %s\n\n", msg.Data)
				}
				if err := rc.Flush(); err != nil {
					return
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	store := &GameStateStore{}
	broker := NewSSEBroker()
	heroDB := NewHeroDatabase()
	draftState := NewDraftState()
	aiClient := NewAIClient()

	// Coach engine
	coachChan := make(chan *GSIPayload, 4)
	coachEngine := NewCoachEngine(aiClient, heroDB, broker)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coachEngine.Run(ctx, coachChan)

	mux := http.NewServeMux()

	// GSI POST + Dashboard GET on root
	gsiHandler := handleGSIPost(store, broker, draftState, heroDB, coachChan)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleDashboard(w, r)
		case http.MethodPost:
			gsiHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/events", handleSSE(broker))

	// Hero list endpoint
	mux.HandleFunc("/api/heroes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(heroesJSON)
	})

	// Suggestions endpoint
	mux.HandleFunc("/api/suggestions", func(w http.ResponseWriter, r *http.Request) {
		suggestions := heroDB.SuggestHeroes(draftState, 10)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(suggestions)
	})

	// Draft state endpoint (GET = read, POST = reset)
	mux.HandleFunc("/api/draft/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snap := draftState.Snapshot()
		json.NewEncoder(w).Encode(snap)
	})

	// Draft input endpoints
	mux.HandleFunc("/api/draft/ally", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			HeroID int `json:"hero_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.HeroID == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodDelete {
			draftState.RemovePick(body.HeroID, true)
		} else {
			draftState.AddPick(body.HeroID, true)
		}
		broadcastSuggestions(heroDB, draftState, broker)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/draft/enemy", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			HeroID int `json:"hero_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.HeroID == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodDelete {
			draftState.RemovePick(body.HeroID, false)
		} else {
			draftState.AddPick(body.HeroID, false)
		}
		broadcastSuggestions(heroDB, draftState, broker)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/draft/role", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		draftState.mu.Lock()
		draftState.RolePreference = body.Role
		draftState.mu.Unlock()
		broadcastSuggestions(heroDB, draftState, broker)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/draft/team", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Team string `json:"team"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		draftState.mu.Lock()
		draftState.PlayerTeam = strings.ToLower(body.Team)
		draftState.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/draft/reset", func(w http.ResponseWriter, r *http.Request) {
		draftState.Reset()
		broadcastSuggestions(heroDB, draftState, broker)
		w.WriteHeader(http.StatusOK)
	})

	log.Println("==============================================")
	log.Println("  Dota 2 Coach Overlay — Running on :3000")
	log.Println("==============================================")
	log.Println("Dashboard : http://localhost:3000")
	log.Println("SSE Stream: http://localhost:3000/events")
	log.Println("GSI POST  : http://localhost:3000/")
	log.Printf("Heroes    : %d loaded", len(heroDB.Heroes))
	if aiClient.enabled {
		log.Println("AI Coach  : Enabled (GPT-4o-mini)")
	} else {
		log.Println("AI Coach  : Disabled (set OPENAI_API_KEY to enable)")
	}
	log.Println("Waiting for game state data...")

	if err := http.ListenAndServe(":3000", mux); err != nil {
		log.Fatal(err)
	}
}

func broadcastSuggestions(heroDB *HeroDatabase, draftState *DraftState, broker *SSEBroker) {
	suggestions := heroDB.SuggestHeroes(draftState, 10)
	sugData, _ := json.Marshal(suggestions)
	broker.SendEvent("draft", string(sugData))

	// Also broadcast draft state so frontend stays in sync
	snap := draftState.Snapshot()
	stateData, _ := json.Marshal(snap)
	broker.SendEvent("draftstate", string(stateData))
}

// ---------------------------------------------------------------------------
// Embedded HTML / CSS / JS
// ---------------------------------------------------------------------------

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Dota 2 Coach Overlay</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;900&family=JetBrains+Mono:wght@700&display=swap');

        * { font-family: 'Inter', sans-serif; }
        .font-mono-bold { font-family: 'JetBrains Mono', monospace; font-weight: 700; }

        body {
            background: #0a0e17;
            background-image:
                radial-gradient(ellipse at 20% 50%, rgba(16, 185, 129, 0.04) 0%, transparent 60%),
                radial-gradient(ellipse at 80% 50%, rgba(56, 189, 248, 0.04) 0%, transparent 60%);
        }

        .glass-card {
            background: rgba(15, 23, 42, 0.75);
            backdrop-filter: blur(12px);
            border: 1px solid rgba(51, 65, 85, 0.5);
        }

        .bar-track {
            background: rgba(30, 41, 59, 0.8);
            position: relative;
            overflow: hidden;
        }

        .bar-track::after {
            content: '';
            position: absolute;
            top: 0; left: -100%; width: 50%; height: 100%;
            background: linear-gradient(90deg, transparent, rgba(255,255,255,0.06), transparent);
            animation: shimmer 3s infinite;
        }

        @keyframes shimmer {
            100% { left: 150%; }
        }

        .bar-fill {
            transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.3s ease;
        }

        @keyframes pulse-danger {
            0%, 100% {
                border-color: rgba(239, 68, 68, 0.6);
                box-shadow: 0 0 15px rgba(239, 68, 68, 0.15);
            }
            50% {
                border-color: rgba(239, 68, 68, 1);
                box-shadow: 0 0 30px rgba(239, 68, 68, 0.35);
            }
        }

        .alert-active {
            animation: pulse-danger 1s ease-in-out infinite;
            border-color: #ef4444 !important;
        }

        .stat-value {
            background: linear-gradient(180deg, rgba(255,255,255,0.95) 0%, rgba(148,163,184,0.9) 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .gpm-value {
            background: linear-gradient(180deg, #fbbf24 0%, #f59e0b 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .xpm-value {
            background: linear-gradient(180deg, #38bdf8 0%, #0ea5e9 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .gold-value {
            background: linear-gradient(180deg, #fde68a 0%, #f59e0b 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .connection-dot {
            width: 8px; height: 8px; border-radius: 50%;
            display: inline-block; margin-right: 6px;
        }

        .connected { background: #10b981; box-shadow: 0 0 6px #10b981; }
        .disconnected { background: #ef4444; box-shadow: 0 0 6px #ef4444; }

        .fade-in {
            animation: fadeIn 0.3s ease-out;
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(-4px); }
            to { opacity: 1; transform: translateY(0); }
        }

        /* Draft Panel Styles */
        .role-btn {
            padding: 6px 14px;
            border-radius: 8px;
            background: rgba(30, 41, 59, 0.8);
            color: #94a3b8;
            font-size: 12px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.15s;
            border: 1px solid transparent;
        }
        .role-btn:hover { background: rgba(51, 65, 85, 0.8); color: #e2e8f0; }
        .role-btn.active {
            background: rgba(245, 158, 11, 0.15);
            color: #fbbf24;
            border-color: rgba(245, 158, 11, 0.4);
        }

        .hero-chip {
            display: inline-flex; align-items: center; gap: 4px;
            padding: 4px 10px; border-radius: 6px;
            font-size: 11px; font-weight: 600;
        }
        .hero-chip.ally { background: rgba(16, 185, 129, 0.15); color: #6ee7b7; }
        .hero-chip.enemy { background: rgba(239, 68, 68, 0.15); color: #fca5a5; }
        .hero-chip .remove {
            cursor: pointer; opacity: 0.4; font-size: 14px; line-height: 1;
        }
        .hero-chip .remove:hover { opacity: 1; }

        .suggestion-card {
            transition: all 0.2s;
            cursor: default;
        }
        .suggestion-card:hover {
            border-color: rgba(245, 158, 11, 0.4);
            background: rgba(30, 41, 59, 0.9);
        }

        .hero-search-grid {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 4px;
            max-height: 280px;
            overflow-y: auto;
        }
        .hero-search-btn {
            padding: 6px 4px;
            border-radius: 6px;
            background: rgba(30, 41, 59, 0.8);
            color: #cbd5e1;
            font-size: 10px;
            font-weight: 600;
            cursor: pointer;
            border: 1px solid transparent;
            transition: all 0.1s;
            text-align: center;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .hero-search-btn:hover {
            background: rgba(51, 65, 85, 0.9);
            color: white;
            border-color: rgba(100, 116, 139, 0.5);
        }

        .add-pick-btn {
            padding: 4px 10px;
            border-radius: 6px;
            background: rgba(30, 41, 59, 0.5);
            color: #64748b;
            font-size: 11px;
            cursor: pointer;
            border: 1px dashed rgba(71, 85, 105, 0.5);
            transition: all 0.15s;
        }
        .add-pick-btn:hover {
            background: rgba(51, 65, 85, 0.5);
            color: #94a3b8;
            border-color: rgba(100, 116, 139, 0.5);
        }

        .modal-overlay {
            position: fixed; inset: 0;
            background: rgba(0, 0, 0, 0.65);
            display: flex; align-items: flex-start; justify-content: center;
            padding-top: 80px;
            z-index: 50;
            backdrop-filter: blur(4px);
        }
    </style>
</head>
<body class="min-h-screen p-3 md:p-5">
    <div class="max-w-2xl mx-auto space-y-3">

        <!-- Top Bar -->
        <div class="glass-card rounded-xl px-5 py-3 flex items-center justify-between">
            <div class="flex items-center gap-3">
                <div>
                    <div class="text-xs text-slate-500 uppercase tracking-widest">Game Clock</div>
                    <div id="clock" class="font-mono-bold text-2xl text-white tracking-wider">00:00</div>
                </div>
            </div>
            <div class="flex items-center gap-4">
                <button onclick="toggleDraftPanel()" class="text-[10px] text-slate-500 hover:text-amber-400 uppercase tracking-widest font-semibold transition-colors border border-slate-700 hover:border-amber-500/40 rounded-lg px-3 py-1.5">Draft</button>
                <div class="text-right">
                    <div class="text-xs text-slate-500 uppercase tracking-widest">State</div>
                    <div id="game-state" class="text-sm font-semibold text-slate-400">Waiting for data...</div>
                </div>
            </div>
        </div>

        <!-- Draft Panel (shown during hero selection) -->
        <div id="draft-panel" style="display:none;" class="space-y-3 fade-in">

            <!-- Role Selector -->
            <div class="glass-card rounded-xl px-5 py-3">
                <div class="flex items-center justify-between mb-2">
                    <span class="text-xs text-slate-500 uppercase tracking-widest">Your Role</span>
                    <button onclick="resetDraft()" class="text-[10px] text-slate-600 hover:text-slate-400 transition-colors">Reset Draft</button>
                </div>
                <div class="flex gap-2 flex-wrap" id="role-buttons">
                    <button class="role-btn" data-role="hard_support" onclick="selectRole(this)">Pos 5</button>
                    <button class="role-btn" data-role="soft_support" onclick="selectRole(this)">Pos 4</button>
                    <button class="role-btn" data-role="offlane" onclick="selectRole(this)">Pos 3</button>
                    <button class="role-btn" data-role="mid" onclick="selectRole(this)">Pos 2</button>
                    <button class="role-btn" data-role="carry" onclick="selectRole(this)">Pos 1</button>
                </div>
            </div>

            <!-- Team Picks -->
            <div class="grid grid-cols-2 gap-3">
                <div class="glass-card rounded-xl p-4">
                    <div class="text-xs text-emerald-400 uppercase tracking-widest mb-2 font-semibold">Allies</div>
                    <div id="ally-picks" class="flex flex-wrap gap-1 mb-2"></div>
                    <button class="add-pick-btn" onclick="openHeroSearch('ally')">+ Add Ally</button>
                </div>
                <div class="glass-card rounded-xl p-4">
                    <div class="text-xs text-red-400 uppercase tracking-widest mb-2 font-semibold">Enemies</div>
                    <div id="enemy-picks" class="flex flex-wrap gap-1 mb-2"></div>
                    <button class="add-pick-btn" onclick="openHeroSearch('enemy')">+ Add Enemy</button>
                </div>
            </div>

            <!-- Suggestions -->
            <div class="glass-card rounded-xl p-4">
                <div class="flex items-center justify-between mb-3">
                    <span class="text-xs text-amber-400 uppercase tracking-widest font-semibold">Suggested Heroes</span>
                    <span id="suggestion-note" class="text-[10px] text-slate-600">Pick a role for better results</span>
                </div>
                <div id="suggestions-grid" class="grid grid-cols-2 gap-2">
                    <div class="col-span-2 text-slate-600 text-sm text-center py-4">Select a role to see suggestions</div>
                </div>
            </div>
        </div>

        <!-- Hero Search Modal -->
        <div id="hero-search-modal" style="display:none;" class="modal-overlay" onclick="closeHeroSearchOuter(event)">
            <div class="glass-card rounded-xl p-4 w-full max-w-md" onclick="event.stopPropagation()">
                <div class="flex items-center justify-between mb-3">
                    <span id="search-title" class="text-xs text-slate-400 uppercase tracking-widest font-semibold">Add Hero</span>
                    <button onclick="closeHeroSearch()" class="text-slate-500 hover:text-white text-lg">&times;</button>
                </div>
                <input id="hero-search-input" type="text" placeholder="Search hero..."
                       class="w-full bg-slate-800/80 text-white rounded-lg px-3 py-2 text-sm mb-3 outline-none border border-slate-700 focus:border-slate-500"
                       oninput="filterHeroSearch()" />
                <div id="hero-search-results" class="hero-search-grid"></div>
            </div>
        </div>

        <!-- Hero Info Bar -->
        <div id="hero-bar" class="glass-card rounded-xl px-5 py-3 flex items-center justify-between" style="display:none;">
            <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-lg bg-slate-800 flex items-center justify-center text-lg font-bold text-emerald-400" id="hero-level">0</div>
                <div>
                    <div id="hero-name" class="text-sm font-bold text-white">Unknown Hero</div>
                    <div id="player-name" class="text-xs text-slate-500">—</div>
                </div>
            </div>
            <div class="text-right">
                <div class="text-xs text-slate-500 uppercase tracking-widest">Gold</div>
                <div id="gold" class="font-mono-bold text-xl gold-value">0</div>
            </div>
        </div>

        <!-- Stats Grid -->
        <div class="grid grid-cols-3 gap-3">
            <!-- KDA -->
            <div class="glass-card rounded-xl p-4 text-center">
                <div class="text-[10px] text-slate-500 uppercase tracking-widest mb-1">K / D / A</div>
                <div id="kda" class="font-mono-bold text-2xl stat-value">0 / 0 / 0</div>
            </div>
            <!-- GPM -->
            <div class="glass-card rounded-xl p-4 text-center">
                <div class="text-[10px] text-slate-500 uppercase tracking-widest mb-1">GPM</div>
                <div id="gpm" class="font-mono-bold text-2xl gpm-value">0</div>
            </div>
            <!-- XPM -->
            <div class="glass-card rounded-xl p-4 text-center">
                <div class="text-[10px] text-slate-500 uppercase tracking-widest mb-1">XPM</div>
                <div id="xpm" class="font-mono-bold text-2xl xpm-value">0</div>
            </div>
        </div>

        <!-- Health Bar -->
        <div class="glass-card rounded-xl p-4">
            <div class="flex justify-between items-center mb-2">
                <span class="text-xs text-emerald-400 uppercase tracking-widest font-semibold">Health</span>
                <span id="health-text" class="text-xs text-slate-400 font-mono-bold">0 / 0</span>
            </div>
            <div class="bar-track rounded-full h-5">
                <div id="health-bar" class="bar-fill h-full rounded-full bg-emerald-500" style="width:0%"></div>
            </div>
        </div>

        <!-- Mana Bar -->
        <div class="glass-card rounded-xl p-4">
            <div class="flex justify-between items-center mb-2">
                <span class="text-xs text-sky-400 uppercase tracking-widest font-semibold">Mana</span>
                <span id="mana-text" class="text-xs text-slate-400 font-mono-bold">0 / 0</span>
            </div>
            <div class="bar-track rounded-full h-5">
                <div id="mana-bar" class="bar-fill h-full rounded-full bg-sky-500" style="width:0%"></div>
            </div>
        </div>

        <!-- Coach Alerts -->
        <div id="alerts" class="glass-card rounded-xl p-4 min-h-[80px] transition-all duration-300">
            <div class="flex items-center justify-between mb-2">
                <span class="text-xs text-slate-500 uppercase tracking-widest font-semibold">Coach Alerts</span>
                <span id="connection-status" class="text-xs text-slate-500 flex items-center">
                    <span class="connection-dot disconnected" id="conn-dot"></span>
                    <span id="conn-text">Connecting...</span>
                </span>
            </div>
            <div id="alert-messages">
                <div class="text-slate-600 text-sm">Waiting for game data...</div>
            </div>
        </div>

        <!-- AI Coach Tips -->
        <div id="coach-tips" class="glass-card rounded-xl p-4 min-h-[60px] transition-all duration-300" style="display:none; border-left: 3px solid rgba(245, 158, 11, 0.5);">
            <div class="flex items-center justify-between mb-2">
                <span class="text-xs text-amber-400 uppercase tracking-widest font-semibold flex items-center gap-1.5">
                    &#9733; AI Coach
                </span>
                <span id="coach-source" class="text-[10px] text-slate-600"></span>
            </div>
            <div id="coach-tip-messages"></div>
        </div>

    </div>

    <script>
    // -------------------------------------------------------------------
    // Hero Database (loaded from /api/heroes)
    // -------------------------------------------------------------------
    var heroList = [];
    var heroMap = {};
    var draftLocal = { allyPicks: [], enemyPicks: [], role: "", phase: "none" };
    var addingTo = "";
    var draftPanelManuallyOpened = false;

    fetch("/api/heroes")
        .then(function(r) { return r.json(); })
        .then(function(heroes) {
            heroList = heroes;
            heroes.forEach(function(h) { heroMap[h.id] = h; });
            console.log("[Draft] Loaded " + heroes.length + " heroes");
        })
        .catch(function(e) { console.error("[Draft] Failed to load heroes:", e); });

    // -------------------------------------------------------------------
    // SSE Connection
    // -------------------------------------------------------------------
    var evtSource = new EventSource("/events");

    evtSource.onopen = function() {
        var dot = document.getElementById("conn-dot");
        var txt = document.getElementById("conn-text");
        dot.className = "connection-dot connected";
        txt.textContent = "Live";
    };

    evtSource.onerror = function() {
        var dot = document.getElementById("conn-dot");
        var txt = document.getElementById("conn-text");
        dot.className = "connection-dot disconnected";
        txt.textContent = "Reconnecting...";
    };

    evtSource.onmessage = function(event) {
        try {
            var data = JSON.parse(event.data);
            updateUI(data);
        } catch(e) {
            console.error("Parse error:", e);
        }
    };

    // Draft suggestions from server
    evtSource.addEventListener("draft", function(event) {
        try {
            var suggestions = JSON.parse(event.data);
            renderSuggestions(suggestions);
        } catch(e) {
            console.error("[Draft] Parse error:", e);
        }
    });

    // Draft state sync from server
    evtSource.addEventListener("draftstate", function(event) {
        try {
            var state = JSON.parse(event.data);
            draftLocal.allyPicks = state.ally_picks || [];
            draftLocal.enemyPicks = state.enemy_picks || [];
            draftLocal.role = state.role_preference || "";
            draftLocal.phase = state.phase || "none";
            renderPickChips();
            syncRoleButtons();
        } catch(e) {
            console.error("[Draft] State parse error:", e);
        }
    });

    // AI Coach tips from server
    evtSource.addEventListener("coach", function(event) {
        try {
            var tips = JSON.parse(event.data);
            renderCoachTips(tips);
        } catch(e) {
            console.error("[Coach] Parse error:", e);
        }
    });

    function renderCoachTips(tips) {
        var container = document.getElementById("coach-tips");
        var messages = document.getElementById("coach-tip-messages");
        var sourceEl = document.getElementById("coach-source");

        if (!tips || tips.length === 0) {
            container.style.display = "none";
            return;
        }

        container.style.display = "block";

        var hasAI = false;
        var html = "";
        tips.forEach(function(tip) {
            var color = "text-slate-300";
            var icon = "&#10148; ";
            if (tip.priority === 1) {
                color = "text-amber-300";
                icon = "&#9888; ";
            } else if (tip.priority === 2) {
                color = "text-sky-300";
                icon = "&#9733; ";
            }

            var sourceTag = "";
            if (tip.source === "ai") {
                hasAI = true;
                sourceTag = ' <span class="text-[9px] text-purple-400/60 ml-1 border border-purple-400/30 rounded px-1">AI</span>';
            }

            html += '<div class="' + color + ' text-sm font-medium fade-in mb-1">'
                 + icon + tip.text + sourceTag + '</div>';
        });
        messages.innerHTML = html;
        sourceEl.textContent = hasAI ? "GPT-4o-mini" : "rules";
    }

    // -------------------------------------------------------------------
    // Formatting Helpers
    // -------------------------------------------------------------------
    function formatTime(seconds) {
        if (seconds === undefined || seconds === null) return "00:00";
        var neg = seconds < 0;
        var abs = Math.abs(seconds);
        var m = Math.floor(abs / 60);
        var s = abs % 60;
        var pad = function(n) { return n < 10 ? "0" + n : "" + n; };
        return (neg ? "-" : "") + pad(m) + ":" + pad(s);
    }

    function formatGameState(state) {
        if (!state) return "Unknown";
        var cleaned = state.replace("DOTA_GAMERULES_STATE_", "");
        var words = cleaned.split("_");
        var result = [];
        for (var i = 0; i < words.length; i++) {
            var w = words[i];
            result.push(w.charAt(0).toUpperCase() + w.slice(1).toLowerCase());
        }
        return result.join(" ");
    }

    function formatHeroName(name) {
        if (!name) return "Unknown Hero";
        var cleaned = name.replace("npc_dota_hero_", "");
        var words = cleaned.split("_");
        var result = [];
        for (var i = 0; i < words.length; i++) {
            var w = words[i];
            result.push(w.charAt(0).toUpperCase() + w.slice(1));
        }
        return result.join(" ");
    }

    // -------------------------------------------------------------------
    // UI Update
    // -------------------------------------------------------------------
    function updateUI(data) {
        // Map / Clock
        if (data.map) {
            document.getElementById("clock").textContent = formatTime(data.map.clock_time);
            document.getElementById("game-state").textContent = formatGameState(data.map.game_state);

            var stateEl = document.getElementById("game-state");
            var gs = data.map.game_state || "";
            if (gs.indexOf("IN_PROGRESS") !== -1) {
                stateEl.className = "text-sm font-semibold text-emerald-400";
            } else if (gs.indexOf("PRE_GAME") !== -1) {
                stateEl.className = "text-sm font-semibold text-amber-400";
            } else if (gs.indexOf("HERO_SELECTION") !== -1 || gs.indexOf("STRATEGY_TIME") !== -1) {
                stateEl.className = "text-sm font-semibold text-purple-400";
            } else {
                stateEl.className = "text-sm font-semibold text-slate-400";
            }

            // Toggle draft panel visibility
            var draftPanel = document.getElementById("draft-panel");
            if (gs.indexOf("HERO_SELECTION") !== -1 || gs.indexOf("STRATEGY_TIME") !== -1) {
                draftPanel.style.display = "block";
                draftLocal.phase = "hero_selection";
            } else if (draftPanelManuallyOpened) {
                draftPanel.style.display = "block";
            } else if (gs.indexOf("IN_PROGRESS") !== -1 || gs.indexOf("PRE_GAME") !== -1) {
                draftPanel.style.display = "none";
                draftLocal.phase = "in_progress";
            }
        }

        // Player stats
        if (data.player) {
            var p = data.player;
            document.getElementById("kda").textContent = p.kills + " / " + p.deaths + " / " + p.assists;
            document.getElementById("gpm").textContent = p.gpm;
            document.getElementById("xpm").textContent = p.xpm;

            if (p.name) {
                document.getElementById("player-name").textContent = p.name;
            }
            if (p.gold !== undefined) {
                document.getElementById("gold").textContent = p.gold.toLocaleString();
            }
        }

        // Hero vitals
        if (data.hero) {
            var h = data.hero;

            // Show hero bar
            document.getElementById("hero-bar").style.display = "flex";

            // Hero info
            document.getElementById("hero-name").textContent = formatHeroName(h.name);
            document.getElementById("hero-level").textContent = h.level || 0;

            // Health bar
            var hp = h.health_percent || 0;
            var healthBar = document.getElementById("health-bar");
            healthBar.style.width = hp + "%";
            document.getElementById("health-text").textContent = (h.health || 0) + " / " + (h.max_health || 0);

            if (hp <= 20) {
                healthBar.className = "bar-fill h-full rounded-full bg-red-500";
            } else if (hp <= 50) {
                healthBar.className = "bar-fill h-full rounded-full bg-amber-500";
            } else {
                healthBar.className = "bar-fill h-full rounded-full bg-emerald-500";
            }

            // Mana bar
            var mp = h.mana_percent || 0;
            document.getElementById("mana-bar").style.width = mp + "%";
            document.getElementById("mana-text").textContent = (h.mana || 0) + " / " + (h.max_mana || 0);

            // Coach Alerts
            updateAlerts(h);
        }
    }

    function updateAlerts(hero) {
        var alertBox = document.getElementById("alerts");
        var alertMessages = document.getElementById("alert-messages");
        var alerts = [];

        if (hero.health_percent <= 20 && hero.alive) {
            alerts.push({ text: "LOW HEALTH (" + hero.health_percent + "%) — Fall back!", type: "danger" });
        }
        if (hero.mana_percent <= 10 && hero.alive) {
            alerts.push({ text: "LOW MANA (" + hero.mana_percent + "%) — Conserve spells!", type: "danger" });
        }
        if (!hero.alive) {
            alerts.push({ text: "DEAD — Respawning in " + (hero.respawn_seconds || 0) + "s", type: "warning" });
        }
        if (hero.stunned) {
            alerts.push({ text: "STUNNED!", type: "warning" });
        }
        if (hero.silenced) {
            alerts.push({ text: "SILENCED!", type: "warning" });
        }
        if (hero.hexed) {
            alerts.push({ text: "HEXED!", type: "warning" });
        }
        if (hero.smoked) {
            alerts.push({ text: "SMOKED — Move carefully", type: "info" });
        }
        if (hero.break) {
            alerts.push({ text: "BREAK — Passives disabled!", type: "warning" });
        }

        // Danger flash
        if (alerts.some(function(a) { return a.type === "danger"; })) {
            alertBox.classList.add("alert-active");
        } else {
            alertBox.classList.remove("alert-active");
        }

        // Render
        if (alerts.length === 0) {
            alertMessages.innerHTML = '<div class="text-emerald-700 text-sm font-semibold">All clear</div>';
        } else {
            var html = "";
            for (var i = 0; i < alerts.length; i++) {
                var a = alerts[i];
                var color = "text-slate-400";
                var icon = "";
                if (a.type === "danger") {
                    color = "text-red-400";
                    icon = "&#9888; ";
                } else if (a.type === "warning") {
                    color = "text-amber-400";
                    icon = "&#9888; ";
                } else {
                    color = "text-sky-400";
                    icon = "&#8505; ";
                }
                html += '<div class="' + color + ' text-sm font-semibold fade-in">' + icon + a.text + '</div>';
            }
            alertMessages.innerHTML = html;
        }
    }

    // -------------------------------------------------------------------
    // Draft Panel Functions
    // -------------------------------------------------------------------
    function toggleDraftPanel() {
        var panel = document.getElementById("draft-panel");
        if (panel.style.display === "none") {
            panel.style.display = "block";
            draftPanelManuallyOpened = true;
        } else {
            panel.style.display = "none";
            draftPanelManuallyOpened = false;
        }
    }

    function selectRole(btn) {
        var role = btn.getAttribute("data-role");
        draftLocal.role = role;

        // Update button styles
        document.querySelectorAll(".role-btn").forEach(function(b) {
            b.classList.remove("active");
        });
        btn.classList.add("active");

        // Send to server
        fetch("/api/draft/role", {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify({role: role})
        }).then(function() { fetchSuggestions(); });
    }

    function syncRoleButtons() {
        document.querySelectorAll(".role-btn").forEach(function(b) {
            if (b.getAttribute("data-role") === draftLocal.role) {
                b.classList.add("active");
            } else {
                b.classList.remove("active");
            }
        });
    }

    function openHeroSearch(target) {
        addingTo = target;
        var modal = document.getElementById("hero-search-modal");
        var title = document.getElementById("search-title");
        title.textContent = target === "ally" ? "Add Ally Hero" : "Add Enemy Hero";
        modal.style.display = "flex";
        var input = document.getElementById("hero-search-input");
        input.value = "";
        input.focus();
        filterHeroSearch();
    }

    function closeHeroSearch() {
        document.getElementById("hero-search-modal").style.display = "none";
    }

    function closeHeroSearchOuter(event) {
        if (event.target === document.getElementById("hero-search-modal")) {
            closeHeroSearch();
        }
    }

    function filterHeroSearch() {
        var query = document.getElementById("hero-search-input").value.toLowerCase();
        var container = document.getElementById("hero-search-results");

        // Exclude already picked heroes
        var excluded = {};
        draftLocal.allyPicks.forEach(function(id) { excluded[id] = true; });
        draftLocal.enemyPicks.forEach(function(id) { excluded[id] = true; });

        var filtered = heroList.filter(function(h) {
            if (excluded[h.id]) return false;
            if (!query) return true;
            return h.localized_name.toLowerCase().indexOf(query) !== -1 ||
                   h.name.toLowerCase().indexOf(query) !== -1;
        });

        var html = "";
        filtered.forEach(function(h) {
            html += '<button class="hero-search-btn" onclick="pickHero(' + h.id + ')" title="' + h.localized_name + '">'
                 + h.localized_name + '</button>';
        });
        if (filtered.length === 0) {
            html = '<div class="col-span-4 text-slate-600 text-sm text-center py-4">No heroes found</div>';
        }
        container.innerHTML = html;
    }

    function pickHero(heroId) {
        var endpoint = addingTo === "ally" ? "/api/draft/ally" : "/api/draft/enemy";
        fetch(endpoint, {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify({hero_id: heroId})
        }).then(function() {
            if (addingTo === "ally") {
                if (draftLocal.allyPicks.indexOf(heroId) === -1) draftLocal.allyPicks.push(heroId);
            } else {
                if (draftLocal.enemyPicks.indexOf(heroId) === -1) draftLocal.enemyPicks.push(heroId);
            }
            renderPickChips();
            closeHeroSearch();
            fetchSuggestions();
        });
    }

    function removeHeroPick(heroId, isAlly) {
        var endpoint = isAlly ? "/api/draft/ally" : "/api/draft/enemy";
        fetch(endpoint, {
            method: "DELETE",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify({hero_id: heroId})
        }).then(function() {
            if (isAlly) {
                draftLocal.allyPicks = draftLocal.allyPicks.filter(function(id) { return id !== heroId; });
            } else {
                draftLocal.enemyPicks = draftLocal.enemyPicks.filter(function(id) { return id !== heroId; });
            }
            renderPickChips();
            fetchSuggestions();
        });
    }

    function renderPickChips() {
        var allyContainer = document.getElementById("ally-picks");
        var enemyContainer = document.getElementById("enemy-picks");

        var allyHTML = "";
        draftLocal.allyPicks.forEach(function(id) {
            var name = heroMap[id] ? heroMap[id].localized_name : "Hero #" + id;
            allyHTML += '<span class="hero-chip ally">' + name
                     + ' <span class="remove" onclick="removeHeroPick(' + id + ', true)">&times;</span></span>';
        });
        allyContainer.innerHTML = allyHTML;

        var enemyHTML = "";
        draftLocal.enemyPicks.forEach(function(id) {
            var name = heroMap[id] ? heroMap[id].localized_name : "Hero #" + id;
            enemyHTML += '<span class="hero-chip enemy">' + name
                      + ' <span class="remove" onclick="removeHeroPick(' + id + ', false)">&times;</span></span>';
        });
        enemyContainer.innerHTML = enemyHTML;
    }

    function fetchSuggestions() {
        fetch("/api/suggestions")
            .then(function(r) { return r.json(); })
            .then(renderSuggestions)
            .catch(function(e) { console.error("[Draft] Fetch suggestions error:", e); });
    }

    function renderSuggestions(suggestions) {
        var container = document.getElementById("suggestions-grid");
        var note = document.getElementById("suggestion-note");

        if (!suggestions || suggestions.length === 0) {
            container.innerHTML = '<div class="col-span-2 text-slate-600 text-sm text-center py-4">No suggestions available</div>';
            return;
        }

        var hasContext = draftLocal.allyPicks.length > 0 || draftLocal.enemyPicks.length > 0 || draftLocal.role;
        if (hasContext) {
            note.textContent = suggestions.length + " heroes ranked";
        } else {
            note.textContent = "Add picks/role for better results";
        }

        var html = "";
        suggestions.forEach(function(s, i) {
            var opacity = i < 4 ? "" : "opacity-70";
            var border = i === 0 ? "border-amber-500/40" : "border-slate-700/50";
            var crown = i === 0 ? '<span class="text-amber-400 text-[10px] font-bold uppercase tracking-wider">Best Pick</span>' : '';
            var scoreColor = s.score >= 0.7 ? "text-emerald-400" : s.score >= 0.5 ? "text-amber-400" : "text-slate-400";

            var attrIcon = "";
            if (s.primary_attr === "str") attrIcon = '<span class="text-red-400 text-[10px]">STR</span>';
            else if (s.primary_attr === "agi") attrIcon = '<span class="text-emerald-400 text-[10px]">AGI</span>';
            else if (s.primary_attr === "int") attrIcon = '<span class="text-sky-400 text-[10px]">INT</span>';
            else attrIcon = '<span class="text-slate-400 text-[10px]">UNI</span>';

            var reason = s.reasons && s.reasons.length > 0 ? s.reasons[0] : "";

            html += '<div class="glass-card rounded-lg p-3 border suggestion-card ' + border + ' ' + opacity + '">'
                 + '<div class="flex items-center justify-between mb-1">'
                 + '<span class="text-sm font-bold text-white truncate">' + s.localized_name + '</span>'
                 + attrIcon
                 + '</div>'
                 + crown
                 + '<div class="font-mono-bold text-lg ' + scoreColor + '">' + (s.score * 100).toFixed(0) + '</div>'
                 + '<div class="text-[10px] text-slate-500 truncate mt-1" title="' + reason + '">' + reason + '</div>'
                 + '</div>';
        });
        container.innerHTML = html;
    }

    function resetDraft() {
        fetch("/api/draft/reset", { method: "POST" }).then(function() {
            draftLocal.allyPicks = [];
            draftLocal.enemyPicks = [];
            draftLocal.role = "";
            renderPickChips();
            syncRoleButtons();
            document.getElementById("suggestions-grid").innerHTML =
                '<div class="col-span-2 text-slate-600 text-sm text-center py-4">Select a role to see suggestions</div>';
        });
    }
    </script>
</body>
</html>`
