# NodeLink 前端设计指南

> 面向 AI Agent 的设计与实现规范。本文件是**唯一规则来源**：生成任何界面之前先读它，产出之后按第 13 节自检。
> 配套文件：`design-system/tokens.css`（变量层）· `design-system/styleguide.html`（组件样本页）· `app/`（8 屏参考实现）

---

## 0. Agent 读取协议

| 顺序 | 动作 | 说明 |
|---|---|---|
| 1 | 读本文件全部 | 不要跳读第 2 节（反模式）和第 13 节（自检） |
| 2 | `@import` tokens.css | 禁止在业务样式里出现裸色值 `#xxxxxx` |
| 3 | 打开 styleguide.html 对照 | 需要确认组件细节时看真实渲染，不要凭记忆猜 |
| 4 | 套用第 12 节页面模板 | 新页面必须从已有骨架派生，不要从空白开始 |
| 5 | 跑第 13 节清单 | 任一条不通过就修，不要交付 |

**三条硬约束（违反即返工）**

1. 单强调色：全局只有一个强调色 `#5bcfc5`，其他彩色仅限语义状态（成功/警告/危险/下行）。
2. 深色唯一：不存在浅色主题，不要生成 `prefers-color-scheme: light` 分支。
3. 尺寸取自 token：颜色、圆角、字号、控件高度、骨架宽度必须用变量，不写魔法数字。

---

## 1. 设计立场

**定位**：面向技术运维人员的付费网络节点管控平台（客户端 + 服务端 + 开通流程）。用户每天盯 8 小时，屏幕上是延迟、丢包、流量、密钥。

因此三条原则按优先级排序：

1. **信息密度优先于呼吸感**。12px 基准字号、60px 表格行、14px 卡内边距。宁可挤，不要空。留白过剩等于浪费用户的屏幕。
2. **层级靠明度与描边，不靠色彩**。一个页面里强调色面积不超过 5%，出现在：主按钮、当前选中项的强调描边/文字、关键正向数值。
3. **状态必须可读**。颜色 + 文字标签 + 圆点三者至少同时出现两者。禁止只用颜色表达状态。

---

## 2. 反模式清单（Agent 最常犯的 10 个错误）

| # | 禁止 | 正确做法 |
|---|---|---|
| 1 | 用 Tailwind 默认灰蓝（`slate-800`、`zinc-700`） | 用 `--card` `--border` `--line2`，本系统色相略带紫，冷灰会显脏 |
| 2 | 给卡片加多层阴影、玻璃拟态、渐变边框 | 卡片 = 纯色底 + 1px `--border` + `--r-xl`，仅此 |
| 3 | 用 `--accent` 做大面积背景或整块卡片 | 强调色只用于小面积（按钮、pill、数值、选中描边） |
| 4 | 按钮高度随手 36/40px | 只用 26 / 32 / 42 三档 |
| 5 | 正文用 14/16px | 正文 11px，标题 13px，页面标题 15px |
| 6 | 中文字体 thought 用 Inter | 中文 `--sans`，数字/ID/IP/密钥 `--mono`，混用会崩 |
| 7 | 卡片之间间距 20/25/30px 随手取 | 4px 阶梯，卡间距固定 12px，区块间距固定 14px |
| 8 | 表格行高 48px + 内边距，看起来很"现代" | 行高 60px，内容垂直居中，这是数据页的密度基线 |
| 9 | 空态画一个大插画 | 空态 = 一行 `--t3` 说明 + 一个次级按钮，居中，无插画 |
| 10 | 加动画：卡片入场、数字滚动、hover 放大 | 只允许颜色/描边/背景过渡（120–150ms），加 `translateY(0.5px)` 按下反馈即可 |

---

## 3. 色彩

### 3.1 表面层（5 级，从深到浅）

```
--bg      #0a0a0b   页面底
--rail    #0c0c0e   顶栏 / 侧栏 / 底部操作栏
--panel   #101013   表头、卡内分区
--card    #141417   卡片、输入框、表格容器
--card-active #18181c  选中行
```

任何"看起来该有底色"的容器，只从这 5 个里选，不要新建。

### 3.2 描边（三层）

