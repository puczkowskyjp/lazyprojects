package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"lazyprojects/internal/config"
	"lazyprojects/internal/editor"
	"lazyprojects/internal/scanner"
	"lazyprojects/internal/ui"
)

func main() {
	listFlag := flag.Bool("list", false, "List discovered projects and exit")
	openFlag := flag.String("open", "", "Open a specific project by exact path")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if *openFlag != "" {
		if err := editor.OpenWithOptions(cfg.Editor, *openFlag, editor.TerminalOptions{
			App:    cfg.Terminal.App,
			Target: cfg.Terminal.Target,
		}); err != nil {
			log.Fatalf("Error opening project: %v", err)
		}
		fmt.Printf("Opened project %s in %s\n", *openFlag, cfg.Editor)
		return
	}

	if *listFlag {
		s := scanner.New(cfg)
		projects, err := s.Scan(context.Background())
		if err != nil {
			log.Fatalf("Error scanning projects: %v", err)
		}
		fmt.Printf("Discovered %d projects:\n\n", len(projects))
		for _, p := range projects {
			branchInfo := ""
			if p.Branch != "" {
				branchInfo = fmt.Sprintf(" (%s)", p.Branch)
			}
			fmt.Printf("• %-25s [%-6s] %s%s\n", p.Name, p.Type, p.Path, branchInfo)
		}
		return
	}

	appUI := ui.New(cfg)
	if err := appUI.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
