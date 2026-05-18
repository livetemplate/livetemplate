# Plan: Lifestyle Planner — LiveTemplate Showcase App

## Context

LiveTemplate needs a flagship app for non-developers that solves a real problem and showcases the library's reactive features. After extensive research, we converged on this insight:

> "People don't always want the cheapest stuff. They want to maintain a good lifestyle while being on a certain budget."

This applies to everyone — parents with kids, couples saving for a house, singles new to Berlin, roommates splitting costs. Anyone who wants to be intentional about their money without being miserable about it.

This isn't a budget tracker (backwards-looking, guilt-driven). It's a **lifestyle planner** (forwards-looking, empowering): "Given your budget, here's what to buy and what to do this week."

Nothing like this exists. Budget apps track spending. Deal apps find discounts. Meal planners suggest recipes. This app **closes the loop** — it recommends concrete products to buy, activities to do, and ways to save, all tailored to your budget and preferences.

## The App

**Working name:** TBD (to be decided with user — candidates: Sparfuchs, HouseholdPlan, Wochenplaner)

**One-liner:** "Your weekly plan — what to eat, what to do, what to save — fitted to your budget."

### How It Works

1. **Instant start:** Pick a persona → immediately see your weekly plan. No forms, no onboarding wizard.
   
   **Prepackaged personas (examples):**
   - "Student — €800/month" (1 person, tight budget, nightlife + culture)
   - "New in Berlin — €2,500/month" (1 person, exploring the city, balanced)
   - "Couple saving up — €3,500/month" (2 people, saving goal, eat well)
   - "Young family — €2,800/month" (3-4 people, kid-friendly activities, groceries)
   - "WG life — €1,200/month" (shared, split costs, social activities)
   
   Each persona pre-fills: household size, budget, priorities, ZIP (default: Berlin central). All editable later.

2. **Every week, the app suggests:**
   - **Grocery bundle** — a curated shopping list with recommended products, where to buy them, total cost. Uses seasonal ingredients, fits your food budget, shows which store is cheapest for each item
   - **Things to do** — free/cheap activities this weekend (parks, museums, events, workshops, nightlife, sports). Pulled from Berlin cultural data + event APIs
   - **Smart savings** — "switch from X to Y and save €Z/month", cheaper alternatives for things you already buy, subscription audit
3. **Household collaboration** — housemates, couples, or household members all see and edit the plan in real-time
4. **Benchmarks** — "Berlin households of 2 with your income typically spend €380/month on groceries — your plan is €350. You have room for an extra dinner out."

### Who It's For

- **Parents with kids** — most price-sensitive, highest motivation, need activity ideas
- **Couples saving for a goal** — house, wedding, travel fund — want to live well while saving
- **Singles new to a city** — "what should I do this weekend?" + "how do I make €2,500 work in Berlin?"
- **Roommates / WGs** — shared grocery budgets, split costs

### What Makes It Sticky

- **Weekly cadence** — new recommendations every week = reason to come back
- **Money saved is visible** — "You've saved €340 this month vs average" — hard to give up
- **Closes the loop** — doesn't just say "spend less on food", says "here's a €85 grocery list for the week"
- **Lifestyle-positive** — suggests things to DO, not just things to cut
- **Shared household** — housemates/partners use it together, creating social accountability

### Open Data Sources

| Source | What It Provides | Access |
|--------|-----------------|--------|
| **DESTATIS GENESIS API** | Household spending benchmarks by household size, income, region | Free, open data license, REST API |
| **kulturdaten.berlin API** | Berlin cultural events, venues, exhibitions, workshops | Free, open source (Technologiestiftung Berlin) |
| **Open Food Facts API** | 2.8M products, nutrition, Nutri-Score, barcode lookup | Free, no auth, Open Database License |
| **Open Prices (OFF)** | Crowdsourced grocery price history | Free, open source, REST API |
| **Marktguru** (community API) | Weekly supermarket deals by ZIP code | Reverse-engineered, community wrappers exist |
| **Eventbrite / visitBerlin** | Free household events in Berlin | Free event listings |

### LiveTemplate Features Showcased

