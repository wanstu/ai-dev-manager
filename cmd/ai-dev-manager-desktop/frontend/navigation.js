(function attachManagementNavigation(global) {
  const routes = [
    'overview',
    'workspaces',
    'environments',
    'runtime',
    'mcp',
    'skills',
    'memory',
    'gateway',
    'diagnostics',
    'exec-allowlist',
    'settings',
    'about',
  ];
  const routeSet = new Set(routes);

  function normalizeRoute(value) {
    const token = String(value || '')
      .replace(/^#\/?/, '')
      .replace(/^\/+/, '')
      .split(/[?#]/, 1)[0]
      .trim()
      .toLowerCase();
    return routeSet.has(token) ? token : 'overview';
  }

  function routeHash(route) {
    return `#/${normalizeRoute(route)}`;
  }

  function createNavigation(options = {}) {
    const documentRef = options.document || global.document;
    const windowRef = options.window || global;
    if (!documentRef || !windowRef) throw new Error('management navigation requires window and document');

    const pages = [...documentRef.querySelectorAll('[data-management-page]')];
    const links = [...documentRef.querySelectorAll('[data-route-link]')];
    const beforeNavigate = typeof options.beforeNavigate === 'function' ? options.beforeNavigate : () => true;
    const afterNavigate = typeof options.afterNavigate === 'function' ? options.afterNavigate : () => {};
    let currentRoute = '';

    function pageFor(route) {
      return pages.find((page) => page.dataset.managementPage === route) || null;
    }

    function updateRouteUI(route, focusHeading) {
      const page = pageFor(route) || pageFor('overview');
      for (const item of pages) item.hidden = item !== page;
      for (const link of links) {
        const isMenuItem = link.classList.contains('nav-link');
        const active = isMenuItem && normalizeRoute(link.dataset.routeLink || link.getAttribute('href')) === route;
        if (active) link.setAttribute('aria-current', 'page');
        else link.removeAttribute('aria-current');
      }
      currentRoute = route;
      const heading = page?.querySelector('[data-page-heading]');
      if (heading) {
        heading.tabIndex = -1;
        if (focusHeading) heading.focus({preventScroll: true});
      }
      afterNavigate(route, page);
      return route;
    }

    function restoreCurrentHash() {
      const route = currentRoute || 'overview';
      if (windowRef.location.hash !== routeHash(route)) {
        windowRef.history.replaceState(null, '', routeHash(route));
      }
    }

    function navigate(route, options = {}) {
      const target = normalizeRoute(route);
      const prior = currentRoute || normalizeRoute(windowRef.location.hash);
      if (target !== prior && beforeNavigate(target, prior) === false) {
        restoreCurrentHash();
        return prior;
      }
      const focusHeading = options.focus !== false;
      const replace = Boolean(options.replace);
      const desiredHash = routeHash(target);
      if (windowRef.location.hash !== desiredHash) {
        if (replace) windowRef.history.replaceState(null, '', desiredHash);
        else windowRef.history.pushState(null, '', desiredHash);
      }
      return updateRouteUI(target, focusHeading);
    }

    function syncFromLocation(focusHeading = true) {
      const target = normalizeRoute(windowRef.location.hash);
      const canonical = routeHash(target);
      if (windowRef.location.hash !== canonical) windowRef.history.replaceState(null, '', canonical);
      if (currentRoute && target !== currentRoute && beforeNavigate(target, currentRoute) === false) {
        restoreCurrentHash();
        return currentRoute;
      }
      return updateRouteUI(target, focusHeading);
    }

    function onRouteLinkClick(event) {
      const link = event.target.closest('[data-route-link]');
      if (!link) return;
      event.preventDefault();
      const target = normalizeRoute(link.dataset.routeLink || link.getAttribute('href'));
      navigate(target, {focus: true});
    }

    documentRef.addEventListener('click', onRouteLinkClick);
    windowRef.addEventListener('popstate', () => syncFromLocation(true));
    windowRef.addEventListener('hashchange', () => syncFromLocation(true));
    syncFromLocation(false);

    return {
      current: () => currentRoute,
      navigate,
      sync: syncFromLocation,
      routes: [...routes],
    };
  }

  const api = {routes: [...routes], normalizeRoute, routeHash, createNavigation};
  global.ADMNavigation = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
