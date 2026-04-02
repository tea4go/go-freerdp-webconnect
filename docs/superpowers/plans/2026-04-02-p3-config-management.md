# P3 配置管理高级功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为连接配置管理新增搜索/过滤、排序切换、分组管理、导入/导出四项功能。

**Architecture:** 所有修改集中在 `frontend/src/views/ConnectForm.vue` 单文件中。数据层（`searchQuery`、`sortBy`、`filterGroup`、`configGroup`、`allGroups`、`filteredConfigs`）已存在于脚本中，本次主要补充 UI 模板和导入/导出方法。模板中配置卡片列表从 `savedConfigs` 切换为 `filteredConfigs`。

**Tech Stack:** Vue 3 (Composition API, `<script setup>`)、TypeScript、localStorage、原生 File API

---

### Task 1: 改造配置列表 header 为工具栏布局

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue:7` (template - config-list-header)
- Modify: `frontend/src/views/ConnectForm.vue:636-651` (style - .config-list-header)

- [ ] **Step 1: 修改 header 模板为 flex 布局 + 操作按钮区**

将第 7 行：
```html
<div class="config-list-header">已保存的配置</div>
```

替换为：
```html
<div class="config-list-header">
  <span class="config-list-title">已保存的配置</span>
  <div class="config-toolbar">
    <select
      v-if="allGroups.length"
      v-model="filterGroup"
      class="toolbar-select"
    >
      <option value="">全部</option>
      <option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option>
    </select>
    <button type="button" class="toolbar-btn" @click="sortBy = sortBy === 'time' ? 'name' : 'time'">
      {{ sortBy === 'time' ? '按时间' : '按名称' }}
    </button>
    <button type="button" class="toolbar-btn" @click="triggerImport">导入</button>
    <button type="button" class="toolbar-btn" @click="handleExportConfigs">导出</button>
  </div>
</div>
```

- [ ] **Step 2: 更新 header 样式为 flex 布局**

将 `.config-list-header` 样式替换为：
```css
.config-list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
}

.config-list-title {
  font-size: 13px;
  font-weight: 600;
  color: #475569;
}

.config-toolbar {
  display: flex;
  gap: 4px;
  align-items: center;
}

.toolbar-btn {
  padding: 3px 8px;
  font-size: 11px;
  border-radius: 5px;
  border: 1px solid #cbd5e1;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.toolbar-btn:hover {
  background: #f1f5f9;
  border-color: #94a3b8;
}

.toolbar-select {
  padding: 3px 6px;
  font-size: 11px;
  border: 1px solid #cbd5e1;
  border-radius: 5px;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  min-width: 50px;
  appearance: auto;
  -webkit-appearance: auto;
  -moz-appearance: auto;
  background-image: none;
  padding-right: 6px;
}
```

- [ ] **Step 3: 验证 header 渲染**

在浏览器中运行 `npm run dev`，确认：
- header 左侧显示"已保存的配置"标题
- 右侧显示"按时间"排序按钮、"导入"和"导出"按钮
- 无配置有分组时，分组下拉不显示
- 按钮样式与整体风格一致

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/ConnectForm.vue
git commit -m "feat: add toolbar layout to config list header with sort/import/export buttons"
```

---

### Task 2: 添加搜索栏并切换配置列表为 filteredConfigs

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue:8-20` (template - config list body)

- [ ] **Step 1: 在 header 与列表体之间添加搜索栏，并将 v-for 切换为 filteredConfigs**

将第 8-20 行的配置列表区域替换为：
```html
<div class="config-search" v-if="savedConfigs.length >= 3">
  <input
    v-model="searchQuery"
    type="text"
    class="config-search-input"
    placeholder="搜索配置..."
  />
</div>
<div v-if="savedConfigs.length" class="config-list-body">
  <div v-if="filteredConfigs.length === 0" class="config-empty">
    没有匹配的配置
  </div>
  <div v-for="cfg in filteredConfigs" :key="cfg.id" class="config-card" :class="{ editing: editingConfigId === cfg.id }" @click="handleEditConfig(cfg)">
    <div class="config-info">
      <span class="config-name">{{ cfg.name }}</span>
      <span class="config-host">{{ cfg.host }}:{{ cfg.port }}</span>
      <span v-if="cfg.group" class="config-group-tag">{{ cfg.group }}</span>
    </div>
    <div class="config-actions">
      <button type="button" class="config-connect-btn" @click.stop="handleQuickConnect(cfg)">连接</button>
      <button type="button" class="config-delete-btn" @click.stop="handleDeleteConfig(cfg.id)">删除</button>
    </div>
  </div>
</div>
<div v-else class="config-empty">暂无保存的配置，填写连接信息后可保存为快捷配置</div>
```

- [ ] **Step 2: 添加搜索栏和分组标签样式**

在 `<style scoped>` 中添加：
```css
.config-search {
  border-bottom: 1px solid #e2e8f0;
}

.config-search-input {
  width: 100%;
  padding: 7px 12px;
  font-size: 12px;
  border: none;
  background: #f8fafc;
  outline: none;
  color: #334155;
  box-sizing: border-box;
}

