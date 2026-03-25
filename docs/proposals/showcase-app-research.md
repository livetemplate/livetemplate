# Showcase App Research: Real-Time Public Transit Dashboard

## Decision

After evaluating 8 niches across open data availability, real-time value, user retention potential, competition gaps, and LiveTemplate feature coverage, **Real-Time Public Transit Dashboard** was selected as the flagship showcase app.

## Why Transit?

### The Problem
Commuters need quick, glanceable transit info. Existing apps are heavy, ad-laden, proprietary, and don't show system-wide status well. There is no good open-source, lightweight, privacy-respecting transit dashboard.

### Why It Attracts Real Users
- **Daily repeat usage**: Commuters check transit 2-4x per day
- **Universal need**: Every city dweller with public transit is a potential user
- **Real-time is the core value**: Not a gimmick — stale transit data is useless
- **Progressive enhancement matters**: Commuters on underground/bad connections benefit from HTTP-first design

### Open Data Sources (All Free)

| Source | Coverage | Format | Real-Time? |
|--------|----------|--------|:----------:|
| GTFS Static | 1000+ agencies worldwide | Standardized feed | No (schedule) |
| GTFS-RT | Major agencies (MTA, BART, MBTA, TriMet, etc.) | Protocol Buffers | Yes |
| TfL Open Data | London | REST + WebSocket | Yes |
| Helsinki HSL | Finland | MQTT | Yes |
| Open Rail Data | UK rail network | STOMP protocol | Yes |

GTFS (General Transit Feed Specification) is the global standard. Over 1000 transit agencies publish GTFS data. GTFS-RT extends it with real-time vehicle positions, trip updates, and service alerts.

### How It Showcases LiveTemplate

| LiveTemplate Feature | Transit Dashboard Usage |
|---------------------|------------------------|
| **Minimal tree diffs** | Arrival ETAs updating every 5-15 seconds — only changed countdown values sent |
| **Range operations (insert/remove)** | New arrivals appear, departed vehicles removed from list |
| **Range operations (reorder)** | Arrivals reorder as ETAs change |
| **Range operations (update)** | Individual vehicle status changes (delayed, cancelled) |
| **Server-initiated actions** | Background polling of GTFS-RT feeds, push updates to clients |
| **Broadcasting / PubSub** | Service alerts broadcast to all users on affected routes |
| **Controller+State pattern** | Controller: transit API clients, caches. State: user's favorite stops, selected routes |
| **Progressive enhancement** | Full schedule works without JS; JS adds live countdown updates |
| **Session management** | Favorite stops persist across sessions |
| **HTTP-first** | Initial page render with current schedule data, no WebSocket required |

### Competition Analysis

| App | Model | Weakness |
|-----|-------|----------|
| Google Maps | Proprietary, ad-supported | Heavy, data-harvesting, no system-wide view |
| Citymapper | Proprietary, VC-funded | Heavy app, limited to major cities |
| Transit App | Proprietary | Ad-supported, privacy concerns |
| Agency-specific apps | Government-built | Usually outdated UI, single-agency only |
| **Gap** | **Open-source, self-hostable** | **No good option exists** |

### Target Audience
- Daily commuters (primary — highest retention)
- Transit enthusiasts / railfans
- City planners and transportation researchers
- Developers wanting to build transit tools

### Retention Drivers
- Customizable favorite stops/routes
- Personalized dashboard layout
- Historical reliability data ("this bus is late 40% of the time")
- Service alert notifications
- Multi-modal support (bus + train + bike-share)

## Other Niches Evaluated

| Niche | Score | Why Not Selected |
|-------|:-----:|-----------------|
| Air Quality Monitor | A | Strong runner-up. Less frequent daily usage than transit. Could be a future Phase 2 addition. |
| OSS Ecosystem Dashboard | A- | Targets developers (good for community), but narrower audience. |
| Gov Spending Explorer | A- | Interesting civic tech angle, but real-time value is weaker. |
| Earthquake/Disaster Monitor | A- | Great real-time showcase, but usage is event-driven (not daily habit). |
| Legislative Tracker | B+ | Session-driven usage, not daily habit. |
| Crypto/Finance Explorer | B+ | High retention but saturated market (TradingView). |
| News/RSS Aggregator | B+ | Lots of competition (Feedly, Miniflux). |

## Berlin: First City Implementation

### Why Berlin?
- **Multi-modal**: U-Bahn, S-Bahn, Tram, Bus, Ferry — showcases different transport types in one dashboard
- **High frequency**: Trains every 2-5 min during rush hour = lots of real-time updates
- **Large tech community**: Good pool of early adopters and potential contributors
- **Strong open data culture**: Active civic tech scene
- **Pain points are well-known**: S-Bahn unreliability (Ringbahn meme), bloated BVG app, confusing Ersatzverkehr

### Berlin Transit APIs

**Primary: `v6.bvg.transport.rest` (by derhuerst)**
- No authentication, free, JSON REST API
- Wraps BVG/VBB HAFAS backend
- Key endpoints:
  - `GET /stops/{id}/departures` — Real-time departures with delays
  - `GET /stops/{id}/arrivals` — Arrivals at a stop
  - `GET /locations?query={name}` — Search stops by name
  - `GET /locations/nearby?latitude={lat}&longitude={lon}` — Nearby stops
  - `GET /radar?north=&west=&south=&east=` — Vehicle positions in bounding box
  - `GET /journeys?from={id}&to={id}` — Route planning
