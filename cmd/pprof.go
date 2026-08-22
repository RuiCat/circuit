package main

import (
	"fmt"
	"net"
	"net/http"
	httppprof "net/http/pprof"
	"os"
	runtimepprof "runtime/pprof"
	"time"
)

// startPprof 启动 pprof 性能剖析支持：
//   - addr      非空时启动 pprof HTTP 调试服务（net/http/pprof），
//     运行中可用 `go tool pprof http://<addr>/debug/pprof/profile` 抓取 CPU/堆/协程等画像；
//   - cpuFile   非空时用 runtime/pprof 记录整个进程生命周期内的 CPU profile；
//   - memFile   非空时在进程退出前写入堆内存 profile。
//
// 返回的 cleanup 依次停止 CPU 采样、落盘堆内存画像并关闭 HTTP 服务；
// 必须在函数返回（含错误退出路径）前调用，否则 profile 文件不完整。
func startPprof(addr, cpuFile, memFile string) (cleanup func(), err error) {
	cleanup = func() {}

	var srv *http.Server
	if addr != "" {
		ln, lerr := net.Listen("tcp", addr)
		if lerr != nil {
			return nil, fmt.Errorf("pprof 监听 %s 失败: %w", addr, lerr)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", httppprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", httppprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", httppprof.Trace)
		srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			_ = srv.Serve(ln)
		}()
		fmt.Fprintf(os.Stderr, "pprof HTTP 服务已启动: http://%s/debug/pprof/\n", ln.Addr())
	}

	var cpuFileHandle *os.File
	if cpuFile != "" {
		f, ferr := os.Create(cpuFile)
		if ferr != nil {
			if srv != nil {
				_ = srv.Close()
			}
			return nil, fmt.Errorf("创建 CPU profile 文件 %s 失败: %w", cpuFile, ferr)
		}
		if perr := runtimepprof.StartCPUProfile(f); perr != nil {
			_ = f.Close()
			if srv != nil {
				_ = srv.Close()
			}
			return nil, fmt.Errorf("启动 CPU profile 失败: %w", perr)
		}
		cpuFileHandle = f
		fmt.Fprintf(os.Stderr, "CPU profile 采样中，退出时写入: %s\n", cpuFile)
	}

	return func() {
		// 先停止 CPU 采样保证文件完整，再落盘堆画像，最后关闭 HTTP 服务
		if cpuFileHandle != nil {
			runtimepprof.StopCPUProfile()
			if cerr := cpuFileHandle.Close(); cerr != nil {
				fmt.Fprintf(os.Stderr, "警告: 关闭 CPU profile 文件失败: %v\n", cerr)
			}
		}
		if memFile != "" {
			if merr := writeMemProfile(memFile); merr != nil {
				fmt.Fprintf(os.Stderr, "警告: %v\n", merr)
			}
		}
		if srv != nil {
			if serr := srv.Close(); serr != nil {
				fmt.Fprintf(os.Stderr, "警告: 关闭 pprof HTTP 服务失败: %v\n", serr)
			}
		}
	}, nil
}

// writeMemProfile 把堆内存 profile 写入指定文件（go tool pprof 可直接分析）。
func writeMemProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建内存 profile 文件 %s 失败: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "警告: 关闭内存 profile 文件失败: %v\n", cerr)
		}
	}()
	if err := runtimepprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("写入内存 profile 失败: %w", err)
	}
	fmt.Fprintf(os.Stderr, "内存 profile 已写入: %s\n", path)
	return nil
}
