<template>
  <div v-if="visible" class="dialog-overlay" @click.self="emit('close')">
    <div class="dialog-box">
      <div class="dialog-header">
        <span>{{ editingId ? '编辑配置' : '新建连接' }}</span>
        <button type="button" class="dialog-close-btn" @click="emit('close')">&times;</button>
      </div>
      <div class="dialog-body">
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
              <label for="dlg-host">主机地址</label>
              <input id="dlg-host" v-model="form.host" type="text" placeholder="192.168.1.100" required />
            </div>
            <div class="form-row">
              <div class="form-group">
                <label for="dlg-port">端口</label>
                <input id="dlg-port" v-model.number="form.port" type="number" />
              </div>
              <div class="form-group">
                <label for="dlg-resolution">分辨率</label>
                <select id="dlg-resolution" v-model="form.resolution">
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
              <label for="dlg-user">用户名</label>
              <input id="dlg-user" v-model="form.user" type="text" placeholder="administrator" />
            </div>
            <div class="form-group">
              <label for="dlg-pass">密码</label>
              <input id="dlg-pass" v-model="form.pass" type="password" />
            </div>
          </div>

          <div v-show="activeTab === 'advanced'" class="tab-panel">
            <div class="form-group">
              <label for="dlg-perf">性能</label>
              <select id="dlg-perf" v-model.number="form.perf">
                <option :value="0">局域网</option>
                <option :value="1">宽带</option>
                <option :value="2">调制解调器</option>
              </select>
            </div>
            <div class="form-group">
              <label for="dlg-fntlm">强制 NTLM 认证</label>
              <select id="dlg-fntlm" v-model.number="form.fntlm">
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
            <input v-model="configGroup" type="text" placeholder="分组" maxlength="30" list="dlg-group-list" class="save-group-input" />
            <datalist id="dlg-group-list">
              <option v-for="g in allGroups" :key="g" :value="g" />
            </datalist>
            <button type="button" class="save-btn" :disabled="!configName.trim()" @click="handleSave">保存</button>
          </div>

          <button type="submit" class="submit-btn" :disabled="connecting">
            {{ connecting ? '连接中...' : '连接' }}
          </button>
          <p v-if="error" class="error">{{ error }}</p>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'

export interface ConfigFormData {
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
}

export interface ConfigDialogPayload {
  editingId: string | null
  name: string
  group: string
  form: ConfigFormData
}

const props = defineProps<{
  visible: boolean
  editingId: string | null
  initialName: string
  initialGroup: string
  initialForm: ConfigFormData
  allGroups: string[]
  connecting: boolean
  error: string
}>()

const emit = defineEmits<{
  close: []
  save: [payload: ConfigDialogPayload]
  connect: [form: ConfigFormData]
}>()

const activeTab = ref<'basic' | 'advanced'>('basic')
const configName = ref('')
const configGroup = ref('')

const form = reactive<ConfigFormData>({
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

watch(() => props.visible, (v) => {
  if (v) {
    Object.assign(form, props.initialForm)
    configName.value = props.initialName
    configGroup.value = props.initialGroup
    activeTab.value = 'basic'
  }
})

watch(() => form.notls, (v) => {
  if (v) form.nonla = true
})

function handleSave() {
  emit('save', {
    editingId: props.editingId,
    name: configName.value,
    group: configGroup.value,
    form: { ...form },
  })
}

function handleConnect() {
  emit('connect', { ...form })
}
</script>

<style scoped>
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9998;
}

.dialog-box {
  background: #fff;
  border-radius: 8px;
  width: 420px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e2e8f0;
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
  flex-shrink: 0;
}

.dialog-close-btn {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: #64748b;
  font-size: 20px;
  cursor: pointer;
  border-radius: 4px;
  transition: background 0.15s;
}

.dialog-close-btn:hover {
  background: #e2e8f0;
  color: #1e293b;
}

.dialog-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.dialog-body::-webkit-scrollbar {
  width: 6px;
}

.dialog-body::-webkit-scrollbar-thumb {
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

@media (max-width: 480px) {
  .dialog-box {
    width: 95vw;
  }
}
</style>
