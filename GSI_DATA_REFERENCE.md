# Dota 2 Game State Integration — Player Data Reference

Complete reference of all data available via Dota 2 GSI when **playing** (not spectating).

> **Key limitation:** As a player, GSI only exposes your own data. Other players' details
> (hero, items, abilities) are NOT available — that is spectator-only. Draft data (team picks/bans)
> is also primarily spectator-only, though partial data may come through.

---

## Configuration

File: `steamapps/common/dota 2 beta/game/dota/cfg/gamestate_integration/gamestate_integration_*.cfg`

Launch option required: `-gamestateintegration`

```
"Dota 2 Coach Overlay"
{
    "uri"               "http://localhost:3000/"
    "timeout"           "5.0"
    "buffer"            "0.1"
    "throttle"          "0.1"
    "heartbeat"         "30.0"
    "data"
    {
        "provider"      "1"     // App info + timestamp
        "map"           "1"     // Match state, clock, scores
        "player"        "1"     // Local player stats
        "hero"          "1"     // Local hero vitals + status
        "abilities"     "1"     // Local hero abilities
        "items"         "1"     // Local hero inventory + stash
        "buildings"     "1"     // All tower/barracks health
        "draft"         "1"     // Pick/ban data (spectator-primary)
        "wearables"     "0"     // Cosmetics (not useful for coaching)
    }
}
```

Set a field to `"1"` to enable, `"0"` to disable.

---

## Game States (Lifecycle)

The `map.game_state` field cycles through these values in order:

| # | State String | Description |
|---|-------------|-------------|
| 0 | `DOTA_GAMERULES_STATE_INIT` | Initial state |
| 1 | `DOTA_GAMERULES_STATE_WAIT_FOR_PLAYERS_TO_LOAD` | Loading screen |
| 2 | `DOTA_GAMERULES_STATE_HERO_SELECTION` | Hero picking phase |
| 3 | `DOTA_GAMERULES_STATE_STRATEGY_TIME` | Pre-game planning (picks confirmed, before horn) |
| 4 | `DOTA_GAMERULES_STATE_PRE_GAME` | Before battle horn, players moving to lanes |
| 5 | `DOTA_GAMERULES_STATE_GAME_IN_PROGRESS` | Active gameplay |
| 6 | `DOTA_GAMERULES_STATE_POST_GAME` | Match ended, score screen |
| 7 | `DOTA_GAMERULES_STATE_DISCONNECT` | Disconnected |
| 8 | `DOTA_GAMERULES_STATE_TEAM_SHOWCASE` | Team showcase phase |
| 9 | `DOTA_GAMERULES_STATE_CUSTOM_GAME_SETUP` | Custom game lobby |
| 10 | `DOTA_GAMERULES_STATE_WAIT_FOR_MAP_TO_LOAD` | Map loading |
| 11 | `DOTA_GAMERULES_STATE_SCENARIO_SETUP` | Scenario setup |
| 12 | `DOTA_GAMERULES_STATE_PLAYER_DRAFT` | Player draft (Captains Mode player assignment) |

### What data is available per phase (Player Mode)

| Phase | provider | map | player | hero | abilities | items | buildings | draft |
|-------|----------|-----|--------|------|-----------|-------|-----------|-------|
| WAIT_FOR_PLAYERS_TO_LOAD | Yes | Partial | Partial | No | No | No | No | No |
| HERO_SELECTION | Yes | Yes | Partial* | No | No | No | No | Partial** |
| STRATEGY_TIME | Yes | Yes | Yes | Yes | Yes | No | No | Partial** |
| PRE_GAME | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No |
| GAME_IN_PROGRESS | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No |
| POST_GAME | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No |

\* During HERO_SELECTION: other players' `steamId` and `name` return empty/zeroed values.
\** Draft data is primarily for spectators. In player mode, it may be empty or partial.

---

## Data Categories

### 1. Provider

App metadata. Sent with every payload.

