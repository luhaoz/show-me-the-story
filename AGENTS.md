# AGENTS.md — AI 小说写手项目指南

> 修改代码、配置、前端、提示词或构建流程后，必须同步更新本文件；只记录长期有效的约束与当前架构，不记录单次修复历史或逐函数清单。

## 项目概览

- 单二进制 Go 应用；Windows amd64 默认使用 Wails v2.15.0 原生窗口，Linux/macOS 保持 Web 模式。前端产物与内置 Skill 通过 `embed.FS` 嵌入。
- Go `1.25.1`，模块 `showmethestory`；Web 模式默认端口 `:48090`，可用 `PORT` 覆盖。Windows GUI 使用随机 loopback 端口，`PORT` 不生效。
- 前端：Vite 5、Svelte 4、Tailwind CSS 4、DaisyUI 5、`@xianii/design-system`；Playwright 仅使用系统 Google Chrome 生成 README 截图。
- 当前项目格式固定为 v4；项目默认保存在程序目录的 `storys/<项目名>/`。
- 项目语言 `zh` / `en` 决定模型提示词、正文和内置 Skill；UI 语言由浏览器独立切换。
- 默认英文入口为 `README.md`，中文入口为 `README.zh.md`；语言导航保持独立的首行链接列表，便于继续增加语言。详细流程维护于 `docs/guide.zh.md` 与 `docs/guide.en.md`，按创作顺序提供双语步骤与故障排查；许可证为 MIT。
- 桌面 GUI、单卡共享库、表单布局和 Windows 发布打包的维护说明集中于 `docs/gui-and-shared-cards.zh.md`；程序发布 ZIP 与个人作品备份必须保持区分，架构边界变化时同步该文档。

## 常用命令

```bash
task build                 # npm install + 前端构建 + Go 二进制
task build:go              # 只构建 Go；要求 frontend/dist 已存在
task dev                   # 构建并启动 Web 模式
task dev:frontend          # Vite 开发服务器 :5173，代理 /api 到 :48090
task dev:gui               # Wails 热重载窗口
task build:gui:windows     # Windows amd64 GUI，WebView2 embed 策略
task screenshots           # 用固定离线样例重建 README 截图
dev.bat                    # Windows 双击启动 Wails 热重载 GUI
.\package.ps1 -Version dev # 检查、构建 Windows GUI 并生成便携 ZIP

go build ./...
go test ./...
go vet ./...
cd frontend && npm run build

node frontend/src/lib/projectRestore.check.js
node frontend/src/lib/forceGraphLayout.check.js
cd frontend && npm run check # 所有前端回归检查
```

提交前至少运行 `go build ./...`、`go test ./...`、`go vet ./...`；修改前端时再运行前端构建及受影响的 `.check.js`。

PR 与 main/v4 推送通过同一检查工作流在 Linux/Windows 原生执行构建、测试及 vet；Windows 固定安装 Wails v2.15.0 并编译 GUI，Linux 额外运行 race 检查。发布复用该工作流，Windows 产出 GUI 便携 ZIP，Linux/macOS 产出 Web 版。

## 架构边界

```text
main.go + run_web.go + run_windows.go
└── internal/httpapi
    ├── internal/agent
    │   └── internal/story
    └── internal/story
        ├── internal/llm
        ├── internal/config
        ├── internal/sse
        ├── internal/i18n
        ├── internal/prose
        └── internal/fsutil
```

- `main.go`：命令行、程序目录、API 配置、开发日志和嵌入前端；`run_web.go` 保留浏览器服务，`run_windows.go` 负责 Wails 窗口、loopback API、原生保存和关闭生命周期。
- `internal/desktop/`：不依赖 Wails 的桌面保存基础能力，使用同目录临时文件与受限回滚替换。
- `dev.bat`：Windows 根目录 Wails GUI 开发入口，先切换到脚本目录并使用该目录下的 `storys/`；直接查找 `go env GOPATH` 下已安装的 `wails.exe`，其次使用 PATH，不得用 `go run ...@version` 导致每次启动重新联网解析 CLI。
- `package.ps1`：Windows 一键发布入口，默认执行前端构建/检查、Go test/vet 和 Wails `embed` 构建，再生成 `release/` 便携 ZIP；必须排除 `storys/`、`api.json`、`shared-cards/`、用户 Skill、日志和其他本机数据。
- `internal/httpapi/`：路由、请求校验、项目管理、异步任务互斥、SSE 和本地化错误。
- `internal/agent/`：助理循环、工具解析与执行、危险操作确认。
- `internal/story/`：大纲、写作、事实与设定、伏笔、导入、Skill、会话、校订等领域逻辑。
- `internal/llm/`：OpenAI 兼容客户端、流式完整性、重试、token 统计、JSON 提取。
- `internal/trilium/`：Trilium ETAPI 客户端、全局连接配置、大纲 HTML 转文本及安全的正文 HTML 输出；HTML 解析直接依赖 `golang.org/x/net/html`。
- `internal/config/`：API/项目配置和中英默认提示词。
- `internal/sse/`、`i18n/`、`prose/`、`fsutil/`、`devlog/`：领域无关基础能力。
- `frontend/src/`：Svelte 页面、组件、stores、API/SSE/i18n；生产产物仅在 `frontend/dist/`。

