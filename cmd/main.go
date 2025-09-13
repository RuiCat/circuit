package main

import (
	"circuit"
	"circuit/mna/debug"
	"fmt"
	"net/http"
	"os"
)

func main() {
	// 从命令行参数获取文件名
	filename := "./test.asc"
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}
	wl := circuit.NewCircuit()
	if err := wl.Load(filename); err != nil {
		fmt.Println(err)
		return
	}
	mna, err := wl.MNA()
	if err != nil {
		fmt.Println(err)
		return
	}
	// 开启调试
	charts := &debug.Charts{}
	mna.Debug = charts
	mna.IsTrapezoidal = true
	go func() {
		// 测试仿真
		if err := circuit.Simulate(10, mna); err != nil {
			fmt.Println(err)
			os.Exit(0)
		}
	}()
	http.HandleFunc("/", charts.Handler)
	http.ListenAndServe(":8081", nil)
}
