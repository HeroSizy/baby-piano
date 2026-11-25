//go:build !ios

package main

import "github.com/nsgo-dev/nsgo"

// routePickerComponent is iOS's AirPlay button (output_ios.go).
const routePickerComponent = "routePicker"

func platformOptions() []nsgo.Option { return nil }

// outputName is unknown here.
func outputName() string { return "" }
