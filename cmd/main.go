package main

import (
	"circuit"
	"circuit/mna"
	"fmt"
)

func main() {

	wl := circuit.NewCircuit()
	fmt.Println(wl.Load("./test.asc"))

	wl.ElementList[0].SetKeyValue("Voltage", float64(5))
	wl.ElementList[1].SetKeyValue("Resistance", float64(10))
	wl.ElementList[2].SetKeyValue("Capacitance", float64(0.01))

	wl.Simulate(2, func(mna *mna.MNA) {
		mna.Debug = true
		// mna.IsTrapezoidal = true
	})

}