| Token | 值 | 用在 |
|---|---|---|
| `--border` | `#27272a` | 卡片、输入框、表格外框（默认描边） |
| `--border-strong` | `#3a3a44` | hover 态卡片、focus 态容器 |
| `--line` | `#1b1b1f` | 侧栏与主区之间的竖分割 |
| `--line2` | `#1d1d21` | 表格行之间的横分割 |
| `--line3` | `#1f1f23` | 卡内小节分割 |

### 3.3 语义色（仅用于状态）

| 语义 | 文字/图标 | 背景 | 描边 | 使用场景 |
|---|---|---|---|---|
| 成功 / 在线 | `--green` `#34d399` | `--accent-bg` | `--accent-border` | 节点在线、健康检查通过、激活成功 |
| 强调 / 品牌 | `--accent` `#5bcfc5` | `--accent-bg` | `--accent-border` | 主按钮、当前选中、关键正向数值 |
| 警告 / 到期 | `--amber` `#f59e0b`（文字用 `--amber-2` `#f5c56b`） | `--amber-bg` | `--amber-border` | 即将到期、套餐锁定、配额临界 |
| 危险 / 吊销 | `--rose` `#f43f5e` | `--rose-bg` | `--rose-border` | 已吊销、失败、危险操作按钮 |
| 下行 / 信息 | `--blue` `#4f8fea` | `--blue-bg` | `--blue-border` | 下行流量、中性提示 |

**对比度底线**：`--t3` 及更暗的文字只能用于 ≤10px 的辅助标签；正文与关键信息必须 `--t2` 以上。琥珀色文字在深底上必须提亮到 `--amber-2`。

### 3.4 图表配色（固定顺序，不要重新编排）

```
主序列（上行/主指标）  --accent #5bcfc5   2px 线 + 面积渐变 0.18 → 0
次序列（下行/对比）    #52525b(--t4)      1.5px 虚线或实线
参考线                --accent-deep #2b7a7a  1px dasharray 3 3
网格线                --grid #1f1f23     仅横向，2–4 条
```

---

## 4. 排版

### 4.1 字体栈

```css
--sans: "Noto Sans SC", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", system-ui, sans-serif;
--mono: "JetBrains Mono", "SFMono-Regular", ui-monospace, "Cascadia Mono", Consolas, monospace;
```

不引入外部 webfont（离线客户端 / 内网环境必须可用）。`--mono` 缺失时降级到系统等宽，视觉可接受。

### 4.2 什么时候用等宽

必须 `--mono`：IP、端口、延迟数值、丢包率、流量、带宽、密钥、进程名（`chrome.exe`）、版本号、时间戳、表头里的数值列、KPI 数值。
必须 `--sans`：所有中文、按钮文案、导航项、标题、说明段落。

数值统一加 `font-variant-numeric: tabular-nums`（用 `.num` 类），否则数据刷新时会横向抖动。

### 4.3 字号阶梯（用哪一档，逐条列清）

| 字号 | 用途 | 字重 | 颜色 |
|---|---|---|---|
| 26px | 流程页主标题（激活页"激活你的客户端"） | 600 | `--t1` |
| 22px | 结果页大数值（成功率、金额） | 500 | `--t1` |
| 20px | KPI 数值 | 500 | `--t1`（关键项可 `--accent`） |
| 18px | 流程页卡片内大标题 | 600 | `--t1` |
| 15px | 页面标题（`.page-title`） | 600 | `--t1` |
| 13px | 区块标题（`.sec-title`）、卡片主标题、节点名 | 600 / 500 | `--t1` |
| 12px | 侧栏导航、面板正文 | 400–500 | `--t2`（选中 `--t1`） |
| 11px | **正文默认**、按钮、输入框、表格主文本 | 400–500 | `--t2` / `--t1` |
| 10px | 表头、KPI 标签、元信息、单位 | 400 | `--t3` |
| 9px | 图例、角标、mini 分段按钮 | 400–500 | `--t3` |

**字重只有三档**：400 / 500 / 600。不存在 700、800。中文在深色背景上 600 已足够重。

