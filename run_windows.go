//go:build windows

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsOptions "github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"showmethestory/internal/desktop"
	"showmethestory/internal/devlog"
	"showmethestory/internal/httpapi"
)

type desktopRuntimeInfo struct {
	Mode     string `json:"mode"`
	APIBase  string `json:"api_base"`
	APIToken string `json:"api_token"`
}

type exportSpec struct {
	path       string
	extension  string
	filterName string
}

type DesktopBridge struct {
	mu           sync.RWMutex
	ctx          context.Context
	info         desktopRuntimeInfo
	client       *http.Client
	application  *httpapi.Application
	server       *http.Server
	closing      atomic.Bool
	closeFlow    atomic.Bool
	pendingFocus atomic.Bool
}

func runApplication(opts launchOptions, resources *appResources) error {
	if opts.Web {
		return runWebApplication(resources)
	}
	return runDesktopApplication(resources)
}

func runDesktopApplication(resources *appResources) error {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("生成桌面会话令牌失败: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("启动桌面 API 失败: %w", err)
	}

	application := httpapi.NewApplication(resources.apiCfg, resources.apiCfgPath, resources.logger, resources.progDir, version, nil, token)
	server := &http.Server{
		Handler:           application,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
		}
		close(serveErrors)
	}()

	bridge := &DesktopBridge{
		info: desktopRuntimeInfo{
			Mode:     "desktop",
			APIBase:  "http://" + listener.Addr().String(),
			APIToken: token,
		},
		client:      &http.Client{Timeout: 10 * time.Minute},
		application: application,
		server:      server,
	}

	runErr := wails.Run(&options.App{
		Title:            "Show Me the Story",
		Width:            1440,
		Height:           900,
		MinWidth:         1024,
		MinHeight:        700,
		BackgroundColour: options.NewRGB(18, 18, 20),
		AssetServer:      &assetserver.Options{Assets: resources.staticFS},
		Bind:             []interface{}{bridge},
		OnStartup:        bridge.startup,
		OnBeforeClose:    bridge.beforeClose,
		OnShutdown:       bridge.shutdown,
		WindowStartState: options.Normal,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID(resources.progDir),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				bridge.focusWindow()
			},
		},
		Windows: &windowsOptions.Options{Theme: windowsOptions.SystemDefault},
	})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	if runErr != nil {
		return runErr
	}
	if serveErr := <-serveErrors; serveErr != nil {
		return fmt.Errorf("桌面 API 异常停止: %w", serveErr)
	}
	return nil
}

func singleInstanceID(dataDir string) string {
	normalized := strings.ToLower(filepath.Clean(dataDir))
	sum := sha256.Sum256([]byte(normalized))
	return "show-me-the-story-" + hex.EncodeToString(sum[:16])
}

func (b *DesktopBridge) startup(ctx context.Context) {
	b.mu.Lock()
	b.ctx = ctx
	b.mu.Unlock()
	if b.pendingFocus.Swap(false) {
		b.focusWindow()
	}
}

func (b *DesktopBridge) RuntimeInfo() desktopRuntimeInfo { return b.info }

func (b *DesktopBridge) SaveExport(kind, projectName, suggestedName string) (desktop.SaveResult, error) {
	spec, err := exportFor(kind, projectName)
	if err != nil {
		return desktop.SaveResult{}, err
	}
	ctx, err := b.runtimeContext()
	if err != nil {
		return desktop.SaveResult{}, err
	}
	name := desktop.SafeSuggestedName(suggestedName, "export"+spec.extension)
	if !strings.EqualFold(filepath.Ext(name), spec.extension) {
		name += spec.extension
	}
	target, err := wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:           "保存导出文件",
		DefaultFilename: name,
		Filters: []wailsruntime.FileFilter{{
			DisplayName: spec.filterName,
			Pattern:     "*" + spec.extension,
		}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return desktop.SaveResult{}, err
	}
	if target == "" {
		return desktop.SaveResult{Cancelled: true}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.info.APIBase+spec.path, nil)
	if err != nil {
		return desktop.SaveResult{}, err
	}
	req.Header.Set("X-Desktop-Token", b.info.APIToken)
	resp, err := b.client.Do(req)
	if err != nil {
		return desktop.SaveResult{}, fmt.Errorf("读取导出内容失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return desktop.SaveResult{}, fmt.Errorf("导出请求失败: HTTP %d", resp.StatusCode)
	}
	if err := desktop.WriteReaderAtomic(target, resp.Body); err != nil {
		return desktop.SaveResult{}, err
	}
	return desktop.SaveResult{Path: target}, nil
}

func (b *DesktopBridge) SaveText(suggestedName, content string) (desktop.SaveResult, error) {
	ctx, err := b.runtimeContext()
	if err != nil {
		return desktop.SaveResult{}, err
	}
	name := desktop.SafeSuggestedName(suggestedName, "export.md")
	target, err := wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:                "保存文本",
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Markdown (*.md)", Pattern: "*.md"},
			{DisplayName: "Text (*.txt)", Pattern: "*.txt"},
		},
	})
	if err != nil {
		return desktop.SaveResult{}, err
	}
	if target == "" {
		return desktop.SaveResult{Cancelled: true}, nil
	}
	if err := desktop.WriteReaderAtomic(target, bytes.NewBufferString(content)); err != nil {
		return desktop.SaveResult{}, err
	}
	return desktop.SaveResult{Path: target}, nil
}

