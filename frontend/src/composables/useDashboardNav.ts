import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAppStore, useAuthStore } from '@/stores'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import {
  BellIcon,
  ChannelIcon,
  ChartIcon,
  CogIcon,
  CreditCardIcon,
  DashboardIcon,
  FolderIcon,
  GiftIcon,
  GlobeIcon,
  KeyIcon,
  OrderIcon,
  OrderListIcon,
  PriceTagIcon,
  RechargeSubscriptionIcon,
  ServerIcon,
  ShieldIcon,
  SignalIcon,
  TicketIcon,
  UserIcon,
  UsersIcon,
} from '@/components/layout/navIcons'

export interface NavItem {
  path: string
  label: string
  icon: unknown
  iconSvg?: string
  hideInSimpleMode?: boolean
  children?: NavItem[]
  /** 为 true 时父项点击只切换展开/折叠，不导航到自身 path；path 仅作稳定 key。 */
  expandOnly?: boolean
  /** featureFlag() 返回 false 时隐藏；undefined/true 时显示（宽容策略，避免设置未加载时菜单闪烁）。 */
  featureFlag?: () => boolean | undefined
}

// 递归过滤 featureFlag() === false 的节点（含子节点）。undefined/true 都视为显示。
function applyFeatureFlags(items: NavItem[]): NavItem[] {
  const out: NavItem[] = []
  for (const item of items) {
    if (item.featureFlag && item.featureFlag() === false) continue
    if (item.children) {
      out.push({ ...item, children: applyFeatureFlags(item.children) })
    } else {
      out.push(item)
    }
  }
  return out
}

/**
 * useDashboardNav —— 控制台导航数据的唯一来源。
 * 旧的 AppSidebar 与新的 Stone 导航轨（AppRail）共用本 composable，确保两套外壳菜单完全一致。
 */
export function useDashboardNav() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()

  const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
  const flagPayment = makeSidebarFlag(FeatureFlags.payment)
  const flagAvailableChannels = makeSidebarFlag(FeatureFlags.availableChannels)
  const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
  const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
  const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
  const flagAdminPayment = () => adminSettingsStore.paymentEnabled

  const customMenuItemsForUser = computed(() => {
    const items = appStore.cachedPublicSettings?.custom_menu_items ?? []
    return items
      .filter((item) => item.visibility === 'user')
      .sort((a, b) => a.sort_order - b.sort_order)
  })

  const customMenuItemsForAdmin = computed(() =>
    adminSettingsStore.customMenuItems
      .filter((item) => item.visibility === 'admin')
      .sort((a, b) => a.sort_order - b.sort_order),
  )

  function finalizeNav(items: NavItem[]): NavItem[] {
    const visible = applyFeatureFlags(items)
    return authStore.isSimpleMode ? visible.filter((item) => !item.hideInSimpleMode) : visible
  }

  // 用户自己的导航项（用户端主菜单 + 管理员"我的账户"子菜单共用）。
  function buildSelfNavItems(withDashboard: boolean): NavItem[] {
    const items: NavItem[] = []
    if (withDashboard) {
      items.push({ path: '/dashboard', label: t('nav.dashboard'), icon: DashboardIcon })
    }
    items.push(
      { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon },
      { path: '/usage', label: t('nav.usage'), icon: ChartIcon, hideInSimpleMode: true },
      { path: '/available-channels', label: t('nav.availableChannels'), icon: ChannelIcon, hideInSimpleMode: true, featureFlag: flagAvailableChannels },
      { path: '/monitor', label: t('nav.channelStatus'), icon: SignalIcon, featureFlag: flagChannelMonitor },
      { path: '/subscriptions', label: t('nav.mySubscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
      { path: '/purchase', label: t('nav.buySubscription'), icon: RechargeSubscriptionIcon, hideInSimpleMode: true, featureFlag: flagPayment },
      { path: '/orders', label: t('nav.myOrders'), icon: OrderListIcon, hideInSimpleMode: true, featureFlag: flagPayment },
      { path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true },
      { path: '/affiliate', label: t('nav.affiliate'), icon: UsersIcon, hideInSimpleMode: true, featureFlag: flagAffiliate },
      { path: '/profile', label: t('nav.profile'), icon: UserIcon },
      ...customMenuItemsForUser.value.map((item): NavItem => ({
        path: `/custom/${item.id}`,
        label: item.label,
        icon: null,
        iconSvg: item.icon_svg,
      })),
    )
    return items
  }

  const userNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(true)))
  const personalNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(false)))

  const adminNavItems = computed((): NavItem[] => {
    const baseItems: NavItem[] = [
      { path: '/admin/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
      { path: '/admin/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
      { path: '/admin/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
      { path: '/admin/groups', label: t('nav.groups'), icon: FolderIcon, hideInSimpleMode: true },
      {
        path: '/admin/channels',
        label: t('nav.channelManagement'),
        icon: ChannelIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        children: [
          { path: '/admin/channels/pricing', label: t('nav.channelPricing'), icon: PriceTagIcon },
          { path: '/admin/channels/monitor', label: t('nav.channelMonitor'), icon: SignalIcon, featureFlag: flagChannelMonitor },
        ],
      },
      { path: '/admin/subscriptions', label: t('nav.subscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
      { path: '/admin/accounts', label: t('nav.accounts'), icon: GlobeIcon },
      { path: '/admin/announcements', label: t('nav.announcements'), icon: BellIcon },
      { path: '/admin/proxies', label: t('nav.proxies'), icon: ServerIcon },
      { path: '/admin/risk-control', label: t('nav.riskControl'), icon: ShieldIcon, hideInSimpleMode: true, featureFlag: flagRiskControl },
      { path: '/admin/redeem', label: t('nav.redeemCodes'), icon: TicketIcon, hideInSimpleMode: true },
      { path: '/admin/promo-codes', label: t('nav.promoCodes'), icon: GiftIcon, hideInSimpleMode: true },
      {
        path: '/admin/affiliates',
        label: t('nav.affiliateManagement'),
        icon: UsersIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        featureFlag: flagAffiliate,
        children: [
          { path: '/admin/affiliates/invites', label: t('nav.affiliateInviteRecords'), icon: UsersIcon },
          { path: '/admin/affiliates/rebates', label: t('nav.affiliateRebateRecords'), icon: OrderIcon },
          { path: '/admin/affiliates/transfers', label: t('nav.affiliateTransferRecords'), icon: CreditCardIcon },
        ],
      },
      {
        path: '/admin/orders',
        label: t('nav.orderManagement'),
        icon: OrderIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        featureFlag: flagAdminPayment,
        children: [
          { path: '/admin/orders/dashboard', label: t('nav.paymentDashboard'), icon: ChartIcon },
          { path: '/admin/orders', label: t('nav.orderManagement'), icon: OrderIcon },
          { path: '/admin/orders/plans', label: t('nav.paymentPlans'), icon: CreditCardIcon },
        ],
      },
      { path: '/admin/usage', label: t('nav.usage'), icon: ChartIcon },
    ]

    const visible = applyFeatureFlags(baseItems)

    // 简单模式下，在系统设置前插入 API 密钥
    if (authStore.isSimpleMode) {
      const filtered = visible.filter((item) => !item.hideInSimpleMode)
      filtered.push({ path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon })
      filtered.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
      for (const cm of customMenuItemsForAdmin.value) {
        filtered.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
      }
      return filtered
    }

    visible.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
    for (const cm of customMenuItemsForAdmin.value) {
      visible.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
    }
    return visible
  })

  return { userNavItems, personalNavItems, adminNavItems }
}
