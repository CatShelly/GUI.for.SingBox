import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { EventsOn } from '@/bridge/runtime'
import { api } from '@/bridge/transport'

import {
  getProxies,
  getConfigs,
  setConfigs,
  onLogs,
  onMemory,
  onConnections,
  onTraffic,
  initWebsocket,
  destroyWebsocket,
  probeApiAvailability,
} from '@/api/kernel'
import { ProcessInfo, KillProcess, ExecBackground, ReadFile, RemoveFile } from '@/bridge'
import {
  CoreConfigFilePath,
  CoreLogFilePath,
  CorePidFilePath,
  CoreWorkingDirectory,
} from '@/constant/kernel'
import { DefaultInboundMixed } from '@/constant/profile'
import { Branch } from '@/enums/app'
import { Inbound, RulesetType, TunStack } from '@/enums/kernel'
import {
  useAppSettingsStore,
  useProfilesStore,
  useLogsStore,
  useEnvStore,
  usePluginsStore,
  useSubscribesStore,
  useRulesetsStore,
} from '@/stores'
import {
  generateConfig,
  updateTrayAndMenus,
  getKernelFileName,
  normalizeProxyHost,
  restoreProfile,
  deepClone,
  message,
  getKernelRuntimeArgs,
  getKernelRuntimeEnv,
  eventBus,
  sleep,
} from '@/utils'

import type { CoreApiConfig, CoreApiProxy } from '@/types/kernel'

export type ProxyType = 'mixed' | 'http' | 'socks'
export type ProxyEndpoint = {
  schema: 'http' | 'socks5'
  host: string
  port: number
  username: string
  password: string
  proxyType: ProxyType
}

