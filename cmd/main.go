package main

import (
	"circuit"
	"circuit/mna/debug"
	"fmt"
	"net/http"
	"os"
	"time"
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
	graph := mna.GetGraph()
	graph.Debug = charts
	graph.IsTrapezoidal = true
	go func() {
		// 测试仿真
		start := time.Now() // 获取当前时间
		if err := circuit.Simulate(2, mna); err != nil {
			fmt.Println(err)
			os.Exit(0)
		}
		fmt.Println("解析运行时间:", time.Since(start))
	}()
	http.HandleFunc("/", charts.Handler)
	http.ListenAndServe(":8081", nil)
}
