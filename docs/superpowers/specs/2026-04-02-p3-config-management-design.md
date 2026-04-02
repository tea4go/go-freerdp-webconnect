# P3 配置管理高级功能设计

## 概述

为 QRDP 的连接配置管理新增 4 个功能：配置导入/导出、分组管理、搜索/过滤、排序。目标用户配置数量 < 20，UI 保持简洁紧凑。

## 前提条件

- 所有修改集中在 `frontend/src/views/ConnectForm.vue` 单文件中
- 代码中已有 `searchQuery`、`sortBy`、`filterGroup`、`configGroup`、`allGroups`、`filteredConfigs` 等 P3 数据层代码，需要补充 UI 层
- `SavedConfig` 接口已包含 `group` 字段
- 延续现有设计风格（圆角、阴影、红色主题色 #cf0a2c）

## 1. 配置列表头部工具栏

### 布局

```
┌─────────────────────────────────────────────┐
│ 已保存的配置  [全部 ▾] [↕名称] [↓导入] [↑导出] │  ← header 行
├─────────────────────────────────────────────┤
│ 🔍 搜索配置名称或主机地址...                    │  ← 搜索栏（配置 >= 3 条时显示）
├─────────────────────────────────────────────┤
│ [配置卡片列表...]                             │
└─────────────────────────────────────────────┘
```

### 具体说明

- **header 行**：改造现有 `.config-list-header`，使用 `display: flex; justify-content: space-between; align-items: center`
  - 左侧：标题文字"已保存的配置"
  - 右侧：操作按钮组（分组下拉、排序按钮、导入按钮、导出按钮）
- **搜索栏**：header 和 config-list-body 之间，仅当 `savedConfigs.length >= 3` 时渲染
- 配置列表中使用 `filteredConfigs` 替代 `savedConfigs` 渲染卡片

## 2. 搜索/过滤

### 搜索

- `input` 框，placeholder "搜索配置..."
- 实时过滤，v-model 绑定到已有的 `searchQuery`
- 匹配字段：`name`、`host`、`group`（已在 `filteredConfigs` computed 中实现）

### 分组过滤

- `select` 下拉框，放在 header 右侧操作区
- 默认选项 "全部"（值为空字符串）
- 选项列表从 `allGroups` computed 动态生成
- 如果 `allGroups` 为空，隐藏该下拉
- v-model 绑定到已有的 `filterGroup`

## 3. 排序

- 按钮切换两种排序方式：
  - 按时间排序（默认）：最近创建的在前
  - 按名称排序：字母顺序
- 点击按钮在两种排序间切换
- 按钮文字反映当前排序方式（如 "按时间" / "按名称"）
- v-model 逻辑绑定到已有的 `sortBy`

## 4. 分组管理

### 保存时指定分组

- 保存配置区域 `.save-config` 中，在配置名称输入框旁增加分组输入框
- 使用 `<input>` + `<datalist>` 实现输入新分组名或选择已有分组的混合体验
- placeholder: "分组（可选）"
- v-model 绑定到已有的 `configGroup`
- 保存成功后清空 `configGroup`

### 编辑时回填分组

- `handleEditConfig` 中回填 `configGroup` 为 `cfg.group`

### 卡片显示分组

- 配置卡片的 `.config-host` 后面显示分组标签（如有）
- 样式：小圆角背景标签，浅色背景，12px 字号

## 5. 导入/导出

### 导出

- 点击导出按钮：
  1. 从 localStorage 直接读取原始数据（密码保持混淆编码）
  2. 序列化为格式化 JSON
  3. 创建 Blob → URL.createObjectURL → 动态 `<a>` 标签触发下载
  4. 文件名：`rdp-configs-YYYYMMDD.json`

### 导入

- 点击导入按钮 → 触发隐藏 `<input type="file" accept=".json">`
- 读取文件，校验格式：
  - 必须是数组
  - 每个元素必须有 `name`（非空字符串）和 `host`（非空字符串）字段
  - 无效条目跳过
- 合并策略：同名配置跳过（不覆盖），新配置追加
- 导入完成后 `alert("成功导入 N 条配置，跳过 M 条同名配置")`
- 安全性：只读取已知的 `SavedConfig` 字段，忽略未知字段；对字段做类型校验

## 6. 样式规范

### 工具栏按钮（通用）

```css
padding: 4px 8px;
font-size: 12px;
border-radius: 6px;
border: 1px solid #cbd5e1;
background: transparent;
color: #64748b;
cursor: pointer;
transition: all 0.2s ease;
```

### 工具栏按钮悬停

```css
background: #f1f5f9;
border-color: #94a3b8;
```

### 搜索框

```css
width: 100%;
padding: 6px 10px;
font-size: 12px;
border: none;
border-top: 1px solid #e2e8f0;
background: #f8fafc;
outline: none;
```

### 分组标签

```css
display: inline-block;
padding: 1px 6px;
font-size: 11px;
border-radius: 4px;
background: #e2e8f0;
color: #475569;
margin-left: 6px;
```

### 分组下拉（header 内）

```css
padding: 4px 6px;
font-size: 12px;
border: 1px solid #cbd5e1;
border-radius: 6px;
background: transparent;
min-width: 60px;
```

## 7. 移动端适配

- 工具栏按钮在窄屏（< 520px）时使用换行或缩小间距
- 搜索框保持全宽
- 保存区域的分组输入框在窄屏时换行到新行

## 8. 修改范围

仅修改 `frontend/src/views/ConnectForm.vue`：
- **模板**：改造 config-list-header、新增搜索栏、保存区增加分组输入、卡片显示分组标签、新增隐藏 file input
- **脚本**：新增 `handleExportConfigs`、`handleImportConfigs` 方法；`handleEditConfig` 增加分组回填；`handleSaveConfig` 中 `configGroup` 已有处理
- **样式**：新增工具栏、搜索框、分组标签等样式
