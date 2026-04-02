<template>
  <div class="app-layout">
    <!-- 标题栏 -->
    <header class="title-bar">
      <div class="title-bar-left">
        <span class="app-name">FreeRDP WebConnect</span>
        <span class="app-version" v-if="version">v{{ version.app }}</span>
      </div>
      <div class="title-bar-right">
        <label class="group-label" v-if="allGroups.length">分组</label>
        <select v-if="allGroups.length" v-model="filterGroup" class="group-filter">
          <option value="">全部</option>
          <option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option>
        </select>
      </div>
    </header>

    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <input
          v-model="searchQuery"
          type="text"
          class="search-input"
          placeholder="搜索配置..."
        />
      </div>
      <div class="toolbar-right">
        <button type="button" class="tool-btn" @click="sortBy = sortBy === 'time' ? 'name' : 'time'">
          {{ sortBy === 'time' ? '按时间' : '按名称' }}
        </button>
        <button type="button" class="tool-btn" @click="triggerImport">导入</button>
        <button type="button" class="tool-btn" @click="handleExportConfigs">导出</button>
        <button type="button" class="tool-btn tool-btn-primary" @click="handleNewConfig">新建连接</button>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 左侧：配置表格 -->
      <div class="config-table-area">
        <table class="config-table" v-if="filteredConfigs.length">
          <thead>
            <tr>
              <th class="col-name">名称</th>
              <th class="col-host">地址</th>
              <th class="col-user">用户</th>
              <th class="col-date">创建时间</th>
              <th class="col-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="cfg in filteredConfigs"
              :key="cfg.id"
              :class="{ selected: editingConfigId === cfg.id }"
              @click="handleEditConfig(cfg)"
            >
              <td class="col-name">
                <span class="cfg-name">{{ cfg.name }}</span>
                <span v-if="cfg.group" class="cfg-group-tag">{{ cfg.group }}</span>
              </td>
              <td class="col-host">{{ cfg.host }}:{{ cfg.port }}</td>
              <td class="col-user">{{ cfg.user || '-' }}</td>
              <td class="col-date">{{ formatDate(cfg.createdAt) }}</td>
              <td class="col-actions">
                <button type="button" class="action-connect" @click.stop="handleQuickConnect(cfg)">连接</button>
                <button type="button" class="action-delete" @click.stop="handleDeleteConfig(cfg.id)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty-state">
          <template v-if="savedConfigs.length && !filteredConfigs.length">没有匹配的配置</template>
          <template v-else>暂无保存的配置，点击"新建连接"开始</template>
        </div>
      </div>

      <!-- 右侧：表单侧栏 -->
      <aside class="form-panel" v-if="showFormPanel">
        <div class="form-panel-header">
          <span>{{ editingConfigId ? '编辑配置' : '新建连接' }}</span>
          <button type="button" class="panel-close-btn" @click="closeFormPanel">&times;</button>
        </div>
        <div class="form-panel-body">
          <form @submit.prevent="handleConnect">
            <div class="tabs">
              <button
                type="button"
                class="tab-btn"
                :class="{ active: activeTab === 'basic' }"
                @click="activeTab = 'basic'"
              >基础参数</button>
              <button
                type="button"
                class="tab-btn"
                :class="{ active: activeTab === 'advanced' }"
                @click="activeTab = 'advanced'"
              >高级参数</button>
            </div>

            <div v-show="activeTab === 'basic'" class="tab-panel">
              <div class="form-group">
                <label for="host">主机地址</label>
                <input id="host" v-model="form.host" type="text" placeholder="192.168.1.100" required />
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label for="port">端口</label>
                  <input id="port" v-model.number="form.port" type="number" />
                </div>
                <div class="form-group">
                  <label for="resolution">分辨率</label>
                  <select id="resolution" v-model="form.resolution">
                    <option value="1024x768">1024 x 768</option>
                    <option value="1280x720">1280 x 720</option>
                    <option value="1280x800">1280 x 800</option>
                    <option value="1366x768">1366 x 768</option>
                    <option value="1440x900">1440 x 900</option>
                    <option value="1920x1080">1920 x 1080</option>
                  </select>
                </div>
              </div>
              <div class="form-group">
                <label for="user">用户名</label>
                <input id="user" v-model="form.user" type="text" placeholder="administrator" />
              </div>
              <div class="form-group">
                <label for="pass">密码</label>
                <input id="pass" v-model="form.pass" type="password" />
              </div>
            </div>

            <div v-show="activeTab === 'advanced'" class="tab-panel">
              <div class="form-group">
                <label for="perf">性能</label>
                <select id="perf" v-model.number="form.perf">
                  <option :value="0">局域网</option>
                  <option :value="1">宽带</option>
                  <option :value="2">调制解调器</option>
                </select>
              </div>
              <div class="form-group">
                <label for="fntlm">强制 NTLM 认证</label>
                <select id="fntlm" v-model.number="form.fntlm">
                  <option :value="0">禁用</option>
                  <option :value="1">NTLM v1</option>
                  <option :value="2">NTLM v2</option>
                </select>
              </div>
              <div class="check-grid">
                <label class="check-item"><input v-model="form.nowallp" type="checkbox" /> 禁用壁纸</label>
                <label class="check-item"><input v-model="form.nowdrag" type="checkbox" /> 禁用窗口全拖动</label>
                <label class="check-item"><input v-model="form.nomani" type="checkbox" /> 禁用菜单动画</label>
                <label class="check-item"><input v-model="form.notheme" type="checkbox" /> 禁用主题</label>
                <label class="check-item"><input v-model="form.nonla" type="checkbox" :disabled="form.notls" /> 禁用 NLA</label>
                <label class="check-item"><input v-model="form.notls" type="checkbox" /> 禁用 TLS</label>
              </div>
            </div>

            <div class="save-config">
              <input v-model="configName" type="text" placeholder="配置名称" maxlength="50" />
              <input v-model="configGroup" type="text" placeholder="分组" maxlength="30" list="group-list" class="save-group-input" />
              <datalist id="group-list">
                <option v-for="g in allGroups" :key="g" :value="g" />
              </datalist>
              <button type="button" class="save-btn" :disabled="!configName.trim()" @click="handleSaveConfig">保存</button>
            </div>

            <button type="submit" class="submit-btn" :disabled="connecting">
              {{ connecting ? '连接中...' : '连接' }}
            </button>
            <p v-if="error" class="error">{{ error }}</p>
          </form>
        </div>
      </aside>
    </div>

    <!-- 状态栏 -->
    <footer class="status-bar">
      <span v-if="version">App {{ version.app }} | FreeRDP {{ version.freerdp }}</span>
      <span>共 {{ savedConfigs.length }} 个配置</span>
    </footer>

    <!-- 自定义确认对话框 -->
    <div v-if="confirmDialog.visible" class="confirm-overlay" @click.self="resolveConfirm(false)">
      <div class="confirm-box">
        <p class="confirm-msg">{{ confirmDialog.message }}</p>
        <div class="confirm-actions">
          <button type="button" class="confirm-btn confirm-cancel" @click="resolveConfirm(false)">取消</button>
          <button type="button" class="confirm-btn confirm-ok" @click="resolveConfirm(true)">确定</button>
        </div>
      </div>
    </div>

    <!-- 隐藏的导入文件 input -->
    <input
      ref="importFileInput"
      type="file"
      accept=".json"
      style="display: none"
      @change="handleImportConfigs"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { Connect, GetVersion } from '../wailsjs/go/backend/App'

