<script setup lang="ts">
import logo from '@/assets/logo'
import { ExitApp } from '@/bridge'
import { useAppStore, useKernelApiStore } from '@/stores'
const appStore = useAppStore()
const kernelApiStore = useKernelApiStore()
</script>
<template>
  <div class="app-titlebar flex items-center py-8 gap-8 px-12">
    <img :src="logo" class="w-24 h-24" /><strong class="app-titlebar__name">SingBox WebUI</strong>
    <span class="text-12">{{ kernelApiStore.running ? '核心运行中' : '核心已停止' }}</span>
    <CustomAction :actions="appStore.customActions.title_bar" />
    <Button class="ml-auto" type="text" @click="ExitApp">退出登录</Button>
  </div>
</template>