依赖必须保持单向：`httpapi → agent → story → 基础包`。`sse` 的事件负载保持 `any`，不得反向依赖领域包。不要恢复已废弃的根目录 `static/` 单页实现。

Windows GUI 只负责窗口、静态资源、原生对话框和生命周期；REST API 与 SSE 仍由真实 `net/http` loopback 服务提供，不迁移为 Wails binding。每次 GUI 启动生成临时 256-bit 令牌：普通 API 使用 `X-Desktop-Token`，SSE 使用 `desktop_token` 查询参数，令牌不得落盘或写日志。`RuntimeInfo` 是前端获取地址与令牌的唯一入口。GUI 导出只允许全文、大纲、校订报告、项目备份四类服务端资源；伏笔路线图通过 `SaveText` 保存。Windows 桌面代码必须使用 build tag 隔离，非 Windows 构建不得引入桌面运行时。

## 持久化与兼容边界

- v4 的 `progress.json` 只存元数据，正文存于 `chapters/NNNNNN.json`；API 通过 `ProgressView` 返回无正文视图和派生字段。
- 配置、进度、设定、会话及校订状态写入必须复用 `fsutil.WriteFileAtomic` 或现有领域保存函数；先提交元数据，再清理孤儿章节文件。
- 正文与进度保存由进程内存储锁串行执行；覆盖前以 `progress.json.rollback` 保存本次涉及文件的旧字节，删除回滚日志后才视为提交并清理孤儿文件。加载、保存或重置前先恢复未完成事务；恢复失败保留日志、阻止操作并返回结构化 `SaveError`。该日志不提供历史版本，也不支持多个程序同时写同一项目。
- 章节读取、JSON 或身份校验失败必须阻止项目加载并保留章号、路径和原因；仅完全未写的 pending 章节允许文件缺失，不得把已有正文的读取失败当作空章节。
- 当前程序只打开 `project_format_version: 4`。v2/v3/未知格式仅可只读探测并提示对应版本，不加载、不迁移、不写回。
- 运行时配置/项目 JSON、`storys/`、`frontend/dist/`、二进制和 `dev.log` 均是本地/生成内容，不提交；已跟踪的 `frontend/package-lock.json` 例外。
- 章节 `Content` 是正文事实源；`Blocks` 从正文派生并保持稳定 ID，块编辑后重建 `Content`。
- 影响正文的请求使用内容版本校验；关联事实受影响时必须显式确认，过期版本返回 409。
- 项目列表支持完整项目 ZIP 备份及恢复为新项目；不包含全局 API 配置或用户 Skill。备份/恢复仅在 AI 空闲时执行，请求写操作与备份快照串行；恢复先在隐藏临时目录校验路径、大小、JSON、v4 和正文完整性，再发布新目录，禁止覆盖已有项目。上限为 ZIP 256 MiB、解压总量 512 MiB、单文件 64 MiB、20000 个条目。
- 项目列表可从任意空闲 v4 项目创建正文新版本：复制当前配置、设定、批次章纲和伏笔计划，清空本版本正文、摘要、事实与旧稿同步状态；继承的前作章节和事实仍作历史上下文。新项目先在隐藏目录校验后发布，源项目不变。
- 全局共享卡片使用独立 schema v1，位于程序目录 `shared-cards/`，不改变项目格式且不进入项目 ZIP；每个文件只保存一个 API、故事、风格、规划、人物、世界观、组织或关系根卡。规划卡采集大纲表单当前本批梗概、长期走向及结尾选项，使用时只回填表单，不改项目，生成批次才持久化；不包含章数。旧规划卡没有梗概时保留当前表单梗概。组织卡必须内嵌成员人物，关系卡必须内嵌两端实体及组织成员。导入根人物/世界观/组织时分配新 ID；依赖按规范化名称复用并补齐缺项，所有成员与关系端点统一重映射。API Key 在 API 卡片中本地明文保存，但列表/详情不得返回原文。文件使用原子写入和尽可能严格的权限，未知 schema、损坏文件或非法引用不得进入列表或项目状态。
- Trilium ETAPI 地址和令牌是程序目录的全局 `trilium.json`，不得进入项目 ZIP、API 响应或日志；空令牌更新保留旧值。大纲页各批次均可按笔记 ID 读取梗概；仅在使用该笔记成功生成批次后，才在 v4 `progress.json` 关联或更新来源服务器、笔记 ID 和标题；复制正文新版本保留该关联。写作及完稿校订页可手动发布：仅在全部本版本章节 accepted 且正文非空时，将按序合并的当前正文和章标题 HTML 创建为来源笔记的直接子笔记；每次新建，继承前作章节不导出，禁止跨服务器发布。