**字间距**：大标题 `letter-spacing: 0.1px`；KPI 数值 `-0.4px`；其余保持 0。中文不要加负字距。

### 4.4 行高

- 标题与单行标签：`1.3`
- 正文默认：`1.4`
- 多行说明段落（安全提示、条款）：`1.6`，且颜色降为 `--t3`

---

## 5. 间距、尺寸、布局骨架

### 5.1 间距阶梯

`2 / 4 / 6 / 8 / 10 / 12 / 14 / 16 / 20 / 24 / 28 / 32 / 40`

实用规律：

| 关系 | 间距 |
|---|---|
| 图标与文字 | 6–8px |
| 同行控件之间 | 8–10px |
| 卡片内元素（纵向） | 8–14px |
| KPI 卡网格 gap | 12px |
| 区块之间（纵向） | 14px |
| 主内容区 padding | 24px |
| 右侧表单面板 padding | 20px |
| 流程页大面板 padding | 28–32px |

### 5.2 固定高度（不要自由发挥）

| 元素 | 高度 |
|---|---|
| pill / 状态胶囊 / mini 分段 | 26px |
| 按钮 / 输入框 / 搜索框 / 侧栏导航项 | 32px |
| 主流程主按钮 / 大密钥输入框 | 42px |
| 表格数据行 | 60px |
| 表头行 | 40px |
| 客户端标题栏 | 44px |
| 底部操作栏 | 52px |
| KPI 卡 | 76px |

### 5.3 三种外壳（选错骨架 = 整体返工）

**A. 客户端外壳** — 桌面工具，图标导航
```
┌───────────────────────────────────────────────┐
│ titlebar 44px（左标题 + 右状态胶囊/搜索）        │
├──────┬────────────────────────────────────────┤
│ rail │ content（padding 24px，纵向 14px gap）   │
│ 76px │                                        │
└──────┴────────────────────────────────────────┘
```
`rail`：`--rail` 底 + 右 `1px --line`，内部 `padding 12px`，图标按钮 40×40 / 圆角 8 / 选中态 `--accent-bg` + 图标 `--accent`。

**B. 服务端外壳** — 文字导航 + 面包屑
```
┌───────────────────────────────────────────────┐
│ breadcrumb 44px（左路径 + 右搜索/头像）          │
├────────┬──────────────────────────────────────┤
│sidebar │ content（padding 24px）                │
│ 240px  │                                        │
│        ├──────────────────────────────────────┤
│        │ statusbar 52px                         │
└────────┴──────────────────────────────────────┘
```
`sidebar`：`--rail` 底 + 右 `1px --line`，`padding 12px`，导航项 32px 高。顶部第一块是**工作区切换卡**（卡片底 + 描边），下方才分组导航。

**C. 流程页外壳** — 开通 / 激活 / 结果，无导航
```
┌───────────────────────────────────────────────┐
│ 可选顶栏 44px                                   │
│                                               │
│   max-width 容器（1120–1200px）居中             │
│   ├ 步骤条 44px                                │
│   ├ 主内容（左主区 + 右 aside 400px）            │
│   └ 底部操作条（吸底，--rail 底 + 上描边）        │
└───────────────────────────────────────────────┘
```

**壳外不允许出现第四种结构。**

### 5.4 分栏宽度（固定值）

| 用途 | 宽度 |
|---|---|
| 客户端图标栏 | 76px |
| 服务端侧栏 | 240px |
| 右侧表单 / 订单摘要 | 400px（`flex: 0 0 400px`） |
| 右侧详情信息栏 | 340px（`flex: 0 0 340px`） |
| 品牌面板（激活页左） | 560px |
| 搜索框 | 300px（紧凑 280px，撑满用 `flex:1`） |
| 表格最小可用宽度 | 880px |
| 设计基准画布 | 1440×900，最小 1120×760 |

主区一律 `flex: 1 1 auto; min-width: 0;`，右侧栏一律 `flex: 0 0 Npx`。**漏掉 `min-width: 0` 会导致表格把布局撑破**。

---

## 6. 组件规格

以下每个组件的尺寸与配色都已在 `app/styles.css` 中实现，实现时**直接复用类名**，不要重写。

### 6.1 按钮 `.btn`

