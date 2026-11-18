// Карта Leaflet (подключается на странице как /static/vendor/leaflet/leaflet.js → window.L).
// Подложка — OpenStreetMap; маркеры — собственные (divIcon), цвет по статусу автомобиля.

const MOSCOW = [55.7522, 37.6156];

export function createMap(container, { center = MOSCOW, zoom = 12 } = {}) {
  const L = window.L;
  const map = L.map(container, { scrollWheelZoom: false }).setView(center, zoom);
  L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">участники OpenStreetMap</a>',
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