interface SavedConfig {
  id: string
  name: string
  group: string
  host: string
  port: number
  user: string
  pass: string
  resolution: string
  perf: number
  fntlm: number
  nowallp: boolean
  nowdrag: boolean
  nomani: boolean
  notheme: boolean
  nonla: boolean
  notls: boolean
  createdAt: number
}

const STORAGE_KEY = 'rdp_saved_configs'
const WARN_KEY = 'rdp_password_warn_dismissed'

const OBFUSCATE_KEY = 0x5A
function obfuscatePass(plain: string): string {
  if (!plain) return ''
  const bytes = new TextEncoder().encode(plain)
  const obfuscated = bytes.map(b => b ^ OBFUSCATE_KEY)
  return btoa(String.fromCharCode(...obfuscated))
}

function deobfuscatePass(encoded: string): string {
  if (!encoded) return ''
  try {
    const decoded = atob(encoded)
    const bytes = new Uint8Array([...decoded].map(c => c.charCodeAt(0) ^ OBFUSCATE_KEY))
    return new TextDecoder().decode(bytes)
  } catch {
    return encoded
  }
}

function loadConfigs(): SavedConfig[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const configs: SavedConfig[] = JSON.parse(raw)
    return configs.map(c => ({ ...c, pass: deobfuscatePass(c.pass) }))
  } catch {
    return []
  }
}