| Feature | How It's Used |
|---------|--------------|
| **Broadcasting / PubSub** | Housemate edits grocery list or picks an activity → appears instantly on everyone's screen |
| **Range operations** | Grocery list items added/removed/checked off; activity suggestions appearing |
| **Minimal diffs** | Price updates, budget progress bars, countdown to events |
| **Server-initiated actions** | Weekly recommendation refresh, new event alerts, deal price changes |
| **Controller+State** | Controller: API clients, recommendation engine, caches. State: household budget, list, preferences |
| **Progressive enhancement** | Works without JS — check your grocery list in-store on bad mobile signal |
| **Session management** | Household sharing via session groups, per-user preferences |

### Feature Priority

**P0 — MVP (validates the concept):**
1. Persona picker — tap a card, instantly see your weekly plan (zero setup)
2. Budget allocation with DESTATIS benchmarks ("households like yours spend X on Y")
3. Weekly grocery bundle — curated list with prices from Marktguru deals + Open Food Facts product data
4. "Best store this week" — which store is cheapest for your bundle
5. Shared household view with real-time sync
6. Progressive enhancement (works without JS)
7. "Customize" drawer — edit persona values (budget, size, ZIP, priorities) after you're already using the app

**P1 — Makes it useful weekly:**
7. Activities & events — free/cheap household things to do this week (kulturdaten.berlin)
8. Price history per product — "is this deal actually good?" (Open Prices)
9. Recurring items — things you buy every week auto-added
10. Budget vs actual tracking — simple progress bars per category

**P2 — Growth & retention:**
11. Smart savings suggestions — cheaper alternatives, subscription audit
12. Seasonal produce recommendations
13. Dietary preferences (vegan, gluten-free via Open Food Facts nutrition data)
14. Savings milestone celebrations ("You've saved €500 since you started!")

### Architecture

```
app/
├── main.go                     # HTTP server, LiveTemplate setup
├── controllers/
│   ├── planner.go              # Main controller: API clients, caches, recommendation engine
│   └── personas.go             # Persona picker + customization
├── state/
│   └── household.go               # HouseholdState: budget, list, preferences, recommendations
├── services/
│   ├── marktguru/client.go     # Deal fetcher (wraps community API)
│   ├── openfoodfacts/client.go # Product lookup
│   ├── openprices/client.go    # Price history
│   ├── kulturdaten/client.go   # Berlin events/activities
│   ├── destatis/client.go      # Spending benchmarks
│   ├── recommender.go          # Grocery bundle + activity recommendation engine
│   └── cache.go                # Shared TTL cache
├── templates/
│   ├── layout.html             # Base layout
│   ├── personas.html           # Persona picker cards
│   ├── dashboard.html          # Weekly plan overview
│   ├── grocery.html            # Grocery bundle + shopping list
│   ├── activities.html         # Suggested activities
│   └── budget.html             # Budget allocation + benchmarks
├── static/
│   └── style.css               # Mobile-first CSS
├── go.mod
└── go.sum
```

**Controller (singleton):**
```go
type PlannerController struct {
    Deals       *marktguru.Client
    Foods       *openfoodfacts.Client
    Prices      *openprices.Client
    Events      *kulturdaten.Client
    Benchmarks  *destatis.Client
    Recommender *recommender.Engine
    Cache       *cache.Store
}
```

**State (per-household session):**
```go
type HouseholdState struct {
    // Persona (pre-filled on selection, editable later)
    PersonaID     string          `json:"persona_id"` // e.g., "student", "couple_saving", "young_family"
    HouseholdSize int             `json:"household_size"`
    ZipCode       string          `json:"zip_code"`
    MonthlyBudget float64         `json:"monthly_budget"`
    Priorities    []string        `json:"priorities"` // e.g., ["eating_well", "going_out", "fitness"]
    
    // Budget allocation
    Allocations  map[string]float64 `json:"allocations"` // category → monthly €
    Benchmarks   map[string]float64 `json:"benchmarks" lvt:"transient"` // DESTATIS data
    
    // Weekly plan
    GroceryBundle []BundleItem      `json:"grocery_bundle"`
    Activities    []Activity        `json:"activities" lvt:"transient"`
    SavingsTips   []SavingTip       `json:"savings_tips" lvt:"transient"`
    
    // Tracking
    WeeklySpend  map[string]float64 `json:"weekly_spend"`
    TotalSaved   float64            `json:"total_saved"`
}
```

