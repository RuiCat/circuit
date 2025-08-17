package main

import (
	"circuit"
	"fmt"
	"net/http"
)

func main() {
	wl := circuit.NewCircuit()
	fmt.Println(wl.Load("./test.asc"))
	wl.ElementList[0].SetKeyValue("Voltage", float64(5))
	wl.ElementList[1].SetKeyValue("Resistance", float64(10))
	wl.ElementList[2].SetKeyValue("Capacitance", float64(0.01))
	wl.ElementList[3].SetKeyValue("Resistance", float64(10))
	wl.ElementList[4].SetKeyValue("Capacitance", float64(0.01))
	mna, err := wl.MNA()
	if err != nil {
		fmt.Println(err)
		return
	}
	// 开启调试
	mna.Debug.IsDebug = true
	mna.IsTrapezoidal = true
	go func() {
		// 测试仿真
		if err := circuit.Simulate(2, mna); err != nil {
			fmt.Println(err)
		}
	}()
	http.HandleFunc("/", mna.Debug.Handler)
	http.ListenAndServe(":8081", nil)
}