function persistConfigs(configs: SavedConfig[]) {
  const encoded = configs.map(c => ({ ...c, pass: obfuscatePass(c.pass) }))
  localStorage.setItem(STORAGE_KEY, JSON.stringify(encoded))
}

function formatDate(ts: number): string {
  return new Date(ts).toLocaleDateString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

const emit = defineEmits<{
  connect: [wsUrl: string, width: number, height: number]
}>()

const form = reactive({
  host: '',
  port: 3389,
  user: 'administrator',
  pass: '',
  resolution: '1024x768',
  perf: 0,
  fntlm: 0,
  nowallp: false,
  nowdrag: false,
  nomani: false,
  notheme: false,
  nonla: false,
  notls: false,
})

const connecting = ref(false)
const error = ref('')
const version = ref<{ app: string; freerdp: string } | null>(null)
const activeTab = ref<'basic' | 'advanced'>('basic')
const configName = ref('')
const savedConfigs = ref<SavedConfig[]>([])
const editingConfigId = ref<string | null>(null)
const importFileInput = ref<HTMLInputElement | null>(null)
const showFormPanel = ref(false)

const confirmDialog = reactive({
  visible: false,
  message: '',
  _resolve: null as ((val: boolean) => void) | null,
})

function showConfirm(message: string): Promise<boolean> {
  return new Promise((resolve) => {
    confirmDialog.message = message
    confirmDialog._resolve = resolve
    confirmDialog.visible = true
  })
}

function resolveConfirm(val: boolean) {
  confirmDialog.visible = false
  if (confirmDialog._resolve) {
    confirmDialog._resolve(val)
    confirmDialog._resolve = null
  }
}

function showAlert(message: string): Promise<boolean> {
  return showConfirm(message)
}

const searchQuery = ref('')
const sortBy = ref<'name' | 'time'>('time')
const filterGroup = ref('')
const configGroup = ref('')

const allGroups = computed(() => {
  const groups = new Set(savedConfigs.value.map(c => c.group).filter(Boolean))
  return [...groups].sort()
})

const filteredConfigs = computed(() => {
  let list = savedConfigs.value

  if (filterGroup.value) {
    list = list.filter(c => c.group === filterGroup.value)
  }

  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase()
    list = list.filter(c =>
      c.name.toLowerCase().includes(q) ||
      c.host.toLowerCase().includes(q) ||
      (c.group && c.group.toLowerCase().includes(q))
    )
  }

  return [...list].sort((a, b) => {
    if (sortBy.value === 'name') return a.name.localeCompare(b.name)
    return b.createdAt - a.createdAt
  })
})

onMounted(async () => {
  savedConfigs.value = loadConfigs()
  try {
    version.value = await GetVersion() as { app: string; freerdp: string }
  } catch {
    // 非 Wails 环境忽略
  }
})

watch(() => form.notls, (v) => {
  if (v) {
    form.nonla = true
  }
})

function handleNewConfig() {
  Object.assign(form, {
    host: '', port: 3389, user: 'administrator', pass: '',
    resolution: '1024x768', perf: 0, fntlm: 0,
    nowallp: false, nowdrag: false, nomani: false,
    notheme: false, nonla: false, notls: false,
  })
  configName.value = ''
  configGroup.value = ''
  editingConfigId.value = null
  activeTab.value = 'basic'
  showFormPanel.value = true
}

function closeFormPanel() {
  showFormPanel.value = false
  editingConfigId.value = null
}

async function handleSaveConfig() {
  const name = configName.value.trim()
  if (!name) return

  if (form.pass && !localStorage.getItem(WARN_KEY)) {
    const ok = await showConfirm('密码将以混淆形式保存在本地浏览器中，请确保设备安全。\n\n点击"确定"继续保存，后续不再提示。')
    if (!ok) return
    localStorage.setItem(WARN_KEY, '1')
  }

  const formSnapshot = {
    host: form.host,
    port: form.port,
    user: form.user,
    pass: form.pass,
    resolution: form.resolution,
    perf: form.perf,
    fntlm: form.fntlm,
    nowallp: form.nowallp,
    nowdrag: form.nowdrag,
    nomani: form.nomani,
    notheme: form.notheme,
    nonla: form.nonla,
    notls: form.notls,
    group: configGroup.value.trim(),
  }

  if (editingConfigId.value) {
    const idx = savedConfigs.value.findIndex(c => c.id === editingConfigId.value)
    if (idx !== -1) {
      savedConfigs.value[idx] = { ...savedConfigs.value[idx], name, ...formSnapshot }
      persistConfigs(savedConfigs.value)
      configName.value = ''
      configGroup.value = ''
      editingConfigId.value = null
      return
    }
  }

  const existing = savedConfigs.value.find(c => c.name === name)
  if (existing) {
    const confirmed = await showConfirm(`配置"${name}"已存在，是否覆盖？`)
    if (!confirmed) return
    Object.assign(existing, formSnapshot)
    persistConfigs(savedConfigs.value)
    configName.value = ''
    configGroup.value = ''
    return
  }

  const cfg: SavedConfig = {
    id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
    name,
    ...formSnapshot,
    createdAt: Date.now(),
  }

  savedConfigs.value.push(cfg)
  persistConfigs(savedConfigs.value)
  configName.value = ''
}

