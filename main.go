package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lizhonghan/gorss/config"
	"github.com/lizhonghan/gorss/feed"
	"github.com/lizhonghan/gorss/ui"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize feed manager
	feeds := cfg.Feeds
	feedManager := feed.NewFeedManager(feeds)

	// Add an "All" feed option
	feeds = append([]config.Feed{{Name: "All", URL: ""}}, feeds...)
	feedManager.Feeds = feeds

	// Create and start the Bubble Tea program
	p := tea.NewProgram(
		ui.NewModel(feedManager),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