```
高度 32px · 左右 padding 12px · 圆角 6px（--r-sm）
字号 11px · 字重 500 · gap 6px（图标与文字）
```

| 变体 | 底色 | 描边 | 文字 | 用途 |
|---|---|---|---|---|
| `.btn.primary` | `--accent` | 无 | `--accent-ink` | 每屏**最多一个**主行动 |
| `.btn.line` | `--card` | `--border` | `--t2` | 常规次级操作 |
| `.btn.ghost` | `#1c1c20` | `--t6` | `--t1` | 工具栏、行内操作 |
| `.btn.danger` | `--rose-bg` | `--rose-border` | `--rose` | 吊销、删除 |
| `.btn.solid-dark` | `#1a1a1e` | `--t6` | `--t1` | 深色实心次按钮 |

- 文字按钮 `.link`：11px，`--accent`，无背景无描边。
- 禁用：`opacity .45` + `cursor: not-allowed`，**不要改颜色**。
- 按下反馈：`transform: translateY(0.5px)`。
- 主流程大按钮：42px 高，字号 12px。

### 6.2 状态胶囊 `.status-pill`

```
高 26px · padding 0 8px · 圆角 6px · gap 6px · 字号 11px · 字重 500
内含 .dot 6×6 圆点
```
变体靠三件套（底/描边/文字）配对，见 3.3 表。文案用两字或三字中文：`在线` `警告` `已吊销` `即将到期`。

### 6.3 分段控件 `.seg`

- mini（卡内图表切换）：20px 高，字号 9px，圆角 4px，容器底 `#1a1a1e`。
- 标准（`.seg.bordered`）：26px 高，字号 11px，圆角 6px，容器底 `--card` + `--border`。
- 选中项：底 `#2a2a30` + 文字 `--t1` + 字重 500。**选中态不用强调色**，避免与主按钮抢注意力。

### 6.4 输入框 / 搜索框

| 类型 | 规格 |
|---|---|
| `.input-box` | 32px 高 / `--r-md` / `--card` 底 / `--border` 描边 / 内部 11px |
| `.search-box` | 与上相同 + 前置 14px 搜索图标 + `color: --t7` placeholder |
| 大密钥输入 | 42px 高 / `--r-lg` / `--mono` / 当前输入内容 `--accent` |
| focus | 描边变 `--border-strong`（搜索框用 `:focus-within`） |

### 6.5 卡片

**标准卡片 `.card`**
```css
background: var(--card);
border: 1px solid var(--border);
border-radius: var(--r-xl);   /* 12px */
padding: 14px;                 /* 内容卡 */
display: flex; flex-direction: column; min-height: 0;
```
hover（可点击时）：`border-color: --border-strong; background: --card-hover;`

**KPI 卡 `.kpi`**
```
76px 高 · padding 14px · 圆角 10px（--r-lg）
结构：标签(10px/--t3) → 数值行(20px mono/500) → 变化量(9px mono)
4 列 grid，gap 12px，`repeat(4, minmax(0,1fr))`
```

**节点卡 `.node-card`**
```
padding 12px · 圆角 10px · gap 8px
行1：国家/地区图标 20px + 节点名 13px/500 + 右侧延迟 11px mono
行2：IP + 区域 10px mono/--t3，单行截断
行3：上行/下行/负载 10px，gap 14px
```
网格：`repeat(3, minmax(0,1fr))`，gap 12px。

**右侧面板 `.panel`**：`--panel` 底 + 左 `1px --line`，`flex: 0 0 400px`，`padding: 20px`，内部纵向 `gap: 14px`，底部按钮用 `margin-top: auto` 吸底。

### 6.6 表格 `.table`

```
外容器：--card 底 / --border 描边 / --r-xl（12px）/ overflow hidden
表头 .t-head：40px 高 / --panel 底 / 10px 文字 / --t3 / 无下边框
数据行 .t-row：60px 高 / 上边框 1px --line2 / hover 底 #18181c
单元格 .cell：padding 0 14px / flex 居中 / gap 10px
```

