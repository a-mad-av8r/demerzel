import { getPreferredFrontend } from '@shared/frontend/preference'

async function bootstrap(): Promise<void> {
  const frontend = await getPreferredFrontend()
  document.documentElement.dataset.frontend = frontend
  const module =
    frontend === 'classic'
      ? await import('./frontends/classic/bootstrap')
      : await import('./frontends/modern/bootstrap')
  await module.bootstrap()
}

function showStartupFailure(): void {
  const labels = {
    message: 'Unable to load the interface. Please retry.',
    retry: 'Reload',
  }
  const root = document.getElementById('app')
  if (!root) return
  const message = document.createElement('p')
  message.setAttribute('role', 'alert')
  message.textContent = labels.message
  const retry = document.createElement('button')
  retry.type = 'button'
  retry.textContent = labels.retry
  retry.addEventListener('click', () => window.location.reload())
  root.replaceChildren(message, retry)
}

void bootstrap().catch(showStartupFailure)