## 核心流程

### 异步任务

- AI 端点统一走 `tryStartTask()`，后台执行并 `defer endTask()`；同一时间只允许一个 AI 任务。
- 章纲编辑及伏笔确认附带的一致性检查同样在修改前预留任务锁，沿用 `taskCtx`、Skill 激活与任务事件；失败或取消不得发布新报告，保存失败必须保留存储诊断，所有提前返回释放预留。
- `taskCtx` 承载取消与任务 token 统计；SSE 提供日志、流式片段、任务状态和领域刷新事件。
- `endTask` 在释放互斥前同步本任务改动且已跟踪的正文知识；校订任务显式跳过该流程。
- 存储失败必须保留 `fsutil.SaveError` 的结构化诊断，不得用普通成功 Toast 覆盖。

### 批次大纲与结尾

- 首批和后续批次统一使用 `POST /api/outline/generate-continuation` 与 `OutlineBatchRequest`。
- 每批需要 `chapter_count`（1–36）和 `outline_synopsis`；可选长期方向与结尾意图。
- 默认 `append` 接在最大章号后；`replace_last` 只替换末尾完整、全部 pending、无正文的批次，并要求确认。
- 大纲页「重新生成大纲」使用独立 `POST /api/outline/restart`：任务空闲时确认后将当前项目重置到首批规划，清除所有章节、正文、批次、事实、伏笔进度、校订及导入断点；保留当前故事配置、人物/世界观等设定、书名、长期方向和 Trilium 来源。先保存空进度元数据，再清理孤儿章节文件与导出副本；不得复用仅适合未写批次的 `replace_last`。
- 预定完结批次之后继续规划需要 `confirm_continue=true`；真正完结仍由用户手动操作。
- 模型输出必须通过章节数量、连续编号和章纲长度校验；失败不得部分提交内存或磁盘状态。
- 首批且配置/进度书名都为空时允许模型推断书名；后续批次不得覆盖已有书名。

### 写作、事实与设定

- 章节状态为 `pending → writing → review → accepted`；失败草稿不得作为正式知识来源。
- 生成、确认、人工编辑、AI 修订、润色和衔接优化均进入知识同步；只处理已跟踪章节，不扫描旧章节补齐。
- 事实使用稳定 ID 与 `MemoryReference{chapter, block_id, quote, content_rev, stale}`；模型只能复用检索上下文中的相同 ID/原文，新事实使用 `id=0`。
- 事实或设定同步的 JSON/身份/证据校验失败可纠正重试一次；最终失败保留原数据和待同步状态，不提交部分结果、不消耗 ID。
- 设定只从 accepted 正文自动同步。无冲突的明确演变可应用；与作者设定冲突或证据不足的变化进入待确认建议。同一实体的多条待确认建议逐条采纳时，后续建议只保留自身拟修改字段，并以最新设定重算前后值；外部改动导致旧快照过期时先保存重算后的建议供作者复核，再次确认才可写入。证据段落失效时不得采纳旧建议，应显示待同步章节并重试知识同步。
- 世界观 `knowledge` 分类表示作者维护的小说知识规则，可以完全虚构；正文同步对已有知识规则的修改必须进入待确认建议。写作页可保存知识并修订选中章节，也可选择已有世界观条目；保存与修订分步执行，修订失败保留设定。
- 章节修订可携带 `worldview_ids`：后端读取完整条目并显式注入作者修改意见，不依赖检索、不静默裁剪，仍受模型总输入预算限制；需要当前正文版本与关联事实影响确认。前端按所选设定修订使用指定章节流程，不自动改其他章纲或正文；自定义修订模板缺少 UserFeedback 占位符时仍补入修改意见。
- 正文、修订、核查、记忆与设定同步复用 `knowledge_retrieval.go` 的本地 BM25 检索；无命中不回退全量知识。
- 检索预算只限制注入的知识上下文，不删除存储中的事实、设定或来源历史。
- 长篇上下文保留最近 20 章原始摘要；超过 40 个已确认章节后，下一次写作、批次规划或规划复盘按每 20 章生成带来源哈希的持久化检查点，每 10 个同级节点继续向上压缩。旧章修改或删除后按需重建；模型失败使用带 degraded 标记的限长本地摘要，下次使用重试并刷新受影响父节点。哈希版本变化会重建旧检查点；取消不得提交新的检查点，来源哈希不占用摘要正文预算。
- 正文只注入有界的近期历史、分层远期摘要、BM25 相关旧章纲和未来 10 章；批次规划、规划复盘、大纲修订、设定协调及伏笔检查同样必须复用有界历史，不得恢复全量历史拼接。
- 伏笔、摘要、事实核查和大纲一致性检查必须服从明确的结尾意图；通用章尾钩子不能覆盖收尾要求。

