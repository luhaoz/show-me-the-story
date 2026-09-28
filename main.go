package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"showmethestory/internal/config"
	"showmethestory/internal/devlog"
	"showmethestory/internal/llm"
	"showmethestory/internal/sse"
)

//go:embed all:frontend/dist
var staticFiles embed.FS

// version is injected by CI via -ldflags "-X main.version=...".
var version = "dev"

const defaultPort = ":48090"

type launchOptions struct {
	Web     bool
	DataDir string
}

type appResources struct {
	progDir    string
	storysDir  string
	apiCfgPath string
	apiCfg     *config.APIConfig
	logger     *sse.LogBroadcaster
	staticFS   fs.FS
}

func main() {
	opts, err := parseLaunchOptions(os.Args[1:])
	if err != nil {
		reportStartupError(err)
		os.Exit(2)
	}

	resources, err := initialiseApplication(opts.DataDir)
	if err != nil {
		devlog.Log("application initialisation failed: %v", err)
		reportStartupError(err)
		os.Exit(1)
	}
	defer resources.logger.Close()

	if err := runApplication(opts, resources); err != nil {
		devlog.Log("application stopped with error: %v", err)
		reportStartupError(err)
		os.Exit(1)
	}
}

func parseLaunchOptions(args []string) (launchOptions, error) {
	var opts launchOptions
	var positional string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--web":
			opts.Web = true
		case arg == "--data-dir":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--data-dir requires a directory")
			}
			i++
			opts.DataDir = args[i]
		case strings.HasPrefix(arg, "--data-dir="):
			opts.DataDir = strings.TrimPrefix(arg, "--data-dir=")
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown option %s", arg)
		default:
			if positional != "" {
				return opts, fmt.Errorf("only one data directory may be specified")
			}
			positional = arg
		}
	}
	if opts.DataDir != "" && positional != "" {
		return opts, fmt.Errorf("use either --data-dir or the positional data directory, not both")
	}
	if opts.DataDir == "" {
		opts.DataDir = positional
	}
	return opts, nil
}

func initialiseApplication(requestedDir string) (*appResources, error) {
	progDir := requestedDir
	if progDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("无法获取当前目录: %w", err)
		}
		progDir = cwd
	}
	absDir, err := filepath.Abs(progDir)
	if err != nil {
		return nil, fmt.Errorf("无法解析数据目录: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("数据目录不可用 %q: %w", absDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("数据目录不是文件夹: %s", absDir)
	}
	progDir = absDir
	devlog.Init(progDir, version)

	storysDir := filepath.Join(progDir, "storys")
	if err := os.MkdirAll(storysDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建项目目录失败: %w", err)
	}

	apiCfgPath := filepath.Join(progDir, "api.json")
	apiCfg, err := config.LoadAPIConfig(apiCfgPath)
	if err != nil {
		return nil, fmt.Errorf("加载API配置失败: %w", err)
	}
	llm.EnsureContextBudget(apiCfg)

	logger := sse.NewLogBroadcaster()
	frontendFS, err := fs.Sub(staticFiles, "frontend/dist")
	if err != nil {
		logger.Close()
		return nil, fmt.Errorf("嵌入静态文件失败: %w", err)
	}

	fmt.Printf(" [系统] 版本: %s\n", version)
	fmt.Printf(" [系统] 程序目录: %s\n", progDir)
	fmt.Printf(" [系统] 项目目录: %s\n", storysDir)
	if devlog.Enabled() {
		fmt.Printf(" [系统] 开发日志: %s\n", filepath.Join(progDir, "dev.log"))
	}
	if apiCfg.BaseURL == "" || apiCfg.Model == "" {
		fmt.Println(" [系统] 检测到空白API配置，已自动生成 api.json")
		fmt.Println(" [系统] 请通过界面配置 API 地址和模型后再使用")
	}

	return &appResources{
		progDir:    progDir,
		storysDir:  storysDir,
		apiCfgPath: apiCfgPath,
		apiCfg:     apiCfg,
		logger:     logger,
		staticFS:   frontendFS,
	}, nil
}
