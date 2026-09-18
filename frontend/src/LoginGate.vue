<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

import { connectEvents, disconnectEvents } from '@/bridge/runtime'
import { api } from '@/bridge/transport'

import App from './App.vue'
const authenticated = ref(false)
const password = ref('')
const loading = ref(true)
const error = ref('')
const unauthorized = () => {
  disconnectEvents()
  if (authenticated.value) location.reload()
}
window.addEventListener('webui:unauthorized', unauthorized)
onUnmounted(() => window.removeEventListener('webui:unauthorized', unauthorized))
async function enter() {
  await connectEvents()
  authenticated.value = true
}
onMounted(async () => {
  try {
    if ((await api('/session')).authenticated) await enter()
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})
async function login() {
  loading.value = true
  error.value = ''
  try {
    await api('/login', { password: password.value })
    password.value = ''
    await enter()
  } catch (e) {
    error.value = String(e)
    disconnectEvents()
  } finally {
    loading.value = false
  }
}
</script>
<template>
  <App v-if="authenticated" />
  <main v-else class="login-page">
    <form class="login-card" @submit.prevent="login">
      <h1>SingBox WebUI</h1>
      <p>登录服务器管理面板</p>
      <label for="password">管理员密码</label>
      <input
        id="password"
        v-model="password"
        type="password"
        autocomplete="current-password"
        required
        autofocus
      />
      <p v-if="error" role="alert" class="login-error">{{ error }}</p>
      <button :disabled="loading || !password">{{ loading ? '连接中…' : '登录' }}</button>
      <small>关闭页面或退出登录不会停止核心。</small>
    </form>
  </main>
</template>
<style scoped>
.login-page {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background: #101827;
  color: #e5eaf3;
  padding: 24px;
}
.login-card {
  width: min(100%, 380px);
  padding: 32px;
  border: 1px solid #354157;
  border-radius: 16px;
  background: #1a2435;
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.login-card h1 {
  font-size: 26px;
  font-weight: 700;
}
.login-card input {
  padding: 12px;
  background: #101827;
  border: 1px solid #53627b;
  border-radius: 8px;
  color: inherit;
}
.login-card button {
  padding: 12px;
  border-radius: 8px;
  background: #466ee8;
  color: white;
}
.login-card button:disabled {
  opacity: 0.5;
}
.login-card small {
  color: #aab8cf;
}
.login-error {
  color: #ffa5a5;
  overflow-wrap: anywhere;
}
</style>