### 导入与完稿校订

- 导入先本地切章，再写 accepted 章节并通过 `import.json` 保存断点；元信息和逐章分析按检查点续跑。
- `postprocess.json` 只接受 v1；自动校订保持 block 数量与对应关系，不改变剧情结构，支持逐章撤销。
- 校订报告锚定章节/block；正文变动后刷新锚点。校订不触发写作知识同步。
- 完稿后创建的续写项目以 `inherited` 标记前作章节与伏笔；前作章节保留为规划上下文但不计入新项目章节进度，伏笔使用独立快照并在界面标明继承来源语义。

## LLM、提示词与 Skill

- API 地址通过 `resolveChatCompletionsURL` 统一解析；严格模式只补 `/chat/completions`，普通裸域名补 `/v1/chat/completions`。
- 流式读取使用 `bufio.Reader`；损坏 SSE、缺少 `finish_reason`/`[DONE]` 或提前 EOF 都是错误，半截响应不得进入 JSON 解析。
- 401/403/404 为致命错误；可重试错误沿用现有指数退避。Agent 已收到流片段后失败时不得再拼接同步回退结果。
- 每个模型请求以 `ContextBudgetTokens - MaxTokens - max(4096, 5% 上下文窗口)` 作为保守估算的输入上限；模型端点能报告更小的真实窗口时自动向下收紧配置，最终预检超限属于不可重试错误。
- 提示词占位符是 `config.RenderPrompt` 的 `{{.Key}}` 字符串替换，不是 `text/template`。
- 新增 prompt 字段时同步更新 `PromptsConfig`、中英默认模板和 `ApplyDefaults`；新增注入块或 system prompt 必须同时提供中英文。
- Skill 包必须包含 schema v1 的 `skill.json` 及其声明的 Markdown 入口（通常为 `SKILL.md`），仅接受安全校验后的 `.md/.txt/.json`；用户 Skill 默认禁用，并按项目语言、`applies_to` 和动作类别过滤。
- 新建项目默认启用对应语言的内置 `adult-fiction-zh` / `adult-fiction-en`，既有项目加载时不得补开。该 Skill 固定为成人尺度 4，仅进入大纲生成/修订、章节生成/修订和润色，必须限定明确成年且明确合意的虚构参与者；中文版本维护直白、文学、现代网文三种语域的紧凑词汇与用法规则。
- 内置 Skill 位于 `internal/story/embeds/skills/`，修改后需要重新编译。

## 前端约定

