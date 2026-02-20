package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// ---------------------------------------------------------------------------
// GSI Payload Structs
// ---------------------------------------------------------------------------

type GSIPayload struct {
	Provider *Provider `json:"provider,omitempty"`
	Map      *MapState `json:"map,omitempty"`
	Player   *Player   `json:"player,omitempty"`
	Hero     *Hero     `json:"hero,omitempty"`
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
}

type Player struct {
	SteamID            string `json:"steamid"`
	Name               string `json:"name"`
	Activity           string `json:"activity"`
	Kills              int    `json:"kills"`
	Deaths             int    `json:"deaths"`
	Assists            int    `json:"assists"`
	LastHits           int    `json:"last_hits"`
	Denies             int    `json:"denies"`
	KillStreak         int    `json:"kill_streak"`
	CommandsIssued     int    `json:"commands_issued"`
	TeamName           string `json:"team_name"`
	Gold               int    `json:"gold"`
	GoldReliable       int    `json:"gold_reliable"`
	GoldUnreliable     int    `json:"gold_unreliable"`
	GoldFromHeroKills  int    `json:"gold_from_hero_kills"`
	GoldFromCreepKills int    `json:"gold_from_creep_kills"`
	GoldFromIncome     int    `json:"gold_from_income"`
	GoldFromShared     int    `json:"gold_from_shared"`
	GPM                int    `json:"gpm"`
	XPM                int    `json:"xpm"`
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

type SSEBroker struct {
	clients    map[chan string]bool
	register   chan chan string
	unregister chan chan string
	broadcast  chan string
}

func NewSSEBroker() *SSEBroker {
	b := &SSEBroker{
		clients:    make(map[chan string]bool),
		register:   make(chan chan string),
		unregister: make(chan chan string),
		broadcast:  make(chan string, 8),
	}
	go b.run()
	return b
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
// HTTP Handlers
// ---------------------------------------------------------------------------

func handleGSIPost(store *GameStateStore, broker *SSEBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload GSIPayload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		store.Update(&payload)

		data, err := json.Marshal(payload)
		if err == nil {
			broker.broadcast <- string(data)
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

		clientChan := make(chan string, 2)
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
				fmt.Fprintf(w, "data: %s\n\n", msg)
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

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleDashboard(w, r)
		case http.MethodPost:
			handleGSIPost(store, broker)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/events", handleSSE(broker))

	log.Println("==============================================")
	log.Println("  Dota 2 Coach Overlay — Running on :3000")
	log.Println("==============================================")
	log.Println("Dashboard : http://localhost:3000")
	log.Println("SSE Stream: http://localhost:3000/events")
	log.Println("GSI POST  : http://localhost:3000/")
	log.Println("Waiting for game state data...")

	if err := http.ListenAndServe(":3000", mux); err != nil {
		log.Fatal(err)
	}
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
            <div class="text-right">
                <div class="text-xs text-slate-500 uppercase tracking-widest">State</div>
                <div id="game-state" class="text-sm font-semibold text-slate-400">Waiting for data...</div>
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

    </div>

    <script>
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
            } else {
                stateEl.className = "text-sm font-semibold text-slate-400";
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
    </script>
</body>
</html>`
