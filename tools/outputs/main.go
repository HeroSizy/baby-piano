// Command outputs lists the computer's sound outputs as the setup page
// shows them, the default one starred; with an id, it makes that output the
// default. It checks kiosk.Outputs and kiosk.SetOutput on a real Mac or PC.
package main

import (
	"fmt"
	"os"

	"github.com/HeroSizy/baby-piano/kiosk"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println("set:", kiosk.SetOutput(os.Args[1]))
	}
	for _, o := range kiosk.Outputs() {
		mark := " "
		if o.Default {
			mark = "*"
		}
		fmt.Printf("%s %s\t%s\n", mark, o.ID, o.Name)
	}
}
