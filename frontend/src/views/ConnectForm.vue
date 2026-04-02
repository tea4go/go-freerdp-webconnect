<template>
  <div class="app-layout">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <input
          v-model="searchQuery"
          type="text"
          class="search-input"
          placeholder="搜索配置..."
        />
        <select v-if="allGroups.length" v-model="filterGroup" class="group-filter">
          <option value="">全部分组</option>
          <option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option>
        </select>
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
      <!-- 配置表格 -->
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
              @dblclick="handleQuickConnect(cfg)"
              @contextmenu.prevent="onRowContextMenu(cfg, $event)"
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
                <button type="button" class="action-edit" @click.stop="handleEditConfig(cfg)">编辑</button>
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
    </div>

    <!-- 配置弹框 -->
    <ConfigDialog
      :visible="showDialog"
      :editing-id="editingConfigId"
      :initial-name="dialogName"
      :initial-group="dialogGroup"
      :initial-form="dialogForm"
      :all-groups="allGroups"
      :connecting="connecting"
      :error="error"
      @close="showDialog = false"
      @save="handleSaveConfig"
      @connect="handleConnectFromDialog"
    />

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

    <!-- 右键菜单 -->
    <ContextMenu
      :visible="ctxMenu.visible"
      :x="ctxMenu.x"
      :y="ctxMenu.y"
      :items="ctxMenuItems"
      @select="onCtxMenuSelect"
      @close="ctxMenu.visible = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Connect, GetVersion, SaveFile, OpenFile } from '../wailsjs/go/backend/App'
import { ClipboardSetText } from '../wailsjs/runtime/runtime'
import ConfigDialog from '../components/ConfigDialog.vue'
import ContextMenu from '../components/ContextMenu.vue'
import type { MenuItem } from '../components/ContextMenu.vue'
import type { ConfigFormData, ConfigDialogPayload } from '../components/ConfigDialog.vue'

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
  connect: [wsUrl: string, width: number, height: number, name: string]
}>()

const defaultForm: ConfigFormData = {
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
}

const connecting = ref(false)
const error = ref('')
const version = ref<{ app: string; freerdp: string } | null>(null)
const savedConfigs = ref<SavedConfig[]>([])
const editingConfigId = ref<string | null>(null)
const showDialog = ref(false)
const dialogName = ref('')
const dialogGroup = ref('')
const dialogForm = ref<ConfigFormData>({ ...defaultForm })

// 右键菜单
const ctxMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  targetId: '' as string,
})

const ctxMenuItems: MenuItem[] = [
  { label: '连接', action: 'connect' },
  { label: '编辑', action: 'edit' },
  { type: 'separator' },
  { label: '复制', action: 'copy' },
  { type: 'separator' },
  { label: '删除', action: 'delete', danger: true },
]

function onRowContextMenu(cfg: SavedConfig, e: MouseEvent) {
  ctxMenu.targetId = cfg.id
  ctxMenu.x = e.clientX
  ctxMenu.y = e.clientY
  ctxMenu.visible = true
}

function onCtxMenuSelect(action: string) {
  const cfg = savedConfigs.value.find(c => c.id === ctxMenu.targetId)
  if (!cfg) return
  switch (action) {
    case 'connect':
      handleQuickConnect(cfg)
      break
    case 'edit':
      handleEditConfig(cfg)
      break
    case 'copy':
      handleCopyConfig(cfg)
      break
    case 'delete':
      handleDeleteConfig(cfg.id)
      break
  }
}

function handleCopyConfig(cfg: SavedConfig) {
  const newCfg: SavedConfig = {
    ...cfg,
    id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
    name: cfg.name + ' - 副本',
    createdAt: Date.now(),
  }
  savedConfigs.value.push(newCfg)
  persistConfigs(savedConfigs.value)
}

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

function handleNewConfig() {
  dialogForm.value = { ...defaultForm }
  dialogName.value = ''
  dialogGroup.value = ''
  editingConfigId.value = null
  error.value = ''
  showDialog.value = true
}

