// Command genassets renders the documentation images that come from the app's
// own drawing code (the tray dots), so the README shows exactly what users see.
//
//	go run ./tools/genassets
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/LuisPuertollano/AgentSemaphore/internal/icon"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

func main() {
	dir := filepath.Join("docs", "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, s := range []state.State{state.Red, state.Orange, state.Green, state.Grey} {
		p := filepath.Join(dir, "dot-"+s.String()+".png")
		if err := os.WriteFile(p, icon.PNGSize(s, 64), 0o644); err != nil {
			log.Fatal(err)
		}
		log.Println("wrote", p)
	}
}
