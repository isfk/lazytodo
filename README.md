# lazytodo

lazygit 架构的极简 TUI 待办清单。左列表、右详情、标题栏状态筛选，约 3400 行。

**中文** | [English](README.en.md)

## 安装

```bash
go install github.com/isfk/lazytodo@latest   # 还没打 tag 时用 @main
```

需要 Go 1.26+。也可以 `git clone` 后 `go run .`。

## 用法

数据存在**当前目录**的 `.lazytodo.json`——换个目录就是另一份待办。文件不存在即空列表，第一次改动才创建。

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

界面语言按 `LC_ALL` → `LC_MESSAGES` → `LANGUAGE` → `LANG` 自动中英切换；中英混排按字符显示宽度对齐与截断（中文占 2 格）。

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
