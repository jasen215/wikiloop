//go:build fts5 && windows

// Vector embedding is not supported on Windows (libtokenizers has no Windows
// prebuilt). FTS keyword search works normally; embed/vector commands are no-ops.
package main

import (
	"fmt"

	"github.com/jasen215/wikiloop/internal/config"
	"github.com/jasen215/wikiloop/internal/kb"
)

func preflightCheckEmbedWarns(_ string, _ *config.Config) []string {
	return []string{"Vector embedding is not supported on Windows — FTS search still works."}
}

func setupRuntimeDeps(_ *config.Config, _ string, _ bool) error { return nil }

func makeEmbedder(_ string, _ *config.Config) kb.Embedder { return nil }

func runEmbedStep(_ string, _ *config.Config) {}

func runEmbed(_ string, _ []string) error {
	return fmt.Errorf("vector embedding is not supported on Windows")
}