async function handleDeleteConfig(id: string) {
  const cfg = savedConfigs.value.find(c => c.id === id)
  if (!cfg) return
  const confirmed = await showConfirm(`确定删除配置"${cfg.name}"？`)
  if (!confirmed) return
  savedConfigs.value = savedConfigs.value.filter(c => c.id !== id)
  persistConfigs(savedConfigs.value)
  if (editingConfigId.value === id) {
    editingConfigId.value = null
    configName.value = ''
    showFormPanel.value = false
  }
}

async function handleExportConfigs() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) {
    await showAlert('没有可导出的配置')
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

function triggerImport() {
  importFileInput.value?.click()
}

function handleImportConfigs(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = async () => {
    try {
      const data = JSON.parse(reader.result as string)
      if (!Array.isArray(data)) {
        await showAlert('导入失败：文件格式无效，需要 JSON 数组')
        return
      }

      let imported = 0
      let skipped = 0

      for (const item of data) {
        if (typeof item.name !== 'string' || !item.name.trim()) { skipped++; continue }
        if (typeof item.host !== 'string' || !item.host.trim()) { skipped++; continue }
        if (savedConfigs.value.some(c => c.name === item.name)) { skipped++; continue }

        const cfg: SavedConfig = {
          id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
          name: String(item.name).trim(),
          group: typeof item.group === 'string' ? item.group.trim() : '',
          host: String(item.host).trim(),
          port: typeof item.port === 'number' ? item.port : 3389,
          user: typeof item.user === 'string' ? item.user : '',
          pass: typeof item.pass === 'string' ? deobfuscatePass(item.pass) : '',
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
      await showAlert(`成功导入 ${imported} 条配置，跳过 ${skipped} 条`)
    } catch {
      await showAlert('导入失败：文件内容不是有效的 JSON')
    }
  }

  reader.readAsText(file)
  input.value = ''
}

function handleEditConfig(cfg: SavedConfig) {
  Object.assign(form, {
    host: cfg.host,
    port: cfg.port,
    user: cfg.user,
    pass: cfg.pass,
    resolution: cfg.resolution,
    perf: cfg.perf,
    fntlm: cfg.fntlm,
    nowallp: cfg.nowallp,
    nowdrag: cfg.nowdrag,
    nomani: cfg.nomani,
    notheme: cfg.notheme,
    nonla: cfg.nonla,
    notls: cfg.notls,
  })
  configName.value = cfg.name
  configGroup.value = cfg.group || ''
  editingConfigId.value = cfg.id
  activeTab.value = 'basic'
  showFormPanel.value = true
}

async function handleQuickConnect(cfg: SavedConfig) {
  Object.assign(form, {
    host: cfg.host,
    port: cfg.port,
    user: cfg.user,
    pass: cfg.pass,
    resolution: cfg.resolution,
    perf: cfg.perf,
    fntlm: cfg.fntlm,
    nowallp: cfg.nowallp,
    nowdrag: cfg.nowdrag,
    nomani: cfg.nomani,
    notheme: cfg.notheme,
    nonla: cfg.nonla,
    notls: cfg.notls,
  })
  await handleConnect()
}

async function handleConnect() {
  if (!form.host) {
    error.value = '请输入主机地址'
    return
  }

  connecting.value = true
  error.value = ''

  try {
    const [w, h] = form.resolution.split('x').map(Number)

    const wsUrl: string = await Connect(
      form.host, form.user, form.pass, form.port, w, h,
      form.perf, form.fntlm,
      form.nowallp, form.nowdrag, form.nomani, form.notheme, form.nonla, form.notls
    )

    emit('connect', wsUrl, w, h)
  } catch (e: any) {
    error.value = e.message || '连接失败'
  } finally {
    connecting.value = false
  }
}
</script>

<style scoped>
/* === 全窗口布局 === */
.app-layout {
  display: flex;
  flex-direction: column;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #f3f4f6;
}

/* === 标题栏 === */
.title-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 44px;
  padding: 0 16px;
  background: #1e293b;
  color: #f1f5f9;
  flex-shrink: 0;
  --wails-draggable: drag;
}

.title-bar-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.app-name {
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.app-version {
  font-size: 11px;
  color: #94a3b8;
}

.title-bar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  --wails-draggable: no-drag;
}

.group-label {
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
}

.group-filter {
  padding: 4px 8px;
  font-size: 12px;
  border: 1px solid #475569;
  border-radius: 4px;
  background: #334155;
  color: #e2e8f0;
  cursor: pointer;
  min-width: 80px;
  appearance: auto;
  -webkit-appearance: auto;
  -moz-appearance: auto;
  background-image: none;
}

/* === 工具栏 === */
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 42px;
  padding: 0 16px;
  background: #ffffff;
  border-bottom: 1px solid #e2e8f0;
  flex-shrink: 0;
  gap: 12px;
}

.toolbar-left {
  flex: 1;
  max-width: 280px;
}

.search-input {
  width: 100%;
  padding: 6px 10px;
  font-size: 12px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: #f9fafb;
  color: #374151;
  outline: none;
  box-sizing: border-box;
}

.search-input:focus {
  border-color: #9ca3af;
  background: #fff;
}

.search-input::placeholder {
  color: #9ca3af;
}

.toolbar-right {
  display: flex;
  gap: 6px;
  align-items: center;
}

.tool-btn {
  padding: 5px 12px;
  font-size: 12px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: #fff;
  color: #374151;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, border-color 0.15s;
}

.tool-btn:hover {
  background: #f3f4f6;
  border-color: #9ca3af;
}

.tool-btn-primary {
  background: #cf0a2c;
  color: #fff;
  border-color: #cf0a2c;
}

.tool-btn-primary:hover {
  background: #b50c2b;
  border-color: #b50c2b;
}

/* === 主内容区 === */
.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* === 配置表格 === */
.config-table-area {
  flex: 1;
  overflow-y: auto;
  background: #fff;
}

.config-table {
  width: 100%;
  border-collapse: collapse;
}

.config-table thead {
  position: sticky;
  top: 0;
  z-index: 1;
}

.config-table th {
  padding: 10px 16px;
  text-align: left;
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  white-space: nowrap;
}

.config-table td {
  padding: 10px 16px;
  font-size: 13px;
  color: #334155;
  border-bottom: 1px solid #f1f5f9;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.config-table tbody tr {
  cursor: pointer;
  transition: background 0.12s;
}

.config-table tbody tr:hover {
  background: #f1f5f9;
}

.config-table tbody tr.selected {
  background: #eff6ff;
}

.col-name {
  min-width: 120px;
}

.col-host {
  width: 180px;
}

.col-user {
  width: 120px;
}

.col-date {
  width: 160px;
}

.col-actions {
  width: 120px;
  text-align: right;
}

td.col-actions {
  text-align: right;
}

.cfg-name {
  font-weight: 600;
  color: #1e293b;
}

.cfg-group-tag {
  display: inline-block;
  padding: 1px 6px;
  font-size: 10px;
  border-radius: 3px;
  background: #e2e8f0;
  color: #475569;
  margin-left: 6px;
  vertical-align: middle;
}

.action-connect {
  padding: 3px 10px;
  font-size: 11px;
  background: #cf0a2c;
  color: #fff;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.15s;
}

.action-connect:hover {
  background: #b50c2b;
}

.action-delete {
  padding: 3px 10px;
  font-size: 11px;
  background: transparent;
  color: #64748b;
  border: 1px solid #d1d5db;
  border-radius: 3px;
  cursor: pointer;
  margin-left: 4px;
  transition: all 0.15s;
}

.action-delete:hover {
  background: #fee2e2;
  color: #dc2626;
  border-color: #fca5a5;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 200px;
  color: #94a3b8;
  font-size: 13px;
}

/* === 表单侧栏 === */
.form-panel {
  width: 360px;
  flex-shrink: 0;
  border-left: 1px solid #e2e8f0;
  background: #fafbfc;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.form-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid #e2e8f0;
  background: #f8fafc;
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.panel-close-btn {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: #64748b;
  font-size: 18px;
  cursor: pointer;
  border-radius: 4px;
  transition: background 0.15s;
}

.panel-close-btn:hover {
  background: #e2e8f0;
  color: #1e293b;
}

.form-panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.form-panel-body::-webkit-scrollbar {
  width: 6px;
}

.form-panel-body::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.4);
  border-radius: 999px;
}

