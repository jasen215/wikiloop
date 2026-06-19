//go:build fts5

package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/jasen215/wikiloop/internal/config"
	"github.com/jasen215/wikiloop/internal/embed"
	"github.com/jasen215/wikiloop/internal/kb"
)

// preflightCheckEmbedWarns returns warnings about missing ONNX runtime or model.
func preflightCheckEmbedWarns(kbRoot string, cfg *config.Config) []string {
	var warns []string
	if _, err := embed.FindOrtLib(cfg.Runtime.OrtLib, kbRoot); err != nil {
		warns = append(warns, "ONNX runtime not found — vector search disabled (FTS still works).\n"+
			"      Install with: brew install onnxruntime")
	}
	if embed.FindModelDir(filepath.Join(kbRoot, "models")) == "" {
		warns = append(warns, fmt.Sprintf(
			"Embedding model not found in %s/models/ — vector search disabled (FTS still works).\n"+
				"      Download bge-small-zh.tar.gz and extract to %s/models/:\n"+
				"      https://github.com/jasen215/wikiloop/releases/tag/models-v1\n"+
				"      tar -xzf bge-small-zh.tar.gz -C $WIKILOOP_KB/models/",
			kbRoot, kbRoot))
	}
	return warns
}

// setupRuntimeDeps locates and configures the ONNX Runtime shared library.
// Required for vector embedding (serve, embed). When verbose, the path is logged.
func setupRuntimeDeps(cfg *config.Config, kbRoot string, verbose bool) error {
	ortPath, err := embed.FindOrtLib(cfg.Runtime.OrtLib, kbRoot)
	if err != nil {
		return fmt.Errorf("onnxruntime: %w", err)
	}
	ort.SetSharedLibraryPath(ortPath)
	if verbose {
		log.Printf("onnxruntime: %s", ortPath)
	}
	return nil
}

// makeEmbedder creates the ONNX embedder if a model dir is found. Returns nil otherwise.
func makeEmbedder(kbRoot string, cfg *config.Config) kb.Embedder {
	modelDir := embed.FindModelDir(filepath.Join(kbRoot, "models"))
	if modelDir == "" {
		return nil
	}
	idleTimeout := cfg.Embedding.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 5 * time.Minute
	}
	return embed.NewONNXEmbedder(modelDir, 384, idleTimeout)
}

// runEmbedStep embeds any un-embedded documents using the ONNX model.
// Skips silently if no model directory is found.
func runEmbedStep(kbRoot string, cfg *config.Config) {
	embedder := makeEmbedder(kbRoot, cfg)
	if embedder == nil {
		return
	}

	db, err := kb.OpenDB(kbRoot)
	if err != nil {
		log.Printf("embed step: open db: %v", err)
		return
	}
	defer db.Close()

	modelName := embed.ModelName(embed.FindModelDir(filepath.Join(kbRoot, "models")))

	written, skipped, err := kb.EmbedDocuments(db, kbRoot, embedder, modelName, false)
	if err != nil {
		log.Printf("embed step: %v", err)
		return
	}
	if written > 0 {
		log.Printf("embed step: %d embedded, %d skipped", written, skipped)
	}
}

// runEmbed runs the embed subcommand.
func runEmbed(kbRoot string, args []string) error {
	fs := flag.NewFlagSet("embed", flag.ContinueOnError)
	full := fs.Bool("full", false, "drop and rebuild the vector store from scratch")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(kbRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := setupRuntimeDeps(cfg, kbRoot, false); err != nil {
		return err
	}

	modelDir := embed.FindModelDir(filepath.Join(kbRoot, "models"))
	if modelDir == "" {
		return fmt.Errorf("model.onnx not found; place model files in <kbRoot>/models/ or next to the binary")
	}

	embedder := embed.NewONNXEmbedder(modelDir, 384, 10*time.Minute)

	db, err := kb.OpenDB(kbRoot)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	modelName := embed.ModelName(modelDir)

	fmt.Printf("embedding with model=%s full=%v\n", modelName, *full)
	written, skipped, err := kb.EmbedDocuments(db, kbRoot, embedder, modelName, *full)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	fmt.Printf("embedded %d document(s), skipped %d unchanged\n", written, skipped)
	return nil
}
