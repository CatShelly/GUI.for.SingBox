import { ref } from 'vue'

import { connectEvents } from '@/bridge/runtime'

import * as Stores from '@/stores'
import { message, sleep } from '@/utils'

const MIN_SPLASH_DURATION = 1000

export const useAppBootstrap = () => {
  const loading = ref(true)
  const percent = ref(0)
  const hasError = ref(false)

  const envStore = Stores.useEnvStore()
  const appSettings = Stores.useAppSettingsStore()
  const profilesStore = Stores.useProfilesStore()
  const subscribesStore = Stores.useSubscribesStore()
  const rulesetsStore = Stores.useRulesetsStore()
  const pluginsStore = Stores.usePluginsStore()
  const kernelApiStore = Stores.useKernelApiStore()

  const showError = (error: unknown) => {
    hasError.value = true
    message.error(error)
  }

  const initialize = async () => {
    await envStore.setupEnv()

    await Promise.all([
      appSettings.setupAppSettings(),
      profilesStore.setupProfiles(),
      subscribesStore.setupSubscribes(),
      rulesetsStore.setupRulesets(),
      pluginsStore.setupPlugins(),
    ])

    const startTime = performance.now()
    percent.value = 20

    await connectEvents()

    percent.value = 40
    await pluginsStore.onReadyTrigger().catch(showError)

    const duration = performance.now() - startTime
    percent.value = duration < 500 ? 80 : 100

    await sleep(Math.max(0, MIN_SPLASH_DURATION - duration))

    loading.value = false
    await kernelApiStore.initCoreState().catch(showError)
  }

  initialize().catch(showError)

  return {
    loading,
    percent,
    hasError,
  }
}