function handleEditConfig(cfg: SavedConfig) {
  dialogForm.value = {
    host: cfg.host, port: cfg.port, user: cfg.user, pass: cfg.pass,
    resolution: cfg.resolution, perf: cfg.perf, fntlm: cfg.fntlm,
    nowallp: cfg.nowallp, nowdrag: cfg.nowdrag, nomani: cfg.nomani,
    notheme: cfg.notheme, nonla: cfg.nonla, notls: cfg.notls,
  }
  dialogName.value = cfg.name
  dialogGroup.value = cfg.group || ''
  editingConfigId.value = cfg.id
  error.value = ''
  showDialog.value = true
}

async function handleSaveConfig(payload: ConfigDialogPayload) {
  const name = payload.name.trim()
  if (!name) return

  if (payload.form.pass && !localStorage.getItem(WARN_KEY)) {
    const ok = await showConfirm('密码将以混淆形式保存在本地浏览器中，请确保设备安全。\n\n点击"确定"继续保存，后续不再提示。')
    if (!ok) return
    localStorage.setItem(WARN_KEY, '1')
  }

  const formSnapshot = { ...payload.form, group: payload.group.trim() }

  if (payload.editingId) {
    const idx = savedConfigs.value.findIndex(c => c.id === payload.editingId)
    if (idx !== -1) {
      savedConfigs.value[idx] = { ...savedConfigs.value[idx], name, ...formSnapshot }
      persistConfigs(savedConfigs.value)
      showDialog.value = false
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
    showDialog.value = false
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
  showDialog.value = false
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
    showDialog.value = false
  }
}

async function handleExportConfigs() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) {
    await showAlert('没有可导出的配置')
    return
  }

  const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
  try {
    await SaveFile(`rdp-configs-${date}.json`, raw)
  } catch (e: any) {
    await showAlert('导出失败：' + (e.message || e))
  }
}

async function triggerImport() {
  try {
    const content = await OpenFile()
    if (!content) return // 用户取消

    const data = JSON.parse(content)
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

async function handleQuickConnect(cfg: SavedConfig) {
  const formData: ConfigFormData = {
    host: cfg.host, port: cfg.port, user: cfg.user, pass: cfg.pass,
    resolution: cfg.resolution, perf: cfg.perf, fntlm: cfg.fntlm,
    nowallp: cfg.nowallp, nowdrag: cfg.nowdrag, nomani: cfg.nomani,
    notheme: cfg.notheme, nonla: cfg.nonla, notls: cfg.notls,
  }
  await doConnect(formData, cfg.name)
}

async function handleConnectFromDialog(formData: ConfigFormData) {
  await doConnect(formData, dialogName.value.trim() || '')
}

async function doConnect(formData: ConfigFormData, name?: string) {
  if (!formData.host) {
    error.value = '请输入主机地址'
    return
  }

  connecting.value = true
  error.value = ''

  try {
    const [w, h] = formData.resolution.split('x').map(Number)

    const wsUrl: string = await Connect(
      formData.host, formData.user, formData.pass, formData.port, w, h,
      formData.perf, formData.fntlm,
      formData.nowallp, formData.nowdrag, formData.nomani, formData.notheme, formData.nonla, formData.notls
    )

    showDialog.value = false
    emit('connect', wsUrl, w, h, name || formData.host)
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
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #f3f4f6;
  position: relative;
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
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  max-width: 400px;
}

.group-filter {
  padding: 6px 10px;
  font-size: 12px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: #f9fafb;
  color: #374151;
  cursor: pointer;
  min-width: 90px;
  height: 30px;
  box-sizing: border-box;
  appearance: auto;
  -webkit-appearance: auto;
  -moz-appearance: auto;
  background-image: none;
}

.group-filter:focus {
  border-color: #9ca3af;
  background: #fff;
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
  width: 20%;
}

.col-host {
  width: 25%;
}

.col-user {
  width: 15%;
}

.col-date {
  width: 22%;
}

.col-actions {
  width: 18%;
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

.action-edit {
  padding: 3px 10px;
  font-size: 11px;
  background: transparent;
  color: #475569;
  border: 1px solid #d1d5db;
  border-radius: 3px;
  cursor: pointer;
  margin-left: 4px;
  transition: all 0.15s;
}

.action-edit:hover {
  background: #eff6ff;
  color: #2563eb;
  border-color: #93c5fd;
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
