package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Yecangyuan/LogcatTool/internal/adb"
	"github.com/Yecangyuan/LogcatTool/internal/model"

	tea "charm.land/bubbletea/v2"
)

var (
	version = "0.1.0"
)

func main() {
	filePath := flag.String("f", "", "从日志文件读取 (离线模式)")
	serial := flag.String("s", "", "指定设备序列号")
	bufSize := flag.Int("n", 50000, "日志缓冲区大小")
	showVersion := flag.Bool("v", false, "显示版本信息")
	debug := flag.Bool("debug", false, "开启调试日志")
	flag.Parse()

	if *showVersion {
		fmt.Printf("LogCaTool v%s\n", version)
		os.Exit(0)
	}

	if *debug {
		f, err := tea.LogToFile("logcatool-debug.log", "debug")
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法打开调试日志: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	opts := model.Options{
		FilePath:   *filePath,
		Serial:     *serial,
		BufferSize: *bufSize,
	}

	if opts.FilePath == "" {
		adbPath, err := adb.FindADB()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			fmt.Fprintln(os.Stderr, "请确保 adb 已安装并在 PATH 中，或使用 -f 选项读取日志文件")
			os.Exit(1)
		}
		opts.ADBPath = adbPath
	}

	p := tea.NewProgram(model.New(opts))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
