package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/puczkowskyjp/lazyprojects/internal/config"
	"github.com/puczkowskyjp/lazyprojects/internal/editor"
	"github.com/puczkowskyjp/lazyprojects/internal/scanner"
	"github.com/puczkowskyjp/lazyprojects/internal/ui"
)

var version = "development"

func resolveVersion() string {
	if version != "development" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	versionFlag := flag.Bool("version", false, "Print lazyprojects version and exit")
	listFlag := flag.Bool("list", false, "List discovered projects and exit")
	openFlag := flag.String("open", "", "Open a specific project by exact path")
	flag.Parse()
	version = resolveVersion()

	if *versionFlag {
		fmt.Printf("lazyprojects %s\n", version)
		os.Exit(0)
	}

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
		cfg.RecordRecentProject(*openFlag)
		if err := cfg.Save(); err != nil {
			log.Fatalf("Opened project, but failed to save recent projects: %v", err)
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

	appUI := ui.New(cfg, version)
	if err := appUI.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
