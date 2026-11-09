# lazytodo

一个模仿 [lazygit](https://github.com/jesseduffield/lazygit) 架构的极简 TUI 待办清单。

目的不是做一个待办应用，而是把 lazygit 的分层方式**裁剪到最小可读规模**：19 个源文件约 3400 行，每一层都还能认出 lazygit 里对应的那个包。

## 安装

```bash
go install github.com/isfk/lazytodo@latest
```

装完直接 `lazytodo`（确认 `$(go env GOPATH)/bin` 在 `PATH` 里）。需要 Go 1.26 以上。

还没打 tag 时 `@latest` 解析不到版本，改用分支：

```bash
go install github.com/isfk/lazytodo@main
```

模块代理拉不到 GitHub 时（镜像没缓存、内网），绕过代理直连：

```bash
GOPROXY=direct go install github.com/isfk/lazytodo@latest
```

从源码跑：

```bash
git clone https://github.com/isfk/lazytodo
cd lazytodo
go run .
```

## 用法

数据存在**当前目录**的 `.lazytodo.json`。换一个目录启动就是另一份待办，互不干扰。文件不存在就是空列表，第一次改动才会创建。

想在仓库里用，把它加进 `.gitignore`。

## 界面

```
╭─ Todos ─All - Doing - Done─╮╭─ Detail ─────────────────────────────────────┐
│  ✓ write the store           ││atomic rename                                │
│  ◐ write the gui             ││                                             │
│  ○ walk the dog              ││                                             │
│                              ││                                             │
└──────────────────────────────┘└──────────────────────────────────────────────┘
1/3 done · 1 doing
[␣] Cycle [a] Add [e] Edit [d] Delete [/] Filter
```

左侧列表（标题栏上带状态筛选标签）、右侧详情、底部状态栏和按键提示栏。宽度小于 40 列时只留列表。

## 状态筛选标签

`Todos` 标题栏那一行上排着 **All / Doing / Done**，**用鼠标点一下就能切换**，当前生效的那个只是文字高亮（加粗），**不加背景色**。

版式照 lazygit 来：两边是边框线，标签之间用 ` - `（空格 + 短横 + 空格）：

```
╭─ Todos ─All - Doing - Done────────────────╮
            ▲ 加粗 = 正在生效
```

- 标签位置由 `View.TabSpans()` 从标题和标签文字算出来，**绘制和命中判定用的是同一个函数**，所以永远点得准
- 点击先于面板判定：标签画在边框上，如果先按「点在视图范围内」判就会误吞
- 空间不够时标签从右往左被丢弃，`TabSpans()` 和绘制一起缩
- 想加 `Todo` 标签？`presentation.statusTabStatuses` 里加一行 `models.StatusTodo` 即可

## 三种状态

`space` 让待办在 **todo → doing → done → todo** 之间循环：

| 标记 | 状态 | 行样式 |
|---|---|---|
| `○` | todo | 默认 |
| `◐` | doing | 黄色加粗 |
| `✓` | done | 绿色变暗 |

> `○` 和 `◐` 在 Unicode 里属于「东亚宽度不确定」字符：多数终端按 1 列渲染，部分 CJK 配置的终端按 2 列。本项目按 1 列算。

## 界面语言（中/英自动识别）

启动时读 `LC_ALL` → `LC_MESSAGES` → `LANGUAGE` → `LANG`（和 libc 一样的优先级），`zh_*` 显示中文，其他显示英文，跟 lazygit 一个思路：

```
╭─ 待办 ─全部 - 进行中 - 完成╮╭─ 详情 ─────────────────────────────────────╮
│ ○ 写存储层                 ││原子写                                      │
╰────────────────────────────╯╰────────────────────────────────────────────╯
0/2 完成
[␣] 切换状态 [a] 新增 [e] 编辑 [d] 删除 [/] 过滤
```

所有文案都在 `pkg/i18n` 的一张表里，中英各一份；表里漏一个字段或两边的 `%` 数量对不上，测试会直接报错。想强制某种语言，改 `config.UserConfig.Language`（还没有配置文件，所以目前只能改代码）。

## 中英混排

宽度是按**字符的显示宽度**算的，不是按字数：中文/日文/韩文和 emoji 占 2 格，英文占 1 格，自动识别，不用配置。

```
│ ○ 写存储层 write the store  │   ← 边框对齐
│ ○ a mixed 中英 title        │
```

三处都用同一个 `tui.DisplayWidth`：列对齐、按宽度截断（不会把中文截成半个）、绘制时按格子推进（一个汉字占两格，后一个字不会盖上去）。长中文行超出面板宽度时截断并加 `…`。

## 边框

圆角 `╭ ╮ ╰ ╯`。边框**永远用终端默认前景色**——它是装饰，写死颜色会和主人的配色打架。

高亮只属于**激活**的那个面板：

| | 激活 | 非激活 |
|---|---|---|
| 面板标题（Todos / Detail） | 本色 + 加粗 | 终端默认色、不加粗 |
| 光标所在行 | 选中底色 | 不画高亮 |
| 状态筛选标签 | 加粗（那个是状态，不看焦点） | 变暗 |

## 按键与鼠标

| 键 / 操作 | 动作 |
|---|---|
| `↓` / `↑` 或 `j` / `k` | 上/下移动 |
| `g` / `G` | 跳到首/尾 |
| `space` | 循环三态 |
| `a` | 新增（打开表单） |
| `e` | 编辑（表单预填标题和详情） |
| `d` | 删除（会先弹确认） |
| `/` | 文本过滤（标题和详情都匹配） |
| `esc` | 清除**两种**过滤 |
| `ctrl+d` | 重新读盘 |
| `q` | 退出 |
| `⇥` / `⇧⇥` | 循环状态筛选：All → Doing → Done（可以绕回） |
| `l` / `→` | 光标进详情面板 |
| `h` / `←` | 光标回列表（详情面板里） |
| `y` | 复制详情面板里选中的那一行（详情面板里） |
| 点标签 | 切换状态筛选 |
| 点列表行 | 选中该行 |

## 详情面板可选中 / 可复制

`l`（或点一下右边面板）把光标移进详情，`j/k` 选行，`y` 复制选中的那一行，`h` 回列表。
选中行会高亮，底部提示栏变成 `[y] Copy [h] Back`。

复制的走终端自己的剪贴板通道（OSC 52），所以**不需要额外依赖、ssh 里也能用**；先在本地找 `pbcopy` / `wl-copy` / `xclip`，没有再退回 OSC 52；两者都没有时才报错。

复制的是**未截断的原文**——面板窄到把长行截成 `…` 时，拷进去仍是完整那一行。没有详情时 `y` 会提示 `Nothing to copy`。

## 表单（弹窗）

`a` / `e` 会打开一个浮层表单，标题在上边框、提示（或拒绝原因）在下边框：

```
     ┌─ New todo ───────────────────────────────────────────────────┐
     │▸ Title                                                       │
     │  buy milk█                                                   │
     │                                                              │
     │  Description                                                 │
     │  get oat milk                                                │
     │  2 litres█                                                   │
     └─ [⇥] next field  [⏎] newline  [^s] submit  [esc] cancel ─────┘
```

| 键 | 行为 |
|---|---|
| 标题行 `⏎` | **直接提交**——「打个标题按回车」是快速路径 |
| 详情行 `⏎` | 插入换行（详情支持多行） |
| `^s` | 任意字段都能提交 |
| `⇥` / `⇧⇥` | 切换字段（循环） |
| `esc` | 取消，不写盘 |

提交被拒绝时（比如标题为空）弹窗**不关闭**，原因显示在下边框，已输入的内容不丢。弹窗打开时鼠标点击也被吞掉，模态是真的模态。

新增时如果当前过滤会遮住新条目（比如正点着 `Done`），两个过滤器会被清掉，免得看上去像没加上。

## 删除确认

`d` 不会直接删，先弹一个只有 y / n 的模态：

```
     ┌─ Confirm ────────────────────────────────────────────────────┐
     │Delete "a rather long todo title that will need to wrap in the│
     │popup"?                                                       │
     └─ [y] yes  [n/esc] cancel ────────────────────────────────────┘
```

| 键 | 行为 |
|---|---|
| `y` | 真的删除 |
| `n` / `esc` | 放弃，什么都不动 |

**`⏎` 故意没绑**——删除是这里唯一不可逆的操作，不该和「提交表单」共用一个肌肉记忆按键。其他任何键（`j`、方向键、空格、`⇥`）都被吞掉，问题会留在屏幕上。

长标题会自动折行，所以能看清到底要删哪一条。

## 分层

| lazygit | 这里 | 干什么 |
|---|---|---|
| `pkg/commands` | `pkg/commands` | 数据层。JSON 存储 + 原子写 + 三态流转 + 两层过滤。**零 UI 依赖** |
| `pkg/gui/presentation` | `pkg/gui/presentation` | model → 文本/标签的纯函数。列对齐、截断、CJK 宽度、行样式 |
| `pkg/gui/types` | `pkg/gui/types` | 共享词汇：`Binding`、`Context`、`State`、`PopupState`、`IGuiCommon` |
| `pkg/gui/context` | `pkg/gui/context` | 面板状态 + 挂载点。`AttachControllers` 靠可选接口做类型断言 |
| `pkg/gui/controllers` | `pkg/gui/controllers` | 按键与点击处理器。不画东西，只改数据然后 `Refresh()` |
| `pkg/gocui` | `pkg/tui` + `pkg/tui/tcelldriver` | 终端层。`Screen` 接口 + 自带 `FakeScreen` |
| `pkg/gui` | `pkg/gui` | 组装、Layout、Refresh、按键/点击分发、弹窗状态机 |
| `pkg/integration` | `pkg/integration` | 无头集成测试 DSL |

数据流是单向循环，没有逆向推送：

```
按键/点击 → gui.HandleKey / HandleClick → binding.Handler() / ctx.HandleClick()
          → store 变更 + Save() → gui.Refresh() → Layout
          → 各 context.HandleRender() → presentation → View
          → gui.Render() → Screen.SetCell → 终端
```

## 七个刻意保留的设计

**1. `GetDisabledReason`**

binding 自带「为什么现在不能用」，handler 里一个守卫 if 都没有：

```go
{
    Keys:              []tui.Key{keys.CycleStatus},
    Handler:           self.cycleStatus,
    GetDisabledReason: self.requireItem,   // 空列表时返回 "No todo selected"
    DisplayOnScreen:   true,
}
```

**2. 提示栏是推导出来的，不是写死的**

`renderOptions` 遍历焦点面板的 binding，只保留 `DisplayOnScreen && Disabled() == nil` 的。空列表时自动变成 `[a] Add [/] Filter`。

**3. 行首留白是一个常量**

列表每行开头有 `listIndent`（1 列）把状态图标推离边框，不然会糊在边框上。改这一个常量就能调。

**4. 行样式随文本一起产出**

`presentation.TodoLines` 返回 `(lines, styles)`，`View` 有一份可选的 `LineStyles`。所以「done 的行变暗」不需要 view 认识 todo，也不需要往文本里塞 ANSI 转义。

**5. 标签几何与绘制的唯一真相**

`View.TabSpans()` 是纯函数（标题 + 标签文字 + 边框宽度 → 坐标区间）。绘制走它，点击命中走它，集成测试 `TabCenter()` 也走它——三处不可能不一致。

**6. 一种弹窗机制覆盖四种用途**

新增表单、编辑表单、过滤框都是同一个 `PopupState` + 同一套字段编辑逻辑，区别只是字段数量和 `Multiline` 标志。`OnSubmit` 返回 error 就让弹窗留着并显示原因。删除确认是同一个模态的另一种形态：没有字段、只有一句问话和一个 `OnConfirm`，于是键盘处理、居中、折行、footer 全都是复用的。

**7. 键盘输入在边界处归一化一次**

终端把空格键当成普通字符（`KeyRune` + `" "`）报上来，而配置里绑的名字是 `KeySpace`。`KeyOfRune` / `RuneOfKey` 是唯一做这次翻译的地方：驱动往下走得到 `KeySpace`（binding 能匹配），文本输入往回取得到 `' '`（能敲进标题）。两边共用一对函数，就不会再出现「测试里能按、真机上按不动」。

## 集成测试

```go
RunTest(t, TestCase{
    Setup: func(shell *Shell) {
        shell.Add("one", "two", "three")
        shell.Cycle(2) // two -> doing
    },
    Run: func(d *TestDriver) {
        d.Click(d.Todos().TabCenter("Doing"))   // 点标题栏上的标签

        d.Todos().Lines(Contains("two").IsSelected())
        d.Status().Lines(Equals("0/3 done · 1 doing · only doing"))
    },
})
```

真按键/真点击 → 真 dispatch → 真 store → 真渲染到 `FakeScreen`，断言的是用户会看到的那几个字符。`d.Keys` 引用配置，`IsSelected()` 校验光标位置，`TabCenter()` 从面板几何算出坐标——测试不写死任何像素或空格数。

## 测试

```bash
go test ./...          # 97 个测试
go vet ./... && gofmt -l .
```

- `pkg/commands` — 存储层单测：残缺文件、ID 递增、原子写往返、三态流转、两层过滤叠加、**旧版 `done: bool` 格式向后兼容**
- `pkg/gui/presentation` — 列对齐、截断、CJK 宽度、三态标记与行首留白、筛选标签、弹窗焦点与光标、**折行**、多行详情
- `pkg/tui` — 滚动窗口、选区裁剪、边框与边栏文字、**标签几何与命中判定**、**空格键的双向归一化**、不可见视图
- `pkg/tui/tcelldriver` — tcell 事件 → `tui.Event` 的翻译表（键盘、**空格键**、**鼠标点击**、resize、各种忽略项）
- `pkg/integration` — 端到端：三态循环、空格键、方向键、点标签切换筛选、点行选中、双过滤与清除、模态吞点击/吞按键、窄屏降级、表单新增/编辑/取消/校验、**删除二次确认**、多行详情

## 依赖

只有一个直接依赖 [`github.com/gdamore/tcell/v3`](https://github.com/gdamore/tcell) —— 和 lazygit 现在用的同一套终端库。`pkg/gui` 和 `pkg/tui` 都不 import 它，只有 `pkg/tui/tcelldriver` 知道 tcell 的存在。

宽度计算用 `github.com/clipperhouse/displaywidth`（Unicode 宽度表）。它本来就是 tcell 的传递依赖，没有新增下载。

## 相比 lazygit 砍掉了什么

| lazygit | 这里 | 为什么 |
|---|---|---|
| `baseController` 嵌 10 个 `GetXxxFn()` 默认实现 | 四个可选接口 + 类型断言 | 只在控制器真需要时才实现 |
| `BaseContext` 二十多个字段（tabs / transient / rerenderOnWidthChange…） | 6 个字段 | 单窗口不需要 |
| `oscommands` 的 pty / 凭据提示 / index.lock 重试 | 直接 `os.ReadFile` + 原子 rename | 没有外部进程要管 |
| 鼠标拖拽、滚轮、悬浮高亮 | 只有左键点击 | 标签和行选中就够了 |
| 光标可在任意位置插入（文本编辑器） | 只在字段末尾追加/退格 | 输入待办不需要中途编辑；这是下一步 |
| 长行自动折行 | 按列宽截断加省略号 | 折行会让行号与模型索引脱钩，需要额外映射 |
| i18n、YAML 配置、自定义命令、undo、worktree、多选 | 无 | 每一样都是一整个子包 |
