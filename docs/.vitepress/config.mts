import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Open Accounting',
  description: 'Documentation for the Open Accounting platform',
  lang: 'en-US',
  base: '/open-accounting/',
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ['meta', { name: 'theme-color', content: '#0f766e' }],
    ['link', { rel: 'icon', href: '/open-accounting/favicon.svg', type: 'image/svg+xml' }],
  ],
  themeConfig: {
    logo: '/favicon.svg',
    siteTitle: 'Open Accounting',
    search: {
      provider: 'local',
    },
    nav: [
      { text: 'Guide', link: '/README' },
      { text: 'API', link: '/API' },
      { text: 'CLI', link: '/CLI' },
      { text: 'Deployment', link: '/DEPLOYMENT' },
    ],
    sidebar: [
      {
        text: 'Overview',
        items: [
          { text: 'Documentation index', link: '/README' },
          { text: 'Current product limits', link: '/CURRENT_PRODUCT_LIMITS' },
          { text: 'Development status', link: '/DEVELOPMENT_STATUS' },
          { text: 'Use-case coverage', link: '/USE_CASE_COVERAGE' },
          { text: 'Architecture', link: '/ARCHITECTURE' },
        ],
      },
      {
        text: 'Interfaces',
        items: [
          { text: 'API reference', link: '/API' },
          { text: 'CLI guide', link: '/CLI' },
          { text: 'Plugins', link: '/PLUGINS' },
          { text: 'Demo E2E testing', link: '/demo-e2e-testing' },
        ],
      },
      {
        text: 'Operations',
        items: [
          { text: 'Deployment', link: '/DEPLOYMENT' },
          { text: 'Pilot operations', link: '/PILOT_OPERATIONS' },
          { text: 'Pilot readiness record', link: '/PILOT_READINESS_RECORD_TEMPLATE' },
          { text: 'e-MTA integration', link: '/EMTA_INTEGRATION' },
        ],
      },
      {
        text: 'Migration',
        items: [
          { text: 'SmartAccounts migration', link: '/SMARTACCOUNTS_MIGRATION' },
          { text: 'Pilot cutover', link: '/SMARTACCOUNTS_PILOT_CUTOVER' },
          { text: 'Sync coverage', link: '/SMARTACCOUNTS_SYNC_COVERAGE_TRACKER' },
          { text: 'Merit and SmartAccounts mapping', link: '/FEATURE_MAPPING_MERIT_SMARTACCOUNTS' },
        ],
      },
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/HMB-research/open-accounting' },
    ],
    editLink: {
      pattern: 'https://github.com/HMB-research/open-accounting/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Open Accounting contributors',
    },
    outline: {
      level: [2, 3],
    },
  },
})
