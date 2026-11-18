// Карта Leaflet (подключается на странице как /static/vendor/leaflet/leaflet.js → window.L).
// Подложка: Stadia Maps, стиль Alidade Smooth (светлая карта на данных OpenStreetMap и
// OpenMapTiles). Без ключа Stadia отдаёт тайлы для localhost; для сайта в интернете домен
// регистрируется в личном кабинете Stadia (бесплатно для некоммерческого использования).
// Маркеры собственные (divIcon), цвет по статусу автомобиля.

const MOSCOW = [55.7522, 37.6156];

export function createMap(container, { center = MOSCOW, zoom = 12 } = {}) {
  const L = window.L;
  const map = L.map(container, { scrollWheelZoom: false }).setView(center, zoom);
  // Строка атрибуции: префикс Leaflet (ссылка и флаг) убран, лицензия библиотеки (BSD)
  // этого не требует, библиотека указана на странице «Об авторе». Указание источника данных
  // обязательно по условиям Stadia Maps, OpenMapTiles и OpenStreetMap (ODbL), оно остаётся.
  map.attributionControl.setPrefix(false);
  L.tileLayer('https://tiles.stadiamaps.com/tiles/alidade_smooth/{z}/{x}/{y}{r}.png', {
    maxZoom: 20,
    attribution: '&copy; <a href="https://stadiamaps.com/">Stadia Maps</a>, &copy; <a href="https://openmaptiles.org/">OpenMapTiles</a>, &copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
  }).addTo(map);
  // Колесо мыши масштабирует карту только после клика по ней, чтобы не мешать прокрутке страницы.
  map.on('click', () => map.scrollWheelZoom.enable());
  map.on('mouseout', () => map.scrollWheelZoom.disable());
  return map;
}

export function carIcon(status) {
  return window.L.divIcon({
    className: `car-marker car-marker-${status}`,
    html: '<span class="car-marker-pin"></span>',
    iconSize: [26, 34],
    iconAnchor: [13, 34],
    popupAnchor: [0, -30],
  });
}

export const leafletReady = () => Boolean(window.L);

/**
 * Группа маркеров с объединением близких машин (плагин Leaflet.markercluster).
 * При отдалении машины собираются в квадрат с числом, при приближении рассыпаются;
 * с 16-го масштаба (уровень улиц) каждая машина показывается отдельно.
 */
export function carClusterGroup() {
  const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  return window.L.markerClusterGroup({
    maxClusterRadius: 48,
    disableClusteringAtZoom: 16,
    showCoverageOnHover: false,
    spiderfyOnMaxZoom: true,
    animate: !reduceMotion,
    iconCreateFunction(cluster) {
      const n = cluster.getChildCount();
      const size = n < 10 ? 36 : n < 100 ? 42 : 48;
      return window.L.divIcon({
        // Число собирается из счётчика плагина, пользовательские данные сюда не попадают.
        html: `<span class="car-cluster-count">${Number(n)}</span>`,
        className: 'car-cluster',
        iconSize: [size, size],
      });
    },
  });
}
