// Command skin-nexus is the SkinNexus server (phase-1 skeleton).
package main

import (
	"fmt"
	"os"

	"skin-nexus/server/internal/auth"
	"skin-nexus/server/internal/ygg"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("skin-nexus", version)
		return
	}
	// Phase-1 skeleton: wiring lands in M1. Interfaces below are frozen:
	//   auth.Registry  — identity sources (password phase 1; microsoft phase 2)
	//   ygg.Signer     — texture signing (LocalSigner now; UnionSigner phase 2)
	//   plugin.Host    — hook contract (runtime frozen: goja)
	_ = auth.NewRegistry
	_ = ygg.NewLocalSigner
	fmt.Println("skin-nexus skeleton — see docs/phase1-dev-plan.md")
}