- Response includes: line info, direction, planned/actual times, delay in seconds, platform, remarks (disruptions)
- Self-hostable via `hafas-rest-api` for production use

**Also available:**
- `v6.vbb.transport.rest` — Broader VBB network (Berlin + Brandenburg)
- VBB GTFS Static — Schedule data from `daten.berlin.de` or `vbb.de` (CSV/ZIP, updated monthly)
- GTFS-RT — Via `opendata-oepnv.de` (free registration, protobuf format)
- BVG FaSta API — Elevator/escalator status for accessibility

### Berlin-Specific Pain Points to Solve

| Pain Point | How the App Addresses It |
|-----------|--------------------------|
| S-Bahn Ringbahn disruptions | Prominent disruption banner at top of dashboard |
| BVG app is slow and bloated | Zero-JS initial load, instant departure info |
| Ersatzverkehr confusion | Clear replacement service indicators with affected routes |
| Evening frequency drop (10 min gaps) | Exact countdowns matter more after 20:00 |
| Multi-operator complexity (BVG + DB S-Bahn) | Unified view across all operators |
| Night service confusion | Time-of-day aware display ("next service at 04:30") |

### UI Design: Glanceable Departure Board

Inspired by German station departure boards (Abfahrtstafel):

```
┌─────────────────────────────────────────────────┐
│ ⚠ S41/S42 Ringbahn: Einschränkungen Südkreuz   │
│   ↔ Neukölln. Ersatzverkehr mit Bussen.         │
├─────────────────────────────────────────────────┤
│ 📍 S+U Alexanderplatz                           │
│                                                 │
│ [S7]  S Westkreuz              2 min            │
│ [S5]  S Strausberg Nord        4 min            │
│ [U2]  Ruhleben                 1 min            │
│ [U5]  Hönow                    3 min   +2 min   │
│ [U8]  Hermannstraße            6 min            │
│ [M4]  Falkenberg               2 min            │
│ [Bus] 100 Zoologischer Garten  8 min            │
│                                                 │
│ 📍 U Weinmeisterstraße                          │
│                                                 │
│ [U8]  Wittenau                  1 min            │
│ [U8]  Hermannstraße             7 min            │
├─────────────────────────────────────────────────┤
│ Last updated: 3 seconds ago                     │
└─────────────────────────────────────────────────┘
```

**Visual elements:**
- Line badges with official BVG/DB colors (S-Bahn green, U-Bahn blue, Bus purple, Tram red)
- Countdown format ("2 min") not clock time ("14:37")
- Delay shown in red ("+2 min")
- Cancelled services struck through with ⚠
- Disruption banner impossible to miss

### Feature Priority

**P0 — Must Have (MVP):**
1. Favorite stops — pre-configured, zero-tap to view
2. Real-time countdowns — auto-updating via LiveTemplate diffs
3. Line badges with correct BVG/DB colors
4. Delay and cancellation indicators
5. Auto-refresh via server push (no manual reload)
6. Disruption banner for saved lines

**P1 — Should Have:**
7. Walking time offset — dim departures you can't make
8. Line filtering per stop — show only lines you care about
9. Multiple stops on one screen — "Home" and "Office" side by side
10. Dark mode — for permanent display / bedside screen

**P2 — Nice to Have:**
11. "Can I make it?" green/yellow/red indicator
12. Historical reliability stats
13. Browser notifications on disruptions

**Explicitly Out of Scope:**
- Trip planning (use Citymapper/Google Maps)
- Ticket purchasing
- Maps
- Account creation for basic usage

### Architecture (LiveTemplate Pattern)

```go
// CONTROLLER: Singleton, holds API client
type TransitController struct {
    Client  *hafas.Client   // VBB/BVG REST client
    Cache   *StopCache      // Shared departure cache
    Ticker  *time.Ticker    // 30s polling interval
}

// STATE: Per-session, user's configuration
type TransitState struct {
    FavoriteStops []FavoriteStop  // User's saved stops
    Departures    map[string][]Departure  // Current departures per stop
    Disruptions   []Disruption   // Active service alerts
    WalkingTimes  map[string]int // Minutes to reach each stop
    DarkMode      bool
}

// Server-initiated polling broadcasts to all sessions viewing same stops
func (c *TransitController) RefreshDepartures(state TransitState, ctx *livetemplate.Context) (TransitState, error) {
    for _, stop := range state.FavoriteStops {
        deps, _ := c.Client.Departures(stop.ID, 15) // next 15 minutes
        state.Departures[stop.ID] = deps
    }
    state.Disruptions = c.Client.ActiveDisruptions()
    return state, nil
}
```

**Data flow:**
1. Server polls VBB API every 30 seconds
2. Broadcasts departure updates to all connected sessions
3. LiveTemplate diffs: only changed countdown values + new/removed departures sent
4. Client DOM updates in-place — no page reload, no full list re-render

## Next Steps

1. **Prototype**: Build MVP with 2-3 Berlin stops using `v6.bvg.transport.rest`
2. **Validate**: Share in r/berlin, Berlin tech meetups, BVG-related communities
3. **Iterate**: Add P1 features based on user feedback
4. **Expand**: Self-host `hafas-rest-api`, add other cities via GTFS-RT
5. **Community**: Open-source as the LiveTemplate reference app