.config-search-input::placeholder {
  color: #94a3b8;
}

.config-search-input:focus {
  background: #ffffff;
}

.config-group-tag {
  display: inline-block;
  padding: 1px 6px;
  font-size: 11px;
  border-radius: 4px;
  background: #e2e8f0;
  color: #475569;
  margin-left: 6px;
}
```

- [ ] **Step 3: 验证搜索和过滤功能**

在浏览器中确认：
- 配置 >= 3 条时搜索框显示
- 输入关键字实时过滤配置（按名称、主机、分组匹配）
- 无匹配时显示"没有匹配的配置"
- 有分组的卡片显示分组标签

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/ConnectForm.vue
git commit -m "feat: add search bar and switch config list to use filteredConfigs"
```

---

### Task 3: 保存配置时支持分组输入

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue:108-111` (template - save-config)
- Modify: `frontend/src/views/ConnectForm.vue:344-363` (script - handleEditConfig)

- [ ] **Step 1: 在保存区域添加分组输入框（带 datalist）**

将第 108-111 行的保存区域替换为：
```html
<div class="save-config">
  <input v-model="configName" type="text" placeholder="配置名称（可选）" maxlength="50" />
  <input v-model="configGroup" type="text" placeholder="分组（可选）" maxlength="30" list="group-list" class="save-group-input" />
  <datalist id="group-list">
    <option v-for="g in allGroups" :key="g" :value="g" />
  </datalist>
  <button type="button" class="save-btn" :disabled="!configName.trim()" @click="handleSaveConfig">保存配置</button>
</div>
```

- [ ] **Step 2: 在 handleEditConfig 中回填 configGroup**

在 `handleEditConfig` 函数中（约第 360 行），在 `configName.value = cfg.name` 之后添加：
```typescript
configGroup.value = cfg.group || ''
```

- [ ] **Step 3: 在 handleSaveConfig 成功后清空 configGroup**

检查 `handleSaveConfig` 函数中所有 return 路径，确保每个路径都有 `configGroup.value = ''`。

查看现有代码，编辑模式中第 303 行已有 `configGroup.value = ''`，同名覆盖中第 315 行也有。新配置创建后第 328 行需确认也有清空操作。如果没有，在 `configName.value = ''` 之后添加 `configGroup.value = ''`。

- [ ] **Step 4: 添加分组输入框样式**

在 `<style scoped>` 中添加：
```css
.save-group-input {
  width: 100px;
  flex-shrink: 0;
}
```

同时更新 `.save-config` 使三个元素合理分布，将现有的 `.save-config input` 规则中的 `flex: 1` 改为只应用到第一个 input：
```css
.save-config input:first-child {
  flex: 1;
}
```

- [ ] **Step 5: 验证分组功能**

在浏览器中确认：
- 保存区域显示"配置名称"和"分组"两个输入框 + 保存按钮
- 输入分组名后保存，分组信息被保存
- 点击已有分组的配置卡片，分组输入框回填正确
- datalist 下拉显示已有分组名

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/ConnectForm.vue
git commit -m "feat: add group input to config save area with datalist suggestions"
```

---