- 列宽用 `flex: 0 0 Npx` 或 `flex: 1`，最后一列常为操作列（`flex: 0 0 60px`）。
- 行内双行结构：主文本 11px `--t2`（或 `--t1`）+ 次文本 10px `--t3`。
- 分组行 `.group-row`：36px 高 / `--panel` 底 / 10px `--t3`，左侧 3px `--accent` 竖条可选。
- 选中行：底 `#18181c`。
- 表格下方留白不要超 40px；若主区还有空间，补分页条或信息卡（参考第 8 屏）。

### 6.7 侧栏导航

```
项高 32px / 圆角 6px / padding 0 10px / 12px 文字 / gap 8px / 图标 15px
默认：文字 --t2，图标 --t3
hover：底 #16161b，文字 --t1
选中：底 --accent-bg + 文字 --accent + 图标 --accent（可加左 2px --accent 竖条）
分组标题 .section-label：9px / --t4 / letter-spacing 0.8px / 上边距 16px
```

### 6.8 步骤条 `.stepper`

```
节点：26px 圆 + 11px 序号/对勾
状态：已完成 = --accent 实心(序号用 --accent-ink) / 当前 = --accent 描边 + --accent 文字 / 未开始 = --t5 描边 + --t4 文字
连接线：1px，完成后 --accent-deep，未完成 --line3
标签：10px，仅当前步用 --t1，其余 --t3
```

### 6.9 进度条 / 资源占用

```
轨道 6px 高 / 圆角 3px / #1f1f23
填充：<70% --accent，70–85% --amber，>85% --rose
右侧数值：10px mono --t3
```

### 6.10 芯片 `.bind-chip` / `.nchip`

```
28px 高 / padding 0 10px / 圆角 8px / --card 底 / --border 描边 / 11px
前缀图标 12px（节点用国旗方块，密钥用钥匙）
可移除：右侧 × 14px，hover 变 --rose
锁定态：--amber 描边 + 锁图标 + 文字 --amber-2，不可点击
```

### 6.11 弹窗 `.modal`

```
遮罩：rgba(0,0,0,.62) + backdrop-filter: blur(2px)
面板：宽 420–560px / --r-2xl（16px）/ --card 底 / --border 描边 / --shadow-pop
结构：标题 15px/600 → 正文 11px --t2 → 底部右对齐按钮组（危险操作左置，主操作右置）
```

### 6.12 空态与加载

- 空态：垂直居中，16px 图标（`--t5`）+ 11px 说明（`--t3`）+ 可选次级按钮。**无插画**。
- 加载：骨架块（`--line2` 底 + 圆角 6px，1.2s 呼吸透明度 1→.55），不用 spinner。
- 错误：行内替换内容，文字 `--rose` + 重试文字按钮。

---

## 7. 图标

- 一律 **inline SVG**，不引图标库。
- 尺寸：12 / 14 / 15 / 18 / 20px，其中导航 15px、按钮内 14px、表内 12px。
- 描边风格：`fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"`；实心图标（Logo、对勾）除外。
- 颜色跟随父级 `currentColor`，不要在 SVG 上写死 `stroke="#fff"`。
- viewBox 统一 `0 0 24 24`（或 16/20 等比），保证缩放清晰。

---

## 8. 数据可视化

**区域折线（吞吐/延迟趋势）**

```
容器：flex: 1 1 auto; min-height: 0;  position: relative;
SVG：width/height 100%，preserveAspectRatio="none" 慎用（会让描边变形）→ 用 viewBox 固定宽高比
网格：仅 2–4 条横向线，--grid
主序列：--accent，stroke-width 2，linecap round
面积：linearGradient 从 rgba(91,207,197,.18) 到 transparent
参考线：--accent-deep，dasharray 3 3
轴标签：9–10px mono --t4，只标 3–5 个刻度
```
- 不要画 Y 轴竖线、不画外框、不加 tooltip 阴影。
- 图例放右上：6px 圆点 + 9px 文字 `--t3`，`gap 5px`。
- 曲线用 Catmull-Rom 转贝塞尔生成平滑路径，不要折线锯齿。

**资源条、环形进度**：见 6.9；环形只用 2px 描边 + `--t5` 底环。

---

## 9. 状态与交互