/* === 表单元素 === */
.tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  margin-bottom: 14px;
}

.tab-btn {
  border: 1px solid #d1d5db;
  border-radius: 4px;
  min-height: 34px;
  background: #fff;
  color: #475569;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
}

.tab-btn:hover {
  border-color: #9ca3af;
}

.tab-btn.active {
  color: #cf0a2c;
  border-color: #cf0a2c;
  background: rgba(207, 10, 44, 0.06);
}

.tab-panel {
  margin-bottom: 12px;
}

.form-group {
  margin-bottom: 12px;
}

.form-row {
  display: flex;
  gap: 8px;
}

.form-row .form-group {
  flex: 1;
}

label {
  display: block;
  color: #6b7280;
  font-size: 12px;
  margin-bottom: 4px;
}

input, select {
  width: 100%;
  padding: 7px 10px;
  height: 34px;
  min-height: 34px;
  background: #fff;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  color: #111827;
  font-size: 13px;
  box-sizing: border-box;
  transition: border-color 0.15s;
}

input:focus, select:focus {
  outline: none;
  border-color: #cf0a2c;
}

input::placeholder {
  color: #9ca3af;
}

select {
  appearance: auto;
  -webkit-appearance: auto;
  -moz-appearance: auto;
  cursor: pointer;
  background-image: none;
  padding-right: 10px;
}