```json
{
    "provider": {
        "name": "Dota 2",
        "appid": 570,
        "version": 47,
        "timestamp": 1700000000
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Always `"Dota 2"` |
| `appid` | int | Always `570` |
| `version` | int | GSI protocol version |
| `timestamp` | int64 | Unix timestamp of this payload |

---

### 2. Map

Match and game clock information. Available from HERO_SELECTION onward.

```json
{
    "map": {
        "name": "start",
        "matchid": "7654321",
        "game_time": 1234,
        "clock_time": 1234,
        "daytime": true,
        "nightstalker_night": false,
        "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS",
        "paused": false,
        "win_team": "none",
        "customgamename": "",
        "ward_purchase_cooldown": 0,
        "radiant_ward_purchase_cooldown": 0,
        "dire_ward_purchase_cooldown": 0,
        "roshan_state": "alive",
        "roshan_state_end_seconds": 0,
        "radiant_score": 15,
        "dire_score": 12
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Map name (usually `"start"`) |
| `matchid` | string | Unique match identifier |
| `game_time` | int | Total elapsed time in seconds (includes pre-game) |
| `clock_time` | int | In-game clock time (negative during pre-game) |
| `daytime` | bool | Whether it is currently daytime |
| `nightstalker_night` | bool | Whether Nightstalker's darkness is active |
| `game_state` | string | Current game phase (see Game States table) |
| `paused` | bool | Whether the game is paused |
| `win_team` | string | Winning team (`"radiant"`, `"dire"`, or `"none"`) |
| `customgamename` | string | Custom game name (empty for regular matches) |
| `ward_purchase_cooldown` | int | Ward purchase cooldown for local player |
| `radiant_ward_purchase_cooldown` | int | Radiant team ward cooldown |
| `dire_ward_purchase_cooldown` | int | Dire team ward cooldown |
| `roshan_state` | string | Roshan status (`"alive"`, `"respawn_base"`, `"respawn_variable"`) |
| `roshan_state_end_seconds` | int | Seconds until Roshan state changes |
| `radiant_score` | int | Radiant team total kills |
| `dire_score` | int | Dire team total kills |

---

### 3. Player

Local player statistics. Full data available from STRATEGY_TIME onward.

```json
{
    "player": {
        "steamid": "76561198012345678",
        "accountid": "52079950",
        "name": "PlayerName",
        "activity": "playing",
        "kills": 5,
        "deaths": 2,
        "assists": 10,
        "last_hits": 150,
        "denies": 12,
        "kill_streak": 3,
        "commands_issued": 4500,
        "kill_list": {"0": 25, "1": 50},
        "team_name": "radiant",
        "player_slot": 0,
        "player_team_slot": 0,
        "gold": 3500,
        "gold_reliable": 1200,
        "gold_unreliable": 2300,
        "gold_from_hero_kills": 2000,
        "gold_from_creep_kills": 5000,
        "gold_from_income": 3000,
        "gold_from_shared": 500,
        "gpm": 450,
        "xpm": 520,
        "net_worth": 15000,
        "hero_damage": 12000,
        "hero_healing": 0,
        "tower_damage": 3000,
        "support_gold_spent": 0,
        "consumable_gold_spent": 1500,
        "item_gold_spent": 12000,
        "gold_lost_to_death": 400,
        "gold_spent_on_buybacks": 0,
        "wards_purchased": 0,
        "wards_placed": 0,
        "wards_destroyed": 0,
        "runes_activated": 3,
        "camps_stacked": 2
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `steamid` | string | Steam 64-bit ID |
| `accountid` | string | Steam account ID |
| `name` | string | Player display name |
| `activity` | string | `"playing"` or `"menu"` |
| `kills` | int | Total kills |
| `deaths` | int | Total deaths |
| `assists` | int | Total assists |
| `last_hits` | int | Total last hits |
| `denies` | int | Total denies |
| `kill_streak` | int | Current kill streak |
| `commands_issued` | int | Total commands (APM proxy) |
| `kill_list` | map | Map of kill index to victim hero ID |
| `team_name` | string | `"radiant"` or `"dire"` |
| `player_slot` | int | Player slot (0-9) |
| `player_team_slot` | int | Slot within team (0-4) |
| `gold` | int | Current total gold |
| `gold_reliable` | int | Reliable gold |
| `gold_unreliable` | int | Unreliable gold |
| `gold_from_hero_kills` | int | Total gold earned from hero kills |
| `gold_from_creep_kills` | int | Total gold earned from creep kills |
| `gold_from_income` | int | Total passive gold income |
| `gold_from_shared` | int | Total shared gold (assists, etc.) |
| `gpm` | int | Gold per minute |
| `xpm` | int | Experience per minute |
| `net_worth` | int | Total net worth |
| `hero_damage` | int | Total hero damage dealt |
| `hero_healing` | int | Total hero healing done |
| `tower_damage` | int | Total tower damage dealt |
| `support_gold_spent` | int | Gold spent on support items |
| `consumable_gold_spent` | int | Gold spent on consumables |
| `item_gold_spent` | int | Total gold spent on items |
| `gold_lost_to_death` | int | Gold lost from dying |
| `gold_spent_on_buybacks` | int | Gold spent on buybacks |
| `wards_purchased` | int | Wards purchased |
| `wards_placed` | int | Wards placed |
| `wards_destroyed` | int | Enemy wards destroyed |
| `runes_activated` | int | Runes picked up |
| `camps_stacked` | int | Neutral camps stacked |

---

### 4. Hero

Local hero vitals, position, and status effects. Available from STRATEGY_TIME onward.

```json
{
    "hero": {
        "xpos": -6500,
        "ypos": -6300,
        "id": 1,
        "name": "npc_dota_hero_antimage",
        "level": 15,
        "xp": 12000,
        "alive": true,
        "respawn_seconds": 0,
        "buyback_cost": 800,
        "buyback_cooldown": 0,
        "health": 1200,
        "max_health": 1500,
        "health_percent": 80,
        "mana": 400,
        "max_mana": 600,
        "mana_percent": 67,
        "silenced": false,
        "stunned": false,
        "disarmed": false,
        "magicimmune": false,
        "hexed": false,
        "muted": false,
        "break": false,
        "aghanims_scepter": false,
        "aghanims_shard": true,
        "smoked": false,
        "has_debuff": false,
        "selected_unit": false,
        "talent_tree": [false, false, true, false, true, false, false, false],
        "attributes_level": 0
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `xpos` | int | X position on the map |
| `ypos` | int | Y position on the map |
| `id` | int | Hero ID (matches OpenDota hero IDs) |
| `name` | string | Internal hero name (e.g., `"npc_dota_hero_antimage"`) |
| `level` | int | Current hero level (1-30) |
| `xp` | int | Current experience points |
| `alive` | bool | Whether hero is alive |
| `respawn_seconds` | int | Seconds until respawn (0 if alive) |
| `buyback_cost` | int | Gold cost to buyback |
| `buyback_cooldown` | int | Seconds until buyback available |
| `health` | int | Current health points |
| `max_health` | int | Maximum health points |
| `health_percent` | int | Health as percentage (0-100) |
| `mana` | int | Current mana points |
| `max_mana` | int | Maximum mana points |
| `mana_percent` | int | Mana as percentage (0-100) |
| `silenced` | bool | Cannot cast spells |
| `stunned` | bool | Cannot act |
| `disarmed` | bool | Cannot attack |
| `magicimmune` | bool | Immune to magic damage/effects |
| `hexed` | bool | Polymorphed |
| `muted` | bool | Cannot use items |
| `break` | bool | Passive abilities disabled |
| `aghanims_scepter` | bool | Has Aghanim's Scepter upgrade |
| `aghanims_shard` | bool | Has Aghanim's Shard upgrade |
| `smoked` | bool | Under Smoke of Deceit |
| `has_debuff` | bool | Has any debuff active |
| `selected_unit` | bool | Whether a non-hero unit is selected |
| `talent_tree` | bool[] | Array of 8 booleans for talent choices (index 0 = level 10 left, etc.) |
| `attributes_level` | int | Bonus attribute levels |

---

### 5. Abilities

Local hero's abilities. Available from STRATEGY_TIME onward.

```json
{
    "abilities": {
        "ability0": {
            "name": "antimage_mana_break",
            "level": 4,
            "can_cast": true,
            "passive": true,
            "ability_active": false,
            "cooldown": 0,
            "ultimate": false,
            "charges": 0,
            "max_charges": 0,
            "charge_cooldown": 0
        },
        "ability1": { ... },
        "ability2": { ... },
        "ability3": { ... },
        "ability4": { ... },
        "ability5": { ... }
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Internal ability name |
| `level` | int | Ability level (0 = not skilled) |
| `can_cast` | bool | Whether ability can be cast right now |
| `passive` | bool | Whether ability is passive |
| `ability_active` | bool | Whether ability is currently active/toggled |
| `cooldown` | int | Remaining cooldown in seconds |
| `ultimate` | bool | Whether this is the ultimate ability |
| `charges` | int | Current charges (for charge-based abilities) |
| `max_charges` | int | Maximum charges |
| `charge_cooldown` | int | Cooldown per charge |

Abilities are indexed as `ability0` through `ability5` (or more for heroes with extra abilities).

---

### 6. Items

Local hero's inventory and stash. Available from PRE_GAME onward.

```json
{
    "items": {
        "slot0": {
            "name": "item_power_treads",
            "purchaser": 0,
            "item_level": 0,
            "contains_rune": "",
            "can_cast": false,
            "cooldown": 0,
            "passive": true,
            "item_charges": 0,
            "ability_charges": 0,
            "max_charges": 0,
            "charge_cooldown": 0,
            "charges": 0
        },
        "slot1": { ... },
        "slot2": { ... },
        "slot3": { ... },
        "slot4": { ... },
        "slot5": { ... },
        "stash0": { ... },
        "stash1": { ... },
        "stash2": { ... },
        "stash3": { ... },
        "stash4": { ... },
        "stash5": { ... },
        "teleport0": { ... },
        "neutral0": { ... }
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Internal item name (e.g., `"item_power_treads"`) or `"empty"` |
| `purchaser` | int | Player slot of who bought the item |
| `item_level` | int | Item level (for upgradeable items) |
| `contains_rune` | string | Rune stored in bottle (e.g., `"haste"`, `"illusion"`) |
| `can_cast` | bool | Whether item active can be used |
| `cooldown` | int | Remaining cooldown |
| `passive` | bool | Whether item is passive |
| `item_charges` | int | Current charges (wards, dust, etc.) |
| `ability_charges` | int | Ability-granted charges |
| `max_charges` | int | Maximum charges |
| `charge_cooldown` | int | Cooldown per charge |
| `charges` | int | Total charges |

Slots: `slot0`-`slot5` (inventory), `stash0`-`stash5` (stash), `teleport0` (TP scroll), `neutral0` (neutral item).

---

### 7. Buildings

All tower and barracks health for both teams. Available from PRE_GAME onward.

```json
{
    "buildings": {
        "radiant": {
            "dota_goodguys_tower1_top": { "health": 1800, "max_health": 1800 },
            "dota_goodguys_tower2_top": { "health": 2500, "max_health": 2500 },
            "dota_goodguys_tower3_top": { "health": 2500, "max_health": 2500 },
            "dota_goodguys_tower1_mid": { "health": 1800, "max_health": 1800 },
            "dota_goodguys_tower2_mid": { "health": 2500, "max_health": 2500 },
            "dota_goodguys_tower3_mid": { "health": 2500, "max_health": 2500 },
            "dota_goodguys_tower1_bot": { "health": 1800, "max_health": 1800 },
            "dota_goodguys_tower2_bot": { "health": 2500, "max_health": 2500 },
            "dota_goodguys_tower3_bot": { "health": 2500, "max_health": 2500 },
            "dota_goodguys_tower4_top": { "health": 2600, "max_health": 2600 },
            "dota_goodguys_tower4_bot": { "health": 2600, "max_health": 2600 },
            "good_rax_melee_top": { "health": 2200, "max_health": 2200 },
            "good_rax_range_top": { "health": 1300, "max_health": 1300 },
            "good_rax_melee_mid": { "health": 2200, "max_health": 2200 },
            "good_rax_range_mid": { "health": 1300, "max_health": 1300 },
            "good_rax_melee_bot": { "health": 2200, "max_health": 2200 },
            "good_rax_range_bot": { "health": 1300, "max_health": 1300 },
            "dota_goodguys_fort": { "health": 4500, "max_health": 4500 }
        },
        "dire": {
            "dota_badguys_tower1_top": { "health": 1800, "max_health": 1800 },
            "...": "same structure as radiant"
        }
    }
}
```

Each building has:

| Field | Type | Description |
|-------|------|-------------|
| `health` | int | Current health (0 = destroyed) |
| `max_health` | int | Maximum health |

Building naming convention:
- Towers: `dota_goodguys_tower{tier}_{lane}` (radiant) / `dota_badguys_tower{tier}_{lane}` (dire)
- Barracks: `good_rax_{melee|range}_{lane}` / `bad_rax_{melee|range}_{lane}`
- Ancient: `dota_goodguys_fort` / `dota_badguys_fort`
- Tiers: 1 (outer), 2 (mid), 3 (base), 4 (ancient flanking)
- Lanes: `top`, `mid`, `bot`

---

### 8. Draft (Spectator-Primary)

Pick and ban data. **Full data only available to spectators/observers.** In player mode, this may be empty or partial.

```json
{
    "draft": {
        "activeteam": 2,
        "pick": true,
        "activeteam_time_remaining": 25.0,
        "radiant_bonus_time": 110.0,
        "dire_bonus_time": 120.0,
        "team2": {
            "home_team": true,
            "pick0_id": 1,
            "pick0_class": "npc_dota_hero_antimage",
            "pick1_id": 25,
            "pick1_class": "npc_dota_hero_lina",
            "pick2_id": 0,
            "pick2_class": "",
            "pick3_id": 0,
            "pick3_class": "",
            "pick4_id": 0,
            "pick4_class": "",
            "ban0_id": 2,
            "ban0_class": "npc_dota_hero_axe",
            "ban1_id": 0,
            "ban1_class": "",
            "ban2_id": 0,
            "ban2_class": "",
            "ban3_id": 0,
            "ban3_class": "",
            "ban4_id": 0,
            "ban4_class": "",
            "ban5_id": 0,
            "ban5_class": ""
        },
        "team3": {
            "home_team": false,
            "pick0_id": 50,
            "pick0_class": "npc_dota_hero_dazzle",
            "...": "same structure"
        }
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `activeteam` | int | Team currently picking/banning (2=Radiant, 3=Dire) |
| `pick` | bool | `true` = current action is pick, `false` = ban |
| `activeteam_time_remaining` | float | Seconds remaining for active team |
| `radiant_bonus_time` | float | Radiant reserve time in seconds |
| `dire_bonus_time` | float | Dire reserve time in seconds |

Per team (`team2` = Radiant, `team3` = Dire):

| Field | Type | Description |
|-------|------|-------------|
| `home_team` | bool | Whether this is the home team |
| `pick0_id` - `pick4_id` | int | Hero ID of pick in slot 0-4 (0 = not yet picked) |
| `pick0_class` - `pick4_class` | string | Hero internal name of pick (empty = not yet picked) |
| `ban0_id` - `ban5_id` | int | Hero ID of ban in slot 0-5 (0 = not banned) |
| `ban0_class` - `ban5_class` | string | Hero internal name of ban (empty = not banned) |

---

## Player Mode vs Spectator Mode

| Data Category | Player Mode | Spectator Mode |
|--------------|-------------|----------------|
| `provider` | Full | Full |
| `map` | Full | Full |
| `player` | **Local player only** | All 10 players |
| `hero` | **Local hero only** | All 10 heroes |
| `abilities` | **Local hero only** | All 10 heroes |
| `items` | **Local hero only** | All 10 heroes |
| `buildings` | Full (both teams) | Full (both teams) |
| `draft` | **Partial/Empty** | Full |
| `wearables` | Local player only | All players |

---

## Hero ID to Name Mapping

Hero IDs in GSI (`hero.id`, `draft.team2.pick0_id`, etc.) match the OpenDota API hero IDs.

Fetch the complete mapping from: `GET https://api.opendota.com/api/heroes`

Each hero object:
```json
{
    "id": 1,
    "name": "npc_dota_hero_antimage",
    "localized_name": "Anti-Mage",
    "primary_attr": "agi",
    "attack_type": "Melee",
    "roles": ["Carry", "Escape", "Nuker"],
    "legs": 2
}
```

The `name` field in GSI (`hero.name`, `draft.team2.pick0_class`) matches the `name` field from OpenDota.

---

## Sources

- [Dota2GSI C# Library](https://github.com/antonpup/Dota2GSI) — comprehensive field reference
- [dota2-gsi Node.js Library](https://github.com/xzion/dota2-gsi) — event key documentation
- [SteamDatabase GameTracking-Dota2](https://github.com/SteamDatabase/GameTracking-Dota2/blob/master/Protobufs/dota_shared_enums.proto) — official enum values
- [OpenDota API](https://docs.opendota.com/) — hero data and matchup endpoints
- [Overwolf Dota 2 GEP](https://dev.overwolf.com/ow-native/reference/live-game-data-gep/supported-games/dota-2/) — data availability notes
