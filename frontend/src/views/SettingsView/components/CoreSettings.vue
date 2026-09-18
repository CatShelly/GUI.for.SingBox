<script setup lang="ts">
import { modal } from '@/utils'

import BranchDetail from './components/BranchDetail.vue'
import CoreConfiguration from './components/CoreConfig.vue'
import SwitchBranch from './components/SwitchBranch.vue'

const handleCoreConfiguraion = async (isAlpha: boolean) => {
  const m = modal({ title: 'settings.kernel.config.name', minWidth: '70' })
  m.setContent(CoreConfiguration, { isAlpha }).open()
}
</script>

<template>
  <div class="flex flex-col gap-8">
    <p class="p-8">
      核心下载到服务器数据目录，按服务器系统和架构选择安装包。更新完成后重启核心生效。
    </p>
    <BranchDetail :is-alpha="false" @config="handleCoreConfiguraion(false)" />
    <BranchDetail :is-alpha="true" @config="handleCoreConfiguraion(true)" />
    <SwitchBranch />
  </div>
</template>