### Differentiation

| | YNAB | Marktguru | Cozi | **This App** |
|---|---|---|---|---|
| Tracks spending | Yes | No | No | Simple |
| Suggests what to buy | No | Deals only | No | **Curated bundles** |
| Suggests what to do | No | No | No | **Activities + events** |
| Household real-time sync | No | No | Yes | **Yes** |
| Budget benchmarks | No | No | No | **DESTATIS data** |
| Privacy-first | No (bank access) | No (tracks users) | No | **Yes, self-hostable** |
| Open source | No | No | No | **Yes** |
| Cost | €15/month | Free (data is the cost) | Free (ads) | **Free** |

### Target Audience & Launch

**Primary:** Budget-conscious Berlin residents — parents, couples, singles, WGs
**Launch channels:** r/berlin, r/Finanzen, r/selfhosted, mydealz.de, Berlin expat groups
**Growth path:** Berlin → other German cities → European cities with open data

### Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Marktguru API instability | Abstract behind interface; fallback to Open Prices + manual entry |
| Recommendation quality at launch | Start simple (cheapest items per category), iterate based on feedback |
| "Just another budget app" perception | Lead with recommendations, not tracking. First screen = "here's your week" |
| Berlin-only initially | Architecture supports pluggable data providers; kulturdaten.berlin is the template for other city APIs |

## Implementation Phases

### Phase 1: Data Services + Core State (Week 1)
- Go module with LiveTemplate dependency
- Marktguru client (deal search by ZIP + keyword)
- Open Food Facts client (product lookup by name/barcode)
- DESTATIS client (spending benchmarks for household profiles)
- Basic recommendation engine (match deals to common grocery categories)
- Tests for all data services

### Phase 2: Persona Picker + Dashboard (Week 2)  
- PlannerController + HouseholdState
- Persona picker template (5 cards, one tap to start)
- Budget allocation view with DESTATIS benchmarks
- Dashboard template showing weekly plan overview
- Customize drawer (edit budget, size, ZIP, priorities after initial pick)
- Actions: SelectPersona, UpdateProfile, UpdateAllocation

### Phase 3: Grocery Bundle (Week 3)
- Grocery bundle recommendation engine
- Shopping list template with add/remove/check-off
- "Best store" calculation per item and overall
- Price display with deal validity dates
- Actions: AddItem, RemoveItem, CheckItem, RefreshBundle

### Phase 4: Real-Time + Household Sharing (Week 4)
- Broadcasting setup for household session groups
- Server-initiated weekly recommendation refresh
- Activity recommendations (kulturdaten.berlin integration)
- Budget progress bars (category spend tracking)
- Progressive enhancement verification

### Phase 5: Polish + Launch (Week 5)
- Docker setup for self-hosting
- Mobile-first CSS polish
- README with screenshots
- Demo instance deployment
- Community launch (Reddit, Berlin forums)

## Verification

1. **Unit tests**: Each data service client, recommendation engine, budget calculations
2. **Integration test**: Full onboarding → grocery bundle → activity suggestions flow
3. **Real-time test**: Two browser tabs as "household members" — add item on one, appears on other
4. **Progressive enhancement**: Disable JS, verify core flows work via HTTP POST
5. **Privacy**: No shopping data or budget info sent to third parties
6. **Benchmark accuracy**: Compare DESTATIS data against known household statistics

## Key Files to Reference (LiveTemplate)

- `mount.go` — Handler setup, session groups, broadcasting
- `context.go` — Form data (GetString, GetInt, BindAndValidate)
- `state.go` — State interface, AsState[T], transient fields (`lvt:"transient"`)
- `dispatch.go` — Action name → method routing
- `config.go` — Env-based configuration
- `docs/references/server-actions.md` — TriggerAction for periodic refresh
- `docs/references/controller-pattern.md` — Controller+State examples