export const useKernelApiStore = defineStore('kernelApi', () => {
  const envStore = useEnvStore()
  const logsStore = useLogsStore()
  const pluginsStore = usePluginsStore()
  const profilesStore = useProfilesStore()
  const subscribesStore = useSubscribesStore()
  const rulesetsStore = useRulesetsStore()
  const appSettingsStore = useAppSettingsStore()

  /** RESTful API */
  const config = ref<CoreApiConfig>({
    port: 0,
    'mixed-port': 0,
    'socks-port': 0,
    'interface-name': '',
    'allow-lan': false,
    mode: '',
    tun: {
      enable: false,
      stack: '',
      device: '',
    },
  })

  let runtimeProfile: App.Profile | undefined

  const proxies = ref<Record<string, CoreApiProxy>>({})

  const refreshConfig = async () => {
    const _config = await getConfigs()

    config.value = {
      ..._config,
      tun: config.value.tun,
    }

    if (!runtimeProfile) {
      const txt = await ReadFile(CoreConfigFilePath)
      runtimeProfile = restoreProfile(JSON.parse(txt))
      const profile = profilesStore.currentProfile
      if (profile) {
        const _profile = deepClone(profile)
        _profile.inbounds.forEach((inbound) => {
          const runtimeInbound = runtimeProfile?.inbounds.find((v) => v.tag === inbound.tag)
          if (runtimeInbound) {
            runtimeInbound.id = inbound.id
          } else {
            inbound.enable = false
            runtimeProfile?.inbounds.push(inbound)
          }
        })
        runtimeProfile.id = _profile.id
        runtimeProfile.outbounds = _profile.outbounds
        runtimeProfile.experimental = _profile.experimental
        runtimeProfile.dns = _profile.dns
        runtimeProfile.route = _profile.route
        runtimeProfile.mixin = _profile.mixin
        runtimeProfile.script = _profile.script
      }
    }

    const mixed = runtimeProfile.inbounds.find((v) => v.enable && v.mixed)
    const http = runtimeProfile.inbounds.find((v) => v.enable && v.http)
    const socks = runtimeProfile.inbounds.find((v) => v.enable && v.socks)
    const tun = runtimeProfile.inbounds.find((v) => v.tun)
    config.value['mixed-port'] = mixed?.mixed?.listen.listen_port || 0
    config.value['port'] = http?.http?.listen.listen_port || 0
    config.value['socks-port'] = socks?.socks?.listen.listen_port || 0
    config.value['allow-lan'] = [
      mixed?.mixed?.listen.listen,
      http?.http?.listen.listen,
      socks?.socks?.listen.listen,
    ].some((address) => address === '0.0.0.0' || address === '::')

    config.value.tun.enable = !!tun?.enable
    config.value.tun.device = tun?.tun?.interface_name || ''
    config.value.tun.stack = tun?.tun?.stack || ''
    config.value['interface-name'] = runtimeProfile.route.default_interface
  }

  const resetConfig = () => {
    config.value.port = 0
    config.value['socks-port'] = 0
    config.value['mixed-port'] = 0
    config.value['interface-name'] = ''
    config.value['allow-lan'] = false
    config.value.mode = ''
    config.value.tun.enable = false
    config.value.tun.stack = ''
    config.value.tun.device = ''
  }

  const updateConfig = async (field: string, value: any) => {
    if (field === 'mode') {
      await setConfigs({ mode: value })
      await refreshConfig()
      return
    }

    const patchInbound = () => {
      if (!runtimeProfile) return
      const inbound = runtimeProfile.inbounds.find(
        (v) =>
          (v.type === Inbound.Mixed && v.mixed?.listen.listen_port) ||
          (v.type === Inbound.Http && v.http?.listen.listen_port) ||
          (v.type === Inbound.Socks && v.socks?.listen.listen_port),
      )
      if (!inbound) {
        throw 'home.overview.needPort'
      }
      inbound.enable = true
    }

    const patchInboundPort = (type: 'mixed' | 'socks' | 'http', port: number) => {
      if (!runtimeProfile) return
      let inbound = runtimeProfile.inbounds.find((v) => v.type === type)
      if (inbound) {
        inbound[type]!.listen.listen_port = port
      } else {
        const _type = DefaultInboundMixed()!
        _type.listen.listen_port = port
        inbound = {
          id: type + '-in',
          tag: type + '-in',
          type: type,
          enable: true,
          [type]: _type,
        }
        runtimeProfile.inbounds.push(inbound)
      }
      inbound.enable = port !== 0
    }

    const patchInboundAddress = (allowLan: boolean) => {
      if (!runtimeProfile) return
      runtimeProfile.inbounds.forEach((inbound) => {
        if (inbound.type === Inbound.Tun) return
        inbound[inbound.type]!.listen.listen = allowLan ? '0.0.0.0' : '127.0.0.1'
      })
    }

    const patchInboundTun = (options: {
      enable: boolean
      stack: string
      device: string
      interface_name: string
    }) => {
      if (!runtimeProfile) return
      const inbound = runtimeProfile.inbounds.find((v) => v.type === Inbound.Tun)
      if (!inbound) throw 'home.overview.needTun'
      options = { ...config.value.tun, ...options }
      inbound.enable = options.enable
      inbound.tun!.stack = (options.stack || TunStack.Mixed) as App.TunStack
      inbound.tun!.interface_name = options.device || ''
      if (options.interface_name) {
        runtimeProfile.route.default_interface = options.interface_name
      }
      runtimeProfile.route.auto_detect_interface = !options.interface_name
    }

    const fieldHandlerMap: Recordable<() => void> = {
      inbound: () => patchInbound(),
      http: () => patchInboundPort(Inbound.Http, value),
      socks: () => patchInboundPort(Inbound.Socks, value),
      mixed: () => patchInboundPort(Inbound.Mixed, value),
      'allow-lan': () => patchInboundAddress(value),
      tun: () => patchInboundTun(value),
      'tun-stack': () => patchInboundTun(value),
      'tun-device': () => patchInboundTun(value),
      'interface-name': () => patchInboundTun(value),
    }

    fieldHandlerMap[field]?.()

    await restartCore(undefined, true)
    await envStore.updateSystemProxyStatus()
  }

  const refreshProviderProxies = async () => {
    const { proxies: b } = await getProxies()
    proxies.value = b
  }

  const { t } = useI18n()

  /* Bridge API */
  const corePid = ref(-1)
  const running = ref(false)
  const starting = ref(false)
  const stopping = ref(false)
  const restarting = ref(false)
  const needRestart = ref(false)
  const coreStateLoading = ref(true)
  let revision = 0
  let initialized = false
  let statusQueue = Promise.resolve()
  type CoreStatus = {
    running: boolean
    pid: number
    revision: number
    profile?: App.Profile
    alpha: boolean
    error: string
  }
  const acceptStatus = async (state: CoreStatus) => {
    const changed = state.pid !== corePid.value || state.revision !== revision
    const wasRunning = running.value
    corePid.value = state.pid
    running.value = state.running
    revision = state.revision
    if (state.profile) runtimeProfile = deepClone(state.profile)
    if (changed || !initialized) {
      destroyWebsocket()
      if (state.running) {
        initWebsocket()
        await Promise.all([refreshConfig(), refreshProviderProxies()]).catch((e) =>
          message.error(e),
        )
        await pluginsStore.onCoreStartedTrigger()
      } else {
        resetConfig()
        if (wasRunning) await pluginsStore.onCoreStoppedTrigger()
      }
    }
    initialized = true
    coreStateLoading.value = false
  }
  const enqueueStatus = (state: CoreStatus) => {
    statusQueue = statusQueue.catch(() => {}).then(() => acceptStatus(state))
    return statusQueue
  }
  const initCoreState = async () => {
    await enqueueStatus(await api<CoreStatus>('/core/status'))
  }
  EventsOn('webui:core', (state: CoreStatus) => {
    void enqueueStatus(state).catch((e) => message.error(e))
  })
  EventsOn('webui:reconnected', () => {
    if (initialized) void initCoreState().catch((e) => message.error(e))
  })

  const applyProfile = async (profile: App.Profile) => {
    let generated = await generateConfig(profile)
    generated = await pluginsStore.onBeforeCoreStartTrigger(generated, profile)
    generated.experimental ??= {}
    generated.experimental.cache_file ??= {}
    generated.experimental.cache_file.path = 'cache.db'
    const alpha = appSettingsStore.app.kernel.branch === Branch.Alpha
    const state = await api<CoreStatus>('/core/apply', {
      config: generated,
      profile: deepClone(profile),
      alpha,
      env: getKernelRuntimeEnv(alpha),
      revision,
    })
    needRestart.value = false
    await enqueueStatus(state)
  }
  const startCore = async (_profile?: App.Profile) => {
    const profile = _profile || profilesStore.getProfileById(appSettingsStore.app.kernel.profile)
    if (!profile) throw new Error('Choose a profile first')
    starting.value = true
    try {
      await applyProfile(profile)
    } finally {
      starting.value = false
      await initCoreState()
    }
  }
  const stopCore = async () => {
    stopping.value = true
    try {
      await pluginsStore.onBeforeCoreStopTrigger()
      await enqueueStatus(await api<CoreStatus>('/core/stop', {}))
    } finally {
      stopping.value = false
      await initCoreState()
    }
  }
  const restartCore = async (cleanupTask?: () => Promise<any>, keepRuntimeProfile = false) => {
    restarting.value = true
    try {
      await pluginsStore.onBeforeCoreStopTrigger()
      if (cleanupTask) {
        await api('/core/stop', {})
        await cleanupTask()
      }
      const profile = keepRuntimeProfile
        ? runtimeProfile
        : profilesStore.getProfileById(appSettingsStore.app.kernel.profile)
      if (!profile) throw new Error('Choose a profile first')
      await applyProfile(profile)
    } finally {
      restarting.value = false
      await initCoreState()
    }
  }
  const restartAppliedCore = async () => {
    restarting.value = true
    try {
      await enqueueStatus(await api<CoreStatus>('/core/restart', {}))
    } finally {
      restarting.value = false
      await initCoreState()
    }
  }

  const getProxyProfileOptions = (proxyType: ProxyType) => {
    const inboundTypeMap = {
      mixed: Inbound.Mixed,
      http: Inbound.Http,
      socks: Inbound.Socks,
    } satisfies Record<ProxyType, Inbound>

    const inbound = runtimeProfile?.inbounds.find(
      (item) => item.enable && item.type === inboundTypeMap[proxyType],
    )

    const inboundOptions =
      proxyType === Inbound.Mixed
        ? inbound?.mixed
        : proxyType === Inbound.Http
          ? inbound?.http
          : inbound?.socks

    const listen = inboundOptions?.listen.listen || ''
    const auth = inboundOptions?.users[0]?.trim()
    const host = normalizeProxyHost((listen || '').trim())

    if (!auth) return { host, username: '', password: '' }

    const [username, ...passwordParts] = auth.split(':')

    return {
      host,
      username: username || '',
      password: passwordParts.join(':'),
    }
  }

  const getProxyEndpoint = (): ProxyEndpoint | undefined => {
    const { port, 'socks-port': socksPort, 'mixed-port': mixedPort } = config.value
    let targetPort = 0
    let proxyType: ProxyType | undefined

    if (mixedPort) {
      targetPort = mixedPort
      proxyType = 'mixed'
    } else if (port) {
      targetPort = port
      proxyType = 'http'
    } else if (socksPort) {
      targetPort = socksPort
      proxyType = 'socks'
    } else {
      return undefined
    }

    const { host, username, password } = getProxyProfileOptions(proxyType)
    const schema = proxyType === 'socks' ? 'socks5' : 'http'

    return {
      schema,
      host,
      port: targetPort,
      username,
      password,
      proxyType,
    }
  }

  eventBus.on('profileChange', ({ id }) => {
    if (running.value && id === appSettingsStore.app.kernel.profile) {
      needRestart.value = true
    }
  })

  eventBus.on('subscriptionChange', ({ id }) => {
    if (running.value && profilesStore.currentProfile) {
      const inUse = profilesStore.currentProfile.outbounds.some(({ outbounds }) =>
        outbounds.some((outbound) => outbound.type === 'Subscription' && outbound.id === id),
      )
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  eventBus.on('subscriptionsChange', () => {
    if (running.value && profilesStore.currentProfile) {
      const enabledSubs = subscribesStore.subscribes.flatMap((v) => (v.disabled ? [] : v.id))
      const inUse = profilesStore.currentProfile.outbounds.some(({ outbounds }) =>
        outbounds.some(
          (outbound) => outbound.type === 'Subscription' && enabledSubs.includes(outbound.id),
        ),
      )
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  const collectRulesetIDs = () => {
    if (!profilesStore.currentProfile) return []
    const l1 = profilesStore.currentProfile.route.rule_set.flatMap((ruleset) =>
      ruleset.type === RulesetType.Local ? ruleset.path : [],
    )
    return l1
  }

  eventBus.on('rulesetChange', ({ id }) => {
    if (running.value && profilesStore.currentProfile) {
      const inUse = profilesStore.currentProfile.route.rule_set.some(
        (ruleset) => ruleset.type === RulesetType.Local && ruleset.path === id,
      )
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  eventBus.on('rulesetsChange', () => {
    if (running.value && profilesStore.currentProfile) {
      const enabledRulesets = rulesetsStore.rulesets.flatMap((v) => (v.disabled ? [] : v.id))
      const inUse = collectRulesetIDs().some((v) => enabledRulesets.includes(v))
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  watch(needRestart, (v) => {
    if (v && appSettingsStore.app.autoRestartKernel) {
      restartCore()
    }
  })

  const watchSources = computed(() => {
    const source = [config.value.mode, config.value.tun.enable]
    if (!appSettingsStore.app.addGroupToMenu) return source.join('')

    const { unAvailable, sortByDelay } = appSettingsStore.app.kernel

    const proxySignature = Object.values(proxies.value)
      .map((group) => group.name + group.now)
      .sort()
      .join()

    return source.concat([proxySignature, unAvailable, sortByDelay]).join('')
  })

  watch([watchSources, running], updateTrayAndMenus)

  return {
    restartAppliedCore,
    startCore,
    stopCore,
    restartCore,
    initCoreState,
    pid: corePid,
    running,
    starting,
    stopping,
    restarting,
    needRestart,
    coreStateLoading,
    config,
    proxies,
    refreshConfig,
    updateConfig,
    refreshProviderProxies,
    getProxyEndpoint,

    onLogs,
    onMemory,
    onTraffic,
    onConnections,
  }
})
