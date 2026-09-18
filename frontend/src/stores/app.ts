import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Download,
  HttpGet,
  MoveFile,
  UnzipZIPFile,
  RemoveFile,
  HttpCancel,
  ReadDir,
  Exec,
} from '@/bridge'
import { LanguageOptions, LocalesFilePath, RollingReleaseDirectory } from '@/constant/app'
import { OS } from '@/enums/app'
import { loadLocale } from '@/lang'
import {
  APP_TITLE,
  APP_VERSION,
  APP_VERSION_API,
  getGitHubApiAuthorization,
  confirm,
  message,
  sampleID,
  sleep,
  modal,
} from '@/utils'

import AboutView from '@/components/_common/AboutView.vue'

import { useEnvStore } from './env'

export const useAppStore = defineStore('app', () => {
  const isAppExiting = ref(false)
  const isAppReloading = ref(false)

  /* Global Menu */
  const menuShow = ref(false)
  const menuList = ref<App.Menu[]>([])
  const menuPosition = ref({
    x: 0,
    y: 0,
  })

  /* Global Tips */
  const tipsShow = ref(false)
  const tipsMessage = ref('')
  const tipsPosition = ref({
    x: 0,
    y: 0,
  })

  /* Modal Stack */
  const modalStack: (() => void)[] = []
  const modalZIndexCounter = 999
  const modalMinimized = ref<
    {
      id: string
      title: () => string
      openFn: () => void
      closeFn: () => void
      minimizeFn: () => void
    }[]
  >([])

  /* Side-by-side modal tabs */
  const modalTabs = ref<
    {
      id: string
      title: () => string
      activate: () => void
      close: () => void
    }[]
  >([])
  const activeModalTab = ref('')
  const modalSideBySide = ref(false)
  const modalSplitActive = ref(false)

  /* i18n */
  const localesLoading = ref(false)
  const locales = ref<{ label: string; value: string }[]>([])
  const loadLocales = async (delay = true, reload = true) => {
    localesLoading.value = true
    const dirs = await ReadDir(LocalesFilePath).catch(() => [])
    const localLanguage = dirs.flatMap((file) => {
      if (file.isDir) return []
      const [name, ext] = file.name.split('.')
      return name && ext === 'json' ? { label: name, value: name } : []
    })
    locales.value = [...LanguageOptions, ...localLanguage]
    reload && (await loadLocale())
    delay && (await sleep(200))
    localesLoading.value = false
  }

  /* Actions */
  const customActions = ref({
    core_state: [] as (App.CustomAction | App.CustomActionFn)[],
    title_bar: [] as (App.CustomAction | App.CustomActionFn)[],
    profiles_header: [] as (App.CustomAction | App.CustomActionFn)[],
    subscriptions_header: [] as (App.CustomAction | App.CustomActionFn)[],
  })
  const addCustomActions = (
    target: keyof typeof customActions.value,
    actions: App.CustomAction | App.CustomAction[] | App.CustomActionFn | App.CustomActionFn[],
  ) => {
    if (!customActions.value[target]) throw new Error('Target does not exist: ' + target)
    const _actions = Array.isArray(actions) ? actions : [actions]
    _actions.forEach((action) => !action.id && (action.id = sampleID()))
    customActions.value[target].push(..._actions)
    const remove = () => {
      customActions.value[target] = customActions.value[target].filter(
        (a) => !_actions.some((added) => added.id === a.id),
      )
    }
    return remove
  }
  const removeCustomActions = (target: keyof typeof customActions.value, id: string | string[]) => {
    if (!customActions.value[target]) throw new Error('Target does not exist: ' + target)
    const ids = Array.isArray(id) ? id : [id]
    customActions.value[target] = customActions.value[target].filter((a) => !ids.includes(a.id!))
  }

  const { t } = useI18n()
  const envStore = useEnvStore()

  /* About Page */
  const showAbout = ref(false)
  const lastCheckTime = ref(0)
  const checkForUpdatesLoading = ref(false)
  const restartable = ref(false)
  const downloading = ref(false)
  const downloadUrl = ref('')
  const downloadDigest = ref('')
  const remoteVersion = ref(APP_VERSION)
  const updatable = computed(() => downloadUrl.value && APP_VERSION !== remoteVersion.value)

  const downloadApp = async () => {
    throw new Error('WebUI updates are installed by the server administrator')
  }
  const checkForUpdates = async (_showTips = false) => {}
  watch(showAbout, (v) => {
    if (v) {
      const m = modal({
        title: 'router.about',
        submit: false,
        cancelText: 'common.close',
        toolbar: {
          minimize: false,
          maximize: false,
        },
        maskClosable: true,
        minWidth: '60',
        afterDestroy() {
          showAbout.value = false
        },
      })
      m.setContent(AboutView).open()
    }
  })

  return {
    isAppExiting,
    isAppReloading,
    menuShow,
    menuPosition,
    menuList,
    tipsShow,
    tipsMessage,
    tipsPosition,
    modalTabs,
    activeModalTab,
    modalSideBySide,
    modalSplitActive,
    modalStack,
    modalMinimized,
    modalZIndexCounter,
    showAbout,
    lastCheckTime,
    checkForUpdatesLoading,
    restartable,
    downloading,
    remoteVersion,
    updatable,
    checkForUpdates,
    downloadApp,
    customActions,
    addCustomActions,
    removeCustomActions,
    localesLoading,
    locales,
    loadLocales,
  }
})
