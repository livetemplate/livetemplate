// Bandwidth Savings Demo - LiveTemplate Tree-Based Optimization
// Shows dramatic bandwidth reduction compared to full HTML replacement
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"

	"github.com/livefir/livetemplate/internal/strategy"
)

func main() {
	fmt.Println("🚀 LiveTemplate Bandwidth Savings Demo")
	fmt.Println("=====================================")

	// Create strategy selector
	selector := strategy.NewStrategySelector()

	// Demo template with mixed static and dynamic content
	templateSource := `<div class="dashboard">
	<header class="header">
		<h1>{{.Title}}</h1>
		<nav>
			<a href="/home">Home</a>
			<a href="/profile">Profile</a>
			<a href="/settings">Settings</a>
		</nav>
	</header>
	<main class="content">
		<div class="user-info">
			<h2>Welcome {{.User.Name}}!</h2>
			<p>Level: {{.User.Level}}</p>
			<p>Score: {{.User.Score}}</p>
			<p>Status: {{.User.Status}}</p>
		</div>
		<div class="stats">
			<div class="stat-card">
				<span class="label">Total Points</span>
				<span class="value">{{.Stats.Points}}</span>
			</div>
			<div class="stat-card">
				<span class="label">Achievements</span>
				<span class="value">{{.Stats.Achievements}}</span>
			</div>
			<div class="stat-card">
				<span class="label">Rank</span>
				<span class="value">{{.Stats.Rank}}</span>
			</div>
		</div>
		<footer>
			<p>Last updated: {{.LastUpdated}}</p>
		</footer>
	</main>
</div>`

	tmpl := template.Must(template.New("demo").Parse(templateSource))

	// Initial data
	initialData := map[string]interface{}{
		"Title": "Gaming Dashboard",
		"User": map[string]interface{}{
			"Name":   "Alice",
			"Level":  "Gold",
			"Score":  1250,
			"Status": "Online",
		},
		"Stats": map[string]interface{}{
			"Points":       15420,
			"Achievements": 24,
			"Rank":         "#42",
		},
		"LastUpdated": "2025-01-15 14:30:00",
	}

	// Updated data (only user score and stats changed)
	updatedData := map[string]interface{}{
		"Title": "Gaming Dashboard",
		"User": map[string]interface{}{
			"Name":   "Alice",
			"Level":  "Gold",
			"Score":  1275, // Changed: +25 points
			"Status": "Online",
		},
		"Stats": map[string]interface{}{
			"Points":       15445, // Changed: +25 points
			"Achievements": 24,
			"Rank":         "#41", // Changed: rank improved
		},
		"LastUpdated": "2025-01-15 14:31:00", // Changed: timestamp
	}

	fmt.Println("1. Full HTML Replacement (Traditional Approach)")
	fmt.Println("----------------------------------------------")

	// Calculate full HTML size
	var fullHTML bytes.Buffer
	if err := tmpl.Execute(&fullHTML, updatedData); err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
	fullHTMLBytes := fullHTML.Bytes()
	fullHTMLSize := len(fullHTMLBytes)

	fmt.Printf("Full HTML Size: %d bytes\n", fullHTMLSize)
	fmt.Printf("Full HTML Content:\n%s\n\n", string(fullHTMLBytes))

	fmt.Println("2. Tree-Based Optimization (LiveTemplate)")
	fmt.Println("----------------------------------------")

	// First render - includes static structure
	fmt.Println("a) Initial Render (includes statics):")
	firstResult, strategyType, err := selector.GenerateUpdate(
		templateSource,
		tmpl,
		nil, // No old data for initial render
		initialData,
		"dashboard",
	)
	if err != nil {
		log.Fatalf("Failed to generate initial render: %v", err)
	}

	firstJSON, _ := json.Marshal(firstResult)
	firstSize := len(firstJSON)
	fmt.Printf("   Strategy: %s\n", strategyType.String())
	fmt.Printf("   Size: %d bytes (includes cached static structure)\n", firstSize)

	// Subsequent update - only dynamics
	fmt.Println("\nb) Incremental Update (only dynamics):")
	result, strategyType, err := selector.GenerateUpdate(
		templateSource,
		tmpl,
		initialData,
		updatedData,
		"dashboard",
	)
	if err != nil {
		log.Fatalf("Failed to generate update: %v", err)
	}

	updateJSON, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("Failed to marshal update: %v", err)
	}
	updateSize := len(updateJSON)

	fmt.Printf("   Strategy: %s\n", strategyType.String())
	fmt.Printf("   Size: %d bytes (dynamics only, statics cached client-side)\n", updateSize)
	fmt.Printf("   Content: %s\n", string(updateJSON))

	// Calculate savings for incremental updates
	savings := fullHTMLSize - updateSize
	savingsPercent := float64(savings) / float64(fullHTMLSize) * 100

	fmt.Println("\n3. Bandwidth Savings Analysis (Incremental Updates)")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Full HTML:        %d bytes\n", fullHTMLSize)
	fmt.Printf("Tree Update:      %d bytes\n", updateSize)
	fmt.Printf("Bytes Saved:      %d bytes\n", savings)
	fmt.Printf("Savings:          %.1f%%\n\n", savingsPercent)

	// Demonstrate multiple updates
	fmt.Println("4. Multiple Updates Simulation")
	fmt.Println("-----------------------------")

	totalFullHTML := 0
	totalTreeUpdates := 0
	updateCount := 5

	for i := 1; i <= updateCount; i++ {
		// Create slightly different data for each update
		newData := map[string]interface{}{
			"Title": "Gaming Dashboard",
			"User": map[string]interface{}{
				"Name":   "Alice",
				"Level":  "Gold",
				"Score":  1250 + (i * 25),
				"Status": "Online",
			},
			"Stats": map[string]interface{}{
				"Points":       15420 + (i * 25),
				"Achievements": 24,
				"Rank":         fmt.Sprintf("#%d", 42-i),
			},
			"LastUpdated": fmt.Sprintf("2025-01-15 14:3%d:00", i),
		}

		// Full HTML approach
		var htmlBuf bytes.Buffer
		if err := tmpl.Execute(&htmlBuf, newData); err != nil {
			log.Printf("Failed to execute template for update %d: %v", i, err)
			continue
		}
		htmlSize := len(htmlBuf.Bytes())
		totalFullHTML += htmlSize

		// Tree-based approach
		treeResult, _, _ := selector.GenerateUpdate(templateSource, tmpl, initialData, newData, "dashboard")
		treeJSON, _ := json.Marshal(treeResult)
		treeSize := len(treeJSON)
		totalTreeUpdates += treeSize

		fmt.Printf("Update %d: Full HTML %d bytes, Tree %d bytes (%.1f%% savings)\n",
			i, htmlSize, treeSize, float64(htmlSize-treeSize)/float64(htmlSize)*100)
	}

	totalSavings := totalFullHTML - totalTreeUpdates
	totalSavingsPercent := float64(totalSavings) / float64(totalFullHTML) * 100

	fmt.Println("\n5. Cumulative Bandwidth Analysis")
	fmt.Println("-------------------------------")
	fmt.Printf("Total Full HTML:     %d bytes\n", totalFullHTML)
	fmt.Printf("Total Tree Updates:  %d bytes\n", totalTreeUpdates)
	fmt.Printf("Total Bytes Saved:   %d bytes\n", totalSavings)
	fmt.Printf("Average Savings:     %.1f%%\n", totalSavingsPercent)

	fmt.Println("\n🎉 Demo Complete! LiveTemplate tree-based optimization provides")
	fmt.Printf("   massive bandwidth savings with %.1f%% reduction on average.\n", totalSavingsPercent)
}