| 状态 | 表现 |
|---|---|
| hover | 底色提亮 1 级（`--card` → `--card-hover`）或描边 `--border-strong`，120ms |
| active | `translateY(0.5px)`，无颜色变化 |
| focus | `box-shadow: 0 0 0 2px rgba(91,207,197,.28)`，不出现浏览器默认 outline |
| disabled | `opacity .45`，保持原色，`cursor: not-allowed` |
| loading | 骨架呼吸 或 按钮内文字换 3 点脉冲；按钮保持原宽度 |
| 危险二次确认 | 必须弹窗，标题说明后果，按钮文案写动词（"确认吊销"） |

**动效约束**：只过渡 `background` `border-color` `color` `opacity` `transform`。时长 120–150ms，缓动 `cubic-bezier(.4,0,.2,1)`。**禁止**入场动画、位移超过 2px、`scale` 变换、超过 240ms 的过渡。

---

## 10. 文案规范

### 10.1 语气

技术同事之间说话：短、具体、给数字。不卖萌、不用感叹号、不写"让我们一起"。

### 10.2 硬规则

- **按钮 = 动词短语**：`保存并启用` `支付 ¥102.40 并生成密钥` `验证并激活` `一键导入`。禁止"确定/提交/下一步"这种无信息量的词（步骤条导航除外）。
- **数字带单位且用半角**：`28 ms` `0.4%` `12.6 GB` `0.7 / 1 Gbps`。范围用 ` / ` 分隔。不要 `28毫秒`。
- **金额**：`¥102.40`（保留两位，符号紧贴数字）。
- **时间**：`2026-11-04 19:34`（24 小时制，零填充）。相对时间用于列表：`3 分钟前`。
- **状态词固定枚举**：`在线` `离线` `警告` `已吊销` `即将到期` `已过期`。不要同义替换。
- **空态**：说明"为什么空"+"能做什么"，一句话，例：`还没有转发规则，新建一条把指定进程的流量指向节点。`
- **危险提示**：说清不可逆后果，例：`密钥吊销后立即失效，已绑定设备会掉线，且无法恢复。`
- **每屏标题唯一**：页面标题 15px，区块标题 13px，不要同级出现两个 600 字重标题。

### 10.3 中英混排

中文与英文/数字之间加一个空格（`HK-01 节点` `0.4% 丢包`）；专有名词保留原大小写（`Gbps` `Chrome` `Docker`）；进程名一律小写带扩展名（`chrome.exe`）。

---

## 11. 无障碍

- 正文与背景对比度 ≥ 4.5:1（`--t2` on `--card` 达标；`--t3` 仅用于 10px 辅助信息）。
- 所有可点击元素有 `focus-visible` 环，键盘可达。
- 状态不单靠颜色：圆点 + 文字标签同现。
- 图标按钮必须有 `aria-label` 或 `title`。
- 表格用语义标签 `<table>` 或给行加 `role="row"`。
- 触控目标最小 32×32px（图标按钮加 padding 补足）。

---

## 12. 新页面脚手架

复制以下骨架再改内容，不要从空白开始。

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>页面名 · NodeLink</title>
  <link rel="stylesheet" href="../design-system/tokens.css" />
  <link rel="stylesheet" href="../app/styles.css" />
</head>
<body>
  <div class="win">
    <!-- 客户端外壳：rail + cmain；服务端外壳：sidebar + smain -->
    <aside class="rail">
      <div class="logo-tile">…</div>
      <nav class="rail-nav">
        <button class="nav-item on" aria-label="概览">…svg…</button>
      </nav>
    </aside>

    <main class="cmain">
      <header class="titlebar">
        <span class="page-title">页面标题</span>
        <span class="spacer"></span>
        <div class="status-pill"><i class="dot green"></i>在线</div>
      </header>

      <section class="content">
        <!-- 区块标题 -->
        <div class="sec-head">
          <span class="sec-title">区块标题</span>
          <span class="sec-count">6</span>
        </div>

        <!-- KPI -->
        <div class="kpi-row">
          <div class="kpi">
            <span class="k">指标名</span>
            <div class="vrow"><span class="v num">28</span><span class="u">ms</span></div>
          </div>
        </div>

        <!-- 表格 -->
        <div class="table">
          <div class="t-head">
            <div class="cell" style="flex:1">列名</div>
            <div class="cell" style="flex:0 0 120px">状态</div>
          </div>
          <div class="t-row">
            <div class="cell" style="flex:1">主文本</div>
            <div class="cell" style="flex:0 0 120px">
              <span class="status-pill"><i class="dot green"></i>在线</span>
            </div>
          </div>
        </div>
      </section>
    </main>
  </div>