- 启动立即尝试恢复服务端当前项目，不等待版本检查；SSE 恢复或任务开始也会触发恢复。任务运行时禁止返回项目列表。
- 所有请求复用 `api` / `apiFetch`，携带当前 UI 语言并统一解析本地化错误；下载失败不得提示成功。
- 前端挂载前必须先初始化 `lib/desktop.js`；Web 模式保持相对 URL，GUI 模式统一拼接 loopback API Base、注入临时令牌并构造带查询令牌的 SSE URL。导出复用 `lib/downloads.js`，GUI 走原生保存，Web 走 Blob 下载。
- UI 文案走 `$t('key', params)`；后端日志/Agent 结果使用 key + args；新增可见文案必须同步 `zh.js` 与 `en.js`。
- 项目语言不可变；选择/创建项目时 UI 语言可跟随项目初始化，之后允许独立切换。
- 使用 `@xianii/design-system` token。正文默认 16px；`text-sm` 用于紧凑控件，`text-xs` 仅用于元数据。
- 长期视觉规范维护于根目录 `DESIGN.md` 与 `.impeccable/design.json`。应用壳层在 1280px 以上保持导航、工作区、助理三栏；低于 1280px 助理改抽屉，低于 1024px 导航也改抽屉，低于 768px 固定双栏编辑器顺序堆叠。顶栏项目标题不得截断，可自然折行；抽屉相对工作区定位，并具有遮罩、可见关闭操作和 reduced-motion 降级。图谱自动适配视口使用短时缓动，reduced-motion 直接应用最终镜头。
- 全局平面风格：`--depth: 0`，无阴影；主要操作实心语义色，普通操作 `btn-outline`，弱操作 `btn-ghost`，危险操作 `btn-error btn-outline`。
- `tabs-box` 是统一描边分段控件；活动项主色填充。正文段落 hover 淡底、点击选中，文字操作栏仅在选中后显示。
- 中间工作区与助理约 2:1，助理宽 18–28rem；中间列必须 `min-w-0`，写作正文网格使用 `minmax(0, 1fr)`。
- 写作章节列表宽 345px；章节正文按需加载，重复点击当前章节必须能够重试失败请求。
- 事实/设定面板位于正文卡片内并默认折叠；摘要常驻，事实标记可展开并定位来源段落。
- 核心操作必须有直接页面按钮，不依赖聊天；破坏性操作用 `ConfirmModal`，基础可访问性不得为精简让步。
- 主界面「卡片库」以可并行操作的右侧栏呈现；配置标题和单条设定都必须提供常显的「加入卡片库」按钮，拖入保存仅作为快捷方式；共享卡必须提供常显的小型「使用」按钮，也可拖回同类型区域。导入组织或关系时自动补齐缺少依赖；已有正文的故事/风格卡要求二次确认，类型、风格或视角变化沿用设定协调流程。
- 共享卡编辑器必须按根卡类型展示与配置页语义一致的完整字段，不得只编辑卡片标题和说明；编辑只修改库中副本。组织成员和关系端点只能引用卡片内已保存的依赖快照。API 详情继续隐藏密钥，编辑时空密钥表示保留原值。
- 共享卡编辑器的字段布局复用配置页的紧凑表单规格：小号块级标签、`w-full` 的 `input/select/textarea`、双列响应式网格和 `bg-base-300` 内层面板；弹窗头部、滚动正文和底部操作区相互独立，禁止恢复依赖 `form-control` 的漂移布局。卡片库条目沿用设定卡的图标、摘要和右侧操作区结构。
- SSE 的 content/chat chunk 使用现有缓冲节流；任务结束清空聊天缓冲，重连、结束事件和运行中轮询均以 `/api/status` 校准状态，不使用本地事件计数判定空闲；跨越更新任务事件或请求的旧响应不得覆盖新状态。

## 测试与精简规则

- 测试按领域文件组织；同一函数的输入变体优先表驱动，跨流程测试保留独立名称。不要仅为减少文件数合并无关测试，Go 已按 package 统一编译。
- 测试 helper 先用标准库和现有 helper；只有多个测试共享且能明显减少重复时才新增 helper。
- README 截图由 `frontend/scripts/screenshots.mjs` 使用系统 Google Chrome 与固定离线样例按 README 语言生成 WebP 到 `docs/screenshots/<语言>/`；工作流只在前端、截图文件或截图工作流变更的 PR 中运行，并允许手动触发。
- 读取源码执行的前端回归脚本必须兼容 LF/CRLF，不依赖平台或 Git 换行配置。
- 非平凡分支、解析、存储安全或兼容边界的改动必须留下最小回归测试；纯删除死代码无需新增测试。
- 不新增单实现接口、工厂、无消费者配置、转发 wrapper 或“以后可能用”的兼容层。
- 优先删除死代码和重复文档；API/SSE/prompt 的完整枚举以路由、结构体和源码为准，不在本文件复制一份。

## 修改检查清单

1. 搜索所有调用者，修共享根因而不是单个症状。
2. 保持包依赖、任务互斥、原子保存、版本校验和双语边界。
3. API 变更核对 `internal/httpapi/web.go`；SSE 变更核对 `frontend/src/lib/sse.js`。
4. prompt/system prompt/注入块/前端文案同步中英文。
5. 执行本文件“常用命令”中的适用检查。
6. 同步更新本文件；用户行为变化时再同步中英文 README。

## graphify

项目知识图谱位于 `graphify-out/`（已 gitignore）。代码库问题优先运行：

```bash
graphify query "<问题>"
graphify path "<A>" "<B>"
graphify explain "<概念>"
```

代码修改后运行 `graphify update .`；图谱生成文件变脏不构成跳过理由。