### Task 4: 实现导出功能

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue` (script - 新增 handleExportConfigs 方法)

- [ ] **Step 1: 添加 handleExportConfigs 方法**

在脚本区域（`handleDeleteConfig` 函数之后）添加：
```typescript
// P3: 导出所有配置为 JSON 文件
function handleExportConfigs() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) {
    alert('没有可导出的配置')
    return
  }

  const blob = new Blob([raw], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
  const a = document.createElement('a')
  a.href = url
  a.download = `rdp-configs-${date}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
```

- [ ] **Step 2: 验证导出功能**

在浏览器中确认：
- 有配置时点击导出 → 下载名为 `rdp-configs-YYYYMMDD.json` 的文件
- 文件内容为 JSON 数组，密码字段是混淆编码（非明文）
- 无配置时点击导出 → 弹出提示"没有可导出的配置"

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/ConnectForm.vue
git commit -m "feat: implement config export to JSON file"
```

---

### Task 5: 实现导入功能

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue` (template - 新增隐藏 file input)
- Modify: `frontend/src/views/ConnectForm.vue` (script - 新增 triggerImport / handleImportConfigs)

- [ ] **Step 1: 在模板末尾添加隐藏 file input**

在 `</form>` 标签之后、`<div class="version">` 之前添加：
```html
<input
  ref="importFileInput"
  type="file"
  accept=".json"
  style="display: none"
  @change="handleImportConfigs"
/>
```

- [ ] **Step 2: 添加 importFileInput ref**

在脚本区域的 ref 声明区（约第 213 行 `editingConfigId` 之后）添加：
```typescript
const importFileInput = ref<HTMLInputElement | null>(null)
```

- [ ] **Step 3: 添加 triggerImport 方法**

在 `handleExportConfigs` 之后添加：
```typescript
// P3: 触发文件选择
function triggerImport() {
  importFileInput.value?.click()
}
```

- [ ] **Step 4: 添加 handleImportConfigs 方法**

在 `triggerImport` 之后添加：
```typescript
// P3: 导入配置
function handleImportConfigs(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = () => {
    try {
      const data = JSON.parse(reader.result as string)
      if (!Array.isArray(data)) {
        alert('导入失败：文件格式无效，需要 JSON 数组')
        return
      }

      const validFields: (keyof SavedConfig)[] = [
        'id', 'name', 'group', 'host', 'port', 'user', 'pass',
        'resolution', 'perf', 'fntlm', 'nowallp', 'nowdrag',
        'nomani', 'notheme', 'nonla', 'notls', 'createdAt'
      ]

      let imported = 0
      let skipped = 0

      for (const item of data) {
        // 校验必填字段
        if (typeof item.name !== 'string' || !item.name.trim()) { skipped++; continue }
        if (typeof item.host !== 'string' || !item.host.trim()) { skipped++; continue }

        // 同名跳过
        if (savedConfigs.value.some(c => c.name === item.name)) { skipped++; continue }

        // 只保留已知字段，构建安全的配置对象
        const cfg: SavedConfig = {
          id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
          name: String(item.name).trim(),
          group: typeof item.group === 'string' ? item.group.trim() : '',
          host: String(item.host).trim(),
          port: typeof item.port === 'number' ? item.port : 3389,
          user: typeof item.user === 'string' ? item.user : '',
          pass: typeof item.pass === 'string' ? item.pass : '',
          resolution: typeof item.resolution === 'string' ? item.resolution : '1024x768',
          perf: typeof item.perf === 'number' ? item.perf : 0,
          fntlm: typeof item.fntlm === 'number' ? item.fntlm : 0,
          nowallp: typeof item.nowallp === 'boolean' ? item.nowallp : false,
          nowdrag: typeof item.nowdrag === 'boolean' ? item.nowdrag : false,
          nomani: typeof item.nomani === 'boolean' ? item.nomani : false,
          notheme: typeof item.notheme === 'boolean' ? item.notheme : false,
          nonla: typeof item.nonla === 'boolean' ? item.nonla : false,
          notls: typeof item.notls === 'boolean' ? item.notls : false,
          createdAt: typeof item.createdAt === 'number' ? item.createdAt : Date.now(),
        }

        savedConfigs.value.push(cfg)
        imported++
      }

      persistConfigs(savedConfigs.value)
      alert(`成功导入 ${imported} 条配置，跳过 ${skipped} 条`)
    } catch {
      alert('导入失败：文件内容不是有效的 JSON')
    }
  }

  reader.readAsText(file)
  input.value = '' // 重置，允许再次选择同一文件
}
```

- [ ] **Step 5: 验证导入功能**

在浏览器中确认：
- 点击"导入"按钮弹出文件选择框
- 选择之前导出的 JSON 文件后成功导入
- 同名配置被跳过，弹窗显示导入/跳过数量
- 无效 JSON 文件提示错误
- 非数组 JSON 提示格式无效
- 导入后配置列表自动更新

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/ConnectForm.vue
git commit -m "feat: implement config import from JSON file with merge strategy"
```

---

### Task 6: 移动端适配 + 最终清理

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue` (style - @media 区域)

- [ ] **Step 1: 添加工具栏和保存区域的移动端样式**

在现有 `@media (max-width: 520px)` 规则块中追加：
```css
.config-list-header {
  flex-wrap: wrap;
  gap: 6px;
}
.config-toolbar {
  flex-wrap: wrap;
  gap: 3px;
}
.save-config {
  flex-wrap: wrap;
}
.save-group-input {
  width: 100% !important;
  order: 3;
}
```

- [ ] **Step 2: 删除脚本中未使用的 validFields 变量警告**

检查 `handleImportConfigs` 中 `validFields` 变量是否实际使用。如果未使用（仅作为注释用途），删除该声明以避免 TypeScript 警告。

- [ ] **Step 3: 更新 requirements.md 标记 P3 任务完成**

将 `requirements.md` 第 183-186 行的 P3 待办项从 `- [ ]` 改为 `- [x]`：
```markdown
- [x] 配置导入/导出（JSON 文件）
- [x] 配置分组管理
- [x] 配置搜索/过滤
- [x] 配置排序
```

- [ ] **Step 4: 运行 TypeScript 编译检查**

```bash
cd frontend && npx vue-tsc --noEmit
```

Expected: 无类型错误

- [ ] **Step 5: 全面验证**

在浏览器中完整验证所有 P3 功能：
1. 搜索：输入关键字过滤配置，清空恢复
2. 排序：切换"按时间"/"按名称"
3. 分组：保存时指定分组 → 卡片显示分组标签 → 下拉过滤按分组
4. 导出：下载 JSON 文件
5. 导入：选择 JSON 文件导入，同名跳过
6. 编辑：点击卡片回填分组信息
7. 移动端：缩小窗口，工具栏和保存区域正确换行

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/ConnectForm.vue requirements.md
git commit -m "feat: complete P3 config management - responsive layout and cleanup"
```
