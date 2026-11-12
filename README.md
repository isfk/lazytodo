# lazytodo

lazygit 架构的极简 TUI 待办清单。左列表、右详情、标题栏状态筛选，约 3400 行。

**中文** | [English](README.en.md)

## 安装

```bash
go install github.com/isfk/lazytodo@latest   # 还没打 tag 时用 @main
```

需要 Go 1.26+。也可以 `git clone` 后 `go run .`。

想让 agent 直接改待办列表，再装一次扩展：TUI 里按 `I`，或 `lazytodo --install-extension pi`。见[让 agent 改待办](#让-agent-改待办)。

## 用法

数据存在**当前目录**的 `.lazytodo.json`——换个目录就是另一份待办。启动时若没有这个文件会自动创建一个空列表。

```
╭─待办─全部 - 进行中 - 完成──╮╭─详情─────────────────────────╮
│ ✓ 写存储层                 ││原子写                          │
│ ◐ 写界面                   ││                                │
│ ○ walk the dog             ││                                │
╰────────────────────────────╯╰────────────────────────────────╯
1/3 完成 · 1 进行中
[␣] 切换状态 [a] 新增 [e] 编辑 [d] 删除 [/] 过滤
```

| 键 / 操作 | 动作 |
|---|---|
| `↓` `↑` / `j` `k` | 移动 |
| `g` `G` | 跳到首 / 尾 |
| `space` | 循环 todo → doing → done |
| `a` / `e` | 新增 / 编辑（标题回车即提交，详情多行，`^s` 提交） |
| `d` | 删除（先确认，按 `y` 才生效） |
| `/` | 文本过滤（标题和详情都匹配） |
| `⇥` `⇧⇥` / 点标签 | 状态筛选 All → Doing → Done |
| `l` `h` | 进 / 出详情面板（面板内 `y` 复制选中行） |
| `esc` | 清除过滤 |
| `ctrl+d` / `q` | 重新读盘 / 退出 |
| `I` | 把 agent 扩展装进 `~/.pi/agent/extensions/` |

界面语言按 `LC_ALL` → `LC_MESSAGES` → `LANGUAGE` → `LANG` 自动中英切换；中英混排按字符显示宽度对齐与截断（中文占 2 格）。

## 让 agent 改待办

扩展源码在仓库里（`pkg/agents/pi/lazytodo.ts`），编译时用 `go:embed` 打进二进制，装的时候不下载任何东西。两种装法：

- TUI 里按 **`I`**
- 或者 `lazytodo --install-extension pi`

写到 `~/.pi/agent/extensions/lazytodo.ts`：目录不存在会建，已有文件内容不同会先备份成 `lazytodo.ts.bak.<时间戳>`。装完在 pi 里 `/reload` 或重开会话即可生效。

装好之后 pi 多了一个 `lazytodo` 工具，agent 能直接列 / 增 / 改 / 删当前目录的待办。TUI 每 400ms 对比一次文件 mtime，变了自动重载——不用按键，屏幕自己更新。

| action | 参数 | 作用 |
|---|---|---|
| `list` | `filter?` | 列出，可按状态筛 |
| `add` | `title`、`detail?` | 新增，返回 id |
| `set` | `id`、`status` | 改状态（todo / doing / done） |
| `remove` | `id` | 删除 |

扩展直接写 `.lazytodo.json`（和 TUI 一样的原子写：临时文件 + rename），并走 pi 的 `withFileMutationQueue`，不会和内置 `edit` / `write` 抢写。

> 列表按目录隔离，所以 agent 和 TUI 得在同一个工作目录，才看得到同一份。

以后要接 codex / claude，只需在 `pkg/agents` 里加一条 `Agent`（目标目录 + 源文件），TUI 的 `I`、`--install-extension`、测试都是复用的。

## 分层

| 包 | 干什么 |
|---|---|
| `pkg/commands` | 数据层：JSON 存储 + 原子写，零 UI 依赖 |
| `pkg/gui/types` `context` `controllers` | 面板状态、按键与点击处理器 |
| `pkg/gui/presentation` | model → 文本行的纯函数（对齐、截断、样式） |
| `pkg/tui` + `tcelldriver` | 终端抽象（View / Screen / FakeScreen），只有驱动 import tcell |
| `pkg/gui` | 组装、布局、刷新、按键分发 |
| `pkg/i18n` | 界面文案（中 / 英） |
| `pkg/integration` | 无头集成测试 DSL |

数据流是单向的：按键/点击 → 改 store → `Refresh()` → 各面板重画，没有逆向推送。

## 测试

```bash
go test ./...   # 97 个：命令层 / 表现层 / tui / 驱动 / 无头集成
```

依赖只有 `github.com/gdamore/tcell/v3` 和它自带的宽度表 `displaywidth`。