.check-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  margin-top: 8px;
}

.check-item {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: 12px;
  color: #475569;
}

.check-item input[type='checkbox'] {
  width: 14px;
  height: 14px;
}

/* === 保存配置 === */
.save-config {
  display: flex;
  gap: 6px;
  margin-top: 14px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.save-config input {
  flex: 1;
  min-width: 0;
  min-height: 32px;
  padding: 6px 10px;
}

.save-group-input {
  width: 80px;
  flex: 0 0 80px;
}

.save-btn {
  padding: 6px 14px;
  min-height: 32px;
  background: #475569;
  color: #fff;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s;
}

.save-btn:hover:not(:disabled) {
  background: #334155;
}

.save-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* === 连接按钮 === */
.submit-btn {
  width: 100%;
  padding: 10px;
  background: #cf0a2c;
  color: #fff;
  border: none;
  border-radius: 4px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  margin-top: 4px;
  transition: background 0.15s;
}

.submit-btn:hover:not(:disabled) {
  background: #b50c2b;
}

.submit-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.error {
  color: #dc2626;
  font-size: 12px;
  margin-top: 6px;
  text-align: center;
}

/* === 状态栏 === */
.status-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 28px;
  padding: 0 16px;
  background: #f8fafc;
  border-top: 1px solid #e2e8f0;
  font-size: 11px;
  color: #94a3b8;
  flex-shrink: 0;
}

/* === 响应式 === */
@media (max-width: 799px) {
  .form-panel {
    position: fixed;
    right: 0;
    top: 86px;
    bottom: 28px;
    width: 360px;
    z-index: 100;
    box-shadow: -4px 0 16px rgba(0, 0, 0, 0.1);
  }

  .col-date {
    display: none;
  }

  .toolbar-right {
    flex-wrap: wrap;
  }
}

@media (max-width: 599px) {
  .col-user {
    display: none;
  }

  .form-panel {
    width: 100%;
    left: 0;
  }
}

/* === 自定义确认对话框 === */
.confirm-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.confirm-box {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  min-width: 320px;
  max-width: 420px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.confirm-msg {
  font-size: 14px;
  color: #1e293b;
  line-height: 1.6;
  margin-bottom: 20px;
  white-space: pre-line;
}

.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.confirm-btn {
  padding: 6px 18px;
  font-size: 13px;
  border-radius: 4px;
  cursor: pointer;
  border: 1px solid #d1d5db;
  transition: all 0.15s;
}

.confirm-cancel {
  background: #fff;
  color: #475569;
}

.confirm-cancel:hover {
  background: #f1f5f9;
}

.confirm-ok {
  background: #cf0a2c;
  color: #fff;
  border-color: #cf0a2c;
}

.confirm-ok:hover {
  background: #b50c2b;
}
</style>
