# 桌面 GUI、共享卡片库与 Windows 打包

本文记录当前改造的长期设计与开发入口，供后续维护使用；实际使用步骤见 [中文使用指南](guide.zh.md)。项目架构约束以 [AGENTS.md](../AGENTS.md) 为准。

## 1. 改造范围

- Windows amd64 默认打开 Wails v2.15.0 桌面窗口，保留 `--web` 浏览器回退。
- Linux/macOS 继续提供 Web 版，不引入桌面运行时。
- 沿用 Svelte 页面、REST API、SSE 与既有写作流程，不重新实现原生表单。
- 原先的整套配置模板需求已调整为单张共享卡片库：人物、世界观等可以独立保存、独立使用，缺少的依赖一起导入。
- 项目格式保持 v4，不迁移数据到 AppData，不改变项目备份格式。

## 2. 桌面架构

```text
Wails 原生窗口
├── 嵌入的 Svelte 前端
│   ├── 只读 RuntimeInfo → API 地址与临时令牌
│   └── REST / SSE → 真实 loopback HTTP 服务
└── 原生桥接 → 保存对话框、外部链接、窗口生命周期

HTTP 应用 → 原有任务、项目、写作与持久化逻辑
```

`main.go` 负责启动参数与公共初始化；`run_windows.go` 承载 Windows 桌面入口，`run_web.go` 保留 Web 入口，非 Windows 入口由 build tag 隔离。HTTP 路由与生命周期复用 `internal/httpapi`，安全保存基础能力位于 `internal/desktop`。

桌面服务只监听 `127.0.0.1:0`。每次启动生成临时 256-bit 令牌，普通 API 请求使用 `X-Desktop-Token`，SSE 使用 `desktop_token` 查询参数。地址与令牌只经 `RuntimeInfo` 返回，不写文件或日志。Wails AssetServer 只负责静态资源，REST/SSE 不经过它，避免自定义协议与流式请求兼容问题。

前端挂载前初始化 `frontend/src/lib/desktop.js`。API 与 SSE 统一使用其运行时配置；`lib/downloads.js` 区分原生保存与 Web Blob 下载。原生导出白名单为全文、大纲、校订报告、项目备份；前端生成的伏笔路线图通过文本保存接口处理。取消保存不提示成功，写入采用同目录临时文件与受限回滚替换，失败不得破坏已有文件。

窗口默认 1440×900，最小 1024×700，使用系统标题栏。外部链接交给系统浏览器。同一规范化数据目录对应同一实例锁，再次启动恢复并聚焦现有窗口。任务空闲时正常关闭；任务运行时先确认取消，等待最多 15 秒后仍未结束则提供继续等待或高风险强制退出，默认继续等待。

## 3. 数据目录与启动

未指定数据目录时使用启动工作目录；显式目录必须已经存在。数据根目录下保存 `api.json`、`storys/`、用户 `skills/`、`shared-cards/` 等本机数据。不要把 `storys/` 本身误当作数据根目录，否则会形成额外的 `storys/` 层级。

```powershell
.\show-me-the-story.exe
.\show-me-the-story.exe --data-dir "D:\developer\show-me-the-story"
.\show-me-the-story.exe --web --data-dir "D:\developer\show-me-the-story"
```

仍兼容首个位置参数作为数据目录。`PORT` 仅影响 Web 模式，默认 48090；GUI 始终使用随机 loopback 端口。不要让 GUI 与 Web 进程同时写同一数据目录。

## 4. 共享卡片库

共享卡片位于数据根目录 `shared-cards/<ID>.json`，使用独立 schema v1，不随项目删除，也不进入项目 ZIP。每个文件保存一个根卡，支持 API、故事参数、风格、人物、世界观、组织和关系，不保存章节、事实、伏笔或项目内部同步状态。

组织卡内嵌成员人物；关系卡内嵌两端实体及组织所需成员，保持依赖闭合。导入人物、世界观或组织根卡时分配新 ID；依赖按规范化名称复用并补齐缺项，组织成员和关系端点统一重映射。卡片不是整个项目配置快照，也不是按名称自动覆盖全部设定。

主界面通过右侧栏操作卡片库。配置标题和单条设定常显「加入卡片库」，库内每张卡常显「使用」；拖拽只是快捷方式，核心操作不能依赖拖拽。