</body>
</html>
```

对应的最小局部样式（只写差异，其余继承 tokens / styles.css）：

```css
.content {
  flex: 1 1 auto; min-height: 0;
  padding: var(--w-content-pad);
  display: flex; flex-direction: column; gap: 14px;
  overflow-y: auto;
}
```

---

## 13. 交付自检清单

生成页面后逐条核对，全通过才交付：

**色彩**
- [ ] 全文没有裸色值，全部来自 tokens
- [ ] 强调色面积 < 5%，只出现在主按钮 / 选中态 / 关键正向数值
- [ ] 语义色只用于状态，没有拿 `--rose` 做装饰
- [ ] 深色底上没有 `--t3` 以下的正文

**排版**
- [ ] 正文字号 11px，标题 13px，页面标题 15px，没有 14/16px
- [ ] 所有数字、IP、密钥、进程名用等宽字体 + tabular-nums
- [ ] 字重只有 400/500/600
- [ ] 中文没有加负字距

**尺寸**
- [ ] 按钮只有 26/32/42 三档高度
- [ ] 表格行 60px，表头 40px
- [ ] 卡片圆角 12px，KPI 卡 10px
- [ ] 所有间距落在 4px 阶梯上
- [ ] 主区有 `min-width: 0`，1440 宽下无横向滚动

**结构**
- [ ] 用的是三种外壳之一，没有自创骨架
- [ ] 右侧栏固定 `flex: 0 0 Npx`，主区 `flex: 1`
- [ ] 每屏只有一个主按钮
- [ ] 表格下方留白 < 40px，没有大片空洞
- [ ] 没有元素重叠、文字截断（长文本用 `.ellipsis`）

**交互**
- [ ] hover / focus / disabled / loading 四态都有
- [ ] 动效只过渡颜色与描边，≤150ms
- [ ] 危险操作有二次确认弹窗
- [ ] 空态是一行说明 + 次级按钮，没有插画
- [ ] 图标按钮有 `aria-label`

**文案**
- [ ] 按钮都是动词短语，没有"确定/提交"
- [ ] 数字带单位、半角、单空格分隔
- [ ] 状态词来自固定枚举
- [ ] 中英文之间有空格

---

## 附：参考实现索引

| 屏幕 | 路由 | 关键结构 |
|---|---|---|
| 客户端概览 | `#/client` | rail 外壳 · 当前节点大卡（左信息+右吞吐图）· 4 KPI · 命中芯片条 · 3×2 节点网格 |
| 客户端规则 | `#/rules` | 左表格 + 右侧 400px 四步新建面板（进程单选 / 服务器选择 / 分段控件 / 保存） |
| 服务端节点 | `#/server` | sidebar 外壳 · 面包屑 · 4 KPI · 筛选栏 · 分组表格 · 底部状态栏 |
| 节点详情 | `#/node` | 左图表 320px + 接入客户端表 · 右 340px 信息卡 + 健康检查 + 资源条 |
| 开通套餐 | `#/checkout` | 流程外壳 · 步骤条 · 三档套餐 · 节点芯片网格 · 右 400px 订单摘要（吸底按钮） |
| 密钥结果 | `#/key` | 成功标题行 · 大密钥框 + 复制 · 元信息双列 · 绑定芯片 · 右导入卡（二维码）· 琥珀警示条 |
| 客户端激活 | `#/activate` | 左 560px 品牌面板 + 右 42px 密钥输入 · 验证通过状态条 · 底部引导 |
| 密钥管理 | `#/keys` | sidebar 外壳 · 4 KPI · 4 段筛选 · 8 列表格（5 行）· 分页 · 三张生命周期卡 |
