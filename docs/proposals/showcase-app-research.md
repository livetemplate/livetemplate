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

## Next Steps

1. **Prototype**: Build a minimal transit dashboard for one city (suggest NYC MTA or London TfL — both have excellent real-time APIs)
2. **Validate**: Share with transit communities for feedback
3. **Expand**: Add more agencies via GTFS-RT standard
4. **Community**: Open-source the app as a reference implementation for LiveTemplate