编辑器按根卡类型展示完整字段，语义与配置页一致：

- 人物：名称、年龄及外貌、性格、背景、目标、能力、备注。
- 世界观：名称、分类、描述、标签。
- 组织：名称、类型、描述与成员。
- 关系：两端实体与关系标签。
- API、故事、风格：对应配置字段；API 密钥详情不得返回原文，编辑留空保留旧值。

编辑只改变卡片库副本，不反向修改来源项目。成员和关系端点只能选卡片已保存的依赖快照。API 卡片中的密钥是本地明文，界面必须明确提醒；卡片文件不能当作可公开分享的无敏感数据文件。

导入先预览并检查状态指纹，状态变化或 AI 任务运行时拒绝提交并要求重新处理。已有正文时，故事/风格覆盖需要额外确认；类型、风格或视角变化沿用现有设定协调流程。

### 表单布局约定

`SharedCardLibrary.svelte` 负责侧栏列表，`SharedCardEditor.svelte` 负责完整编辑表单，避免把功能继续堆入 `Config.svelte`。

编辑表单使用块级小号标签与 `w-full` 输入框、选择框和文本框；字段以响应式双列网格组织，窄屏单列，内部面板使用 `bg-base-300`。弹窗头部、可滚动正文与底部操作区独立，不使用依赖旧 `form-control` 行为的布局。列表条目沿用设定卡的图标、摘要及右侧操作区。回归脚本 `frontend/src/lib/sharedCardUI.check.js` 检查关键结构，但不能替代实际视觉验收。

## 5. 开发启动与发布打包

`dev.bat` 切换到脚本所在目录，启动 Wails 热重载，并使用本仓库根目录下的数据。它优先查找 `go env GOPATH` 的 `bin/wails.exe`，其次查找 PATH，不再通过 `go run ...@version` 每次联网解析 CLI。首次缺少工具时手动安装：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
.\dev.bat
```

显式入口还有 `task dev:gui`；`task dev` 保留 Web 开发模式。若没有窗口，先检查本地 Wails CLI、终端错误、WebView2 和 `dev.log`，不要把等待 CLI 下载误认为窗口已经启动。

根目录 `package.ps1` 打包的是可分发的 Windows amd64 程序，**不是源码归档或个人作品备份**：

```powershell
.\package.ps1 -Version dev
.\package.ps1 -Version 1.0.0 -OutputDirectory release
# 仅在已单独完成检查时使用：
.\package.ps1 -Version dev -SkipChecks
```

默认流程是 `npm ci`、前端构建与回归检查、Go test/vet、Wails 生产构建，再生成 `release/show-me-the-story-<版本>-windows-amd64.zip`。`-SkipChecks` 只跳过检查，不跳过安装和构建。同名 ZIP 会被覆盖，应使用不同版本名保留旧包。

生产构建使用 `-webview2 embed`，内嵌的是缺失 WebView2 Runtime 时使用的官方 bootstrapper，而不是整个运行时。ZIP 包含可执行文件、双语 README、LICENSE 和双语使用指南；不包含本机 `storys/`、`api.json`、共享卡片、用户 Skill 或日志。源码中的内置 Skill 随二进制嵌入。脚本只清理自己创建的临时打包目录。

项目备份请使用界面的项目 ZIP 功能；搬迁全部本机数据应在任务结束并关闭程序后复制完整数据目录。项目 ZIP 与发布 ZIP 都不能代替全局数据迁移。

## 6. 检查与验收边界

维护时执行前端构建和 `npm run check`，以及 `go build ./...`、`go test ./...`、`go vet ./...`、Windows Wails 生产构建。CI 在 Linux 额外运行 race 检查，在 Windows 原生编译 GUI；Linux/macOS 发布 Web 产物。

本轮改造已完成构建与自动检查，并验证过本地 Wails 开发启动及发布 ZIP 生成。卡片表单真实视觉、SSE 长任务、任务中关闭窗口、原生保存成功/取消/失败仍需完整交互验收；不能把结构回归检查或构建成功等同于这些场景已通过。建议使用临时数据目录验收，不直接拿作品原件测试失败路径。

本文件只记录桌面、卡片与打包边界；内置写作 Skill 的内容与启用规则另见使用指南与 AGENTS.md，不在此重复。