func (b *DesktopBridge) OpenExternalURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return errors.New("只允许打开 HTTP 或 HTTPS 链接")
	}
	ctx, err := b.runtimeContext()
	if err != nil {
		return err
	}
	wailsruntime.BrowserOpenURL(ctx, raw)
	return nil
}

func exportFor(kind, projectName string) (exportSpec, error) {
	switch kind {
	case "book":
		return exportSpec{path: "/api/export/txt", extension: ".txt", filterName: "Text (*.txt)"}, nil
	case "outline":
		return exportSpec{path: "/api/export/outline", extension: ".md", filterName: "Markdown (*.md)"}, nil
	case "proofread-report":
		return exportSpec{path: "/api/proofread/export", extension: ".md", filterName: "Markdown (*.md)"}, nil
	case "project-backup":
		if strings.TrimSpace(projectName) == "" {
			return exportSpec{}, errors.New("项目备份需要项目名称")
		}
		return exportSpec{path: "/api/projects/" + url.PathEscape(projectName) + "/backup", extension: ".zip", filterName: "ZIP (*.zip)"}, nil
	default:
		return exportSpec{}, fmt.Errorf("不支持的导出类型: %s", kind)
	}
}

func (b *DesktopBridge) runtimeContext() (context.Context, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.ctx == nil {
		return nil, errors.New("桌面窗口尚未就绪")
	}
	return b.ctx, nil
}

func (b *DesktopBridge) focusWindow() {
	ctx, err := b.runtimeContext()
	if err != nil {
		b.pendingFocus.Store(true)
		return
	}
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
}

func (b *DesktopBridge) beforeClose(ctx context.Context) bool {
	if b.closing.Load() || !b.application.IsTaskRunning() {
		return false
	}
	choice, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "任务仍在运行",
		Message:       "当前 AI 任务仍在运行。是否取消任务并关闭程序？",
		Buttons:       []string{"继续使用", "取消任务并关闭"},
		DefaultButton: "继续使用",
		CancelButton:  "继续使用",
	})
	if err != nil || choice != "取消任务并关闭" {
		return true
	}
	if b.closeFlow.CompareAndSwap(false, true) {
		go b.cancelAndClose(ctx)
	}
	return true
}

func (b *DesktopBridge) cancelAndClose(ctx context.Context) {
	b.application.CancelTask()
	for {
		waitCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := b.application.WaitIdle(waitCtx)
		cancel()
		if err == nil {
			b.closing.Store(true)
			wailsruntime.Quit(ctx)
			return
		}
		choice, dialogErr := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
			Type:          wailsruntime.WarningDialog,
			Title:         "任务尚未结束",
			Message:       "任务在 15 秒内未能结束。继续等待更安全；强制退出可能留下未完成的模型请求。",
			Buttons:       []string{"继续等待", "强制退出（高风险）"},
			DefaultButton: "继续等待",
			CancelButton:  "继续等待",
		})
		if dialogErr == nil && choice == "强制退出（高风险）" {
			b.closing.Store(true)
			wailsruntime.Quit(ctx)
			return
		}
	}
}

func (b *DesktopBridge) shutdown(context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.server.Shutdown(ctx); err != nil {
		devlog.Log("desktop HTTP shutdown failed: %v", err)
	}
}

func reportStartupError(err error) {
	fmt.Fprintf(os.Stderr, " [错误] %v\n", err)
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	message, _ := syscall.UTF16PtrFromString(err.Error())
	title, _ := syscall.UTF16PtrFromString("Show Me the Story")
	messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
