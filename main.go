package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Yecangyuan/LogcatTool/internal/adb"
	"github.com/Yecangyuan/LogcatTool/internal/logdiff"
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
	replay := flag.Bool("replay", false, "按日志时间戳回放 -f 指定的离线日志")
	replaySpeed := flag.Float64("replay-speed", 1, "离线回放速度倍率")
	diffBase := flag.String("diff-base", "", "作为基线的日志文件")
	diffCandidate := flag.String("diff-candidate", "", "作为候选的日志文件")
	showVersion := flag.Bool("v", false, "显示版本信息")
	debug := flag.Bool("debug", false, "开启调试日志")
	flag.Parse()

	if *showVersion {
		fmt.Printf("LogcatTool v%s\n", version)
		os.Exit(0)
	}

	if (*diffBase == "") != (*diffCandidate == "") {
		fmt.Fprintln(os.Stderr, "错误: --diff-base 和 --diff-candidate 必须同时提供")
		os.Exit(1)
	}
	if *diffBase != "" {
		base, err := logdiff.ReadCapture(*diffBase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		candidate, err := logdiff.ReadCapture(*diffCandidate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(logdiff.FormatReport(logdiff.CompareCaptures(base, candidate)))
		return
	}
	if *replay && *filePath == "" {
		fmt.Fprintln(os.Stderr, "错误: --replay 需要配合 -f <日志文件> 使用")
		os.Exit(1)
	}
	if *replaySpeed <= 0 {
		fmt.Fprintln(os.Stderr, "错误: --replay-speed 必须大于 0")
		os.Exit(1)
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
		FilePath:    *filePath,
		Serial:      *serial,
		BufferSize:  *bufSize,
		ReplayMode:  *replay,
		ReplaySpeed: *replaySpeed,
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
