# Dota 2 Coach Overlay — Setup & Testing Guide

## Prerequisites

Install Go 1.21 or later:

```bash
# macOS (Homebrew)
brew install go

# Or download from https://go.dev/dl/
```

Verify installation:

```bash
go version
```

## Quick Start

```bash
cd /Users/amirhossein/Desktop/projects/Dota2-GSI
go run main.go
```

You should see:

```
==============================================
  Dota 2 Coach Overlay — Running on :3000
==============================================
Dashboard : http://localhost:3000
SSE Stream: http://localhost:3000/events
GSI POST  : http://localhost:3000/
Waiting for game state data...
```

Open **http://localhost:3000** in your browser.

## Testing Without Dota 2

Use `curl` to simulate GSI payloads. Open a second terminal and run these commands while the dashboard is open in your browser.

### Test 1: Normal Gameplay

```bash
curl -X POST http://localhost:3000/ \
  -H "Content-Type: application/json" \
  -d '{
    "provider": {"name": "Dota 2", "appid": 570, "version": 47, "timestamp": 1234567890},
    "map": {"name": "start", "matchid": "123", "game_time": 754, "clock_time": 754, "daytime": true, "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS"},
    "player": {"steamid": "123", "name": "TestPlayer", "activity": "playing", "kills": 5, "deaths": 2, "assists": 10, "gold": 2350, "gpm": 450, "xpm": 500},
    "hero": {"id": 1, "name": "npc_dota_hero_antimage", "level": 15, "alive": true, "health": 1200, "max_health": 1500, "health_percent": 80, "mana": 300, "max_mana": 500, "mana_percent": 60}
  }'
```

**Expected:** Dashboard shows clock at `12:34`, game state "Game In Progress" (green), K/D/A `5 / 2 / 10`, healthy green health bar at 80%, blue mana bar at 60%, hero "Antimage" at level 15, gold 2,350.

### Test 2: Low Health + Low Mana (Danger Alerts)

```bash
curl -X POST http://localhost:3000/ \
  -H "Content-Type: application/json" \
  -d '{
    "provider": {"name": "Dota 2", "appid": 570, "version": 47, "timestamp": 1234567890},
    "map": {"name": "start", "matchid": "123", "game_time": 1200, "clock_time": 1200, "daytime": false, "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS"},
    "player": {"steamid": "123", "name": "TestPlayer", "activity": "playing", "kills": 7, "deaths": 4, "assists": 12, "gold": 890, "gpm": 520, "xpm": 580},
    "hero": {"id": 1, "name": "npc_dota_hero_antimage", "level": 18, "alive": true, "health": 200, "max_health": 1800, "health_percent": 11, "mana": 30, "max_mana": 600, "mana_percent": 5, "silenced": true}
  }'
```

**Expected:** Health bar turns RED at 11%, mana bar nearly empty at 5%. Coach Alerts box pulses red with warnings: "LOW HEALTH", "LOW MANA", and "SILENCED". Clock shows `20:00`.

### Test 3: Dead Hero

```bash
curl -X POST http://localhost:3000/ \
  -H "Content-Type: application/json" \
  -d '{
    "provider": {"name": "Dota 2", "appid": 570, "version": 47, "timestamp": 1234567890},
    "map": {"name": "start", "matchid": "123", "game_time": 1800, "clock_time": 1800, "daytime": true, "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS"},
    "player": {"steamid": "123", "name": "TestPlayer", "activity": "playing", "kills": 8, "deaths": 5, "assists": 14, "gold": 450, "gpm": 490, "xpm": 540},
    "hero": {"id": 1, "name": "npc_dota_hero_phantom_assassin", "level": 22, "alive": false, "respawn_seconds": 42, "health": 0, "max_health": 2200, "health_percent": 0, "mana": 0, "max_mana": 700, "mana_percent": 0}
  }'
```

**Expected:** Both bars at 0%. Coach Alerts shows "DEAD — Respawning in 42s". Hero name changes to "Phantom Assassin", level 22. Clock at `30:00`.

### Test 4: Continuous Simulation (30-second game loop)

This script sends an update every second with gradually declining health and mana, simulating a fight sequence:

```bash
for i in $(seq 1 30); do
  HP=$((100 - i * 3))
  MP=$((100 - i * 2))
  if [ $HP -lt 0 ]; then HP=0; fi
  if [ $MP -lt 0 ]; then MP=0; fi
  curl -s -X POST http://localhost:3000/ \
    -H "Content-Type: application/json" \
    -d "{
      \"provider\":{\"name\":\"Dota 2\",\"appid\":570,\"version\":47,\"timestamp\":$(date +%s)},
      \"map\":{\"name\":\"start\",\"matchid\":\"123\",\"game_time\":$((300+i)),\"clock_time\":$((300+i)),\"daytime\":true,\"game_state\":\"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS\"},
      \"player\":{\"steamid\":\"123\",\"name\":\"TestPlayer\",\"activity\":\"playing\",\"kills\":$((i/5)),\"deaths\":$((i/10)),\"assists\":$((i/3)),\"gold\":$((1500+i*30)),\"gpm\":$((400+i*5)),\"xpm\":$((450+i*4))},
      \"hero\":{\"id\":1,\"name\":\"npc_dota_hero_invoker\",\"level\":$((10+i/5)),\"alive\":true,\"health\":$((HP*18)),\"max_health\":1800,\"health_percent\":$HP,\"mana\":$((MP*6)),\"max_mana\":600,\"mana_percent\":$MP}
    }"
  sleep 1
done
```

**Expected:** Watch the health bar animate from green to yellow to red, mana bar shrink, and coach alerts trigger as health drops below 20% and mana below 10%.

## Dota 2 Integration (When You Have Dota Installed)

Copy `gamestate_integration_coach.cfg` to the Dota 2 GSI config folder:

| Platform | Path |
|----------|------|
| **Windows** | `C:\Program Files (x86)\Steam\steamapps\common\dota 2 beta\game\dota\cfg\gamestate_integration\` |
| **macOS** | `~/Library/Application Support/Steam/steamapps/common/dota 2 beta/game/dota/cfg/gamestate_integration/` |
| **Linux** | `~/.steam/steam/steamapps/common/dota 2 beta/game/dota/cfg/gamestate_integration/` |

Create the `gamestate_integration` folder if it does not exist.

Then:

1. Start the overlay server: `go run main.go`
2. Open the dashboard in a browser: http://localhost:3000
3. Launch Dota 2 and start a game (bot match works fine)
4. The dashboard updates in real-time as you play

To use in the Steam Overlay:

1. Add http://localhost:3000 as a non-Steam game URL, or
2. Use the Steam Overlay browser (Shift+Tab) and navigate to http://localhost:3000

## Architecture

```
Dota 2 Client
    |
    | POST / (JSON every ~100ms)
    v
Go HTTP Server (:3000)
    |
    |-- GameStateStore (thread-safe, latest state)
    |-- SSEBroker (broadcasts to all browser clients)
    |
    | GET /events (Server-Sent Events stream)
    v
Browser (EventSource -> DOM updates)
```

## Troubleshooting

- **Port 3000 already in use**: Kill the existing process with `lsof -ti:3000 | xargs kill` then retry
- **No updates in browser**: Check the connection dot in the Coach Alerts section — it should be green ("Live")
- **TailwindCSS not loading**: Requires an internet connection for the CDN script tag
