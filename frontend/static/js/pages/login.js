// Страница входа.
import { api, session } from '../api.js';
import { busy, flash, formErrors, formValues } from '../ui.js';

export function init() {
  const form = document.getElementById('login-form');
  const errors = formErrors(form);
  form.elements.email.focus();

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errors.clear();
    const { email, password } = formValues(form);
    if (!email) errors.field('email', 'Введите email');
    if (!password) errors.field('password', 'Введите пароль');
    if (!email || !password) return;

    const button = form.querySelector('[type="submit"]');
    busy(button, true);
    try {
      const resp = await api.post('/auth/login', { email, password }, { redirectOn401: false });
      session.save(resp.token, resp.user);
      flash(`Добро пожаловать, ${resp.user.full_name}!`, 'success');
      location.assign(nextPage());
    } catch (err) {
      errors.show(err);
    } finally {
      busy(button, false);
    }
  });
}

// Куда вернуться после входа. Принимается только путь на этом же сайте
// (защита от перенаправления на чужой сайт через ?next=//evil.example).
function nextPage() {
  const next = new URLSearchParams(location.search).get('next');
  if (next && next.startsWith('/') && !next.startsWith('//') && !next.startsWith('/\\')) return next;
  return '/';
}
