import { protocolMessages as protocols } from '../protocols'
import { enGB as inspector } from './inspector'
import { enGB as home } from './home'
import { enGB as accessKeys } from './access-keys'
import { enGB as logs } from './logs'
import { enGB as usage } from './usage'
import { enGB as health } from './health'
import { enGB as credentialCards } from './credential-cards'
import { enGB as groupWorkflows } from './group-workflows'
import { enGB as parameterRules } from './parameter-rules'
import { enGB as groupDetail } from './group-detail'
import { enGB as groupMessages } from './groups'
import { enGB as ui } from './ui'
import { enGB as groupCreate } from './group-create'
import { enGB as modelSelection } from './model-selection'
import { enGB as modelManager } from './model-manager'
import { enGB as settingsForm } from './settings-form'
import { enGB as autoModel } from './auto-model'
import { enGB as subscriptions } from './subscriptions'

export default {
  home,
  inspector,
  protocols,
  logs,
  usage,
  health,
  accessKeys,
  groupWorkflows,
  parameterRules,
  credentialCards,
  groupDetail,
  subscriptions,
  modelSelection,
  modelManager,
  settingsForm,
  autoModel,
  groupCreate,
  ui,
  ...groupMessages,
  sections: {
    workspace: 'Workspace',
    observe: 'Observability',
    system: 'System',
  },
  shell: {
    appName: 'Demerzel',
    refresh: 'Refresh',
    refreshedAt: 'Updated {time}',
    importCredentials: 'Import credentials',
    goHome: 'Demerzel overview',
    collapseSidebar: 'Collapse sidebar',
    expandSidebar: 'Expand sidebar',
    close: 'Close',
    documentation: 'User guide',
    mobileNavigationDescription: 'Open a management workspace or observability page.',
    navigationFailed: 'Unable to load this page. Please reload.',
    reload: 'Reload',
  },
  auth: {
    title: 'Sign in to Demerzel',
    description: 'Enter an admin key or access key. The server identifies your permissions.',
    keyLabel: 'Sign-in key',
    keyPlaceholder: 'Enter an AUTH_KEY or access key',
    reveal: 'Show sign-in key',
    conceal: 'Hide sign-in key',
    submit: 'Sign in',
    submitting: 'Verifying…',
    required: 'Enter a sign-in key',
    invalidFormat: 'The sign-in key cannot contain whitespace',
    invalid: 'The sign-in key is invalid. Check it and try again.',
    locked: 'Too many authentication attempts. Try again in {seconds} seconds.',
    lockEnded: 'You can verify your session again.',
    network: 'Unable to reach the management API. Check the service and try again.',
    invalidResponse: 'The management API returned an unrecognised response. Try again later.',
    restoreTitle: 'Verify your session',
    checking: 'Verifying the current session…',
    retry: 'Verify again',
    changeKey: 'Use another key',
    signedIn: 'Signed in. Opening your page…',
    continue: 'Continue',
    remember: 'Remember me',
    logout: 'Sign out',
    readOnly: 'Access key · Read-only',
    readOnlyDescription:
      'View only this key’s overview, models, usage, and request history. System configuration cannot be changed.',
    help: {
      title: 'Which key can I use?',
      accessKeyTitle: 'Access key',
      accessKey:
        'Use a distributed access key to view its own data. It cannot change configuration.',
      adminTitle: 'Admin key',
      admin: 'If AUTH_KEY is configured, use its value from your deployment system.',
      file: 'Without AUTH_KEY, read the key from {path}; the default container path is {containerPath}.',
      container:
        'For Podman Compose, enter the container from your own terminal and read the file. Keep the key out of logs and chats.',
    },
  },
  pages: {
    home: { title: 'Overview' },
    groups: { title: 'Groups' },
    groupDetail: { title: 'Group details' },
    models: { title: 'Models' },
    accessKeys: { title: 'Access keys' },
    usage: { title: 'Usage' },
    logs: { title: 'Request logs' },
    health: { title: 'Runtime health' },
    inspector: { title: 'Route inspection' },
    settings: { title: 'Global settings' },
    import: { title: 'Import credentials' },
    login: { title: 'Sign in' },
  },
  notFound: {
    title: 'Page not found',
    description: 'Check the page address or return to the overview.',
    backHome: 'Back to overview',
  },
  appearance: {
    theme: 'Display mode',
    themes: {
      system: 'System',
      light: 'Light',
      dark: 'Dark',
    },
    persistenceFailed: 'Browser storage is unavailable. Your preferences apply to this visit only.',
  },
  system: {
    currentVersion: 'Current version: {version}',
    loadingVersion: 'Loading…',
    versionUnavailable: 'Version unavailable',
  },
  navigation: 'Main navigation',
  skipToContent: 'Skip to main content',
  interfaceSettings: 'Interface',
  frontend: {
    description: 'Reloads the page and applies only to this browser.',
    current: 'Current interface',
    saveFailed: 'Unable to save your preference. Allow browser storage for this site and retry.',
    modern: { title: 'Modern' },
    classic: { title: 'Classic' },
  },
}
