const ICON_SRC = "/favicon.svg";
const SIZE = 64;

let original: HTMLLinkElement[] | null = null;
let badgeLink: HTMLLinkElement | null = null;
let sourceImage: Promise<HTMLImageElement> | null = null;
let latestCount = 0;

function loadSource(): Promise<HTMLImageElement> {
  sourceImage ??= new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = ICON_SRC;
  });
  return sourceImage;
}

function drawBadge(img: HTMLImageElement, count: number): string | null {
  const canvas = document.createElement("canvas");
  canvas.width = SIZE;
  canvas.height = SIZE;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;
  ctx.drawImage(img, 0, 0, SIZE, SIZE);
  const r = SIZE * 0.3;
  ctx.beginPath();
  ctx.arc(SIZE - r, r, r, 0, Math.PI * 2);
  ctx.fillStyle = "#dc2626";
  ctx.fill();
  ctx.fillStyle = "#ffffff";
  ctx.font = `bold ${Math.round(r * 1.4)}px sans-serif`;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText(count > 9 ? "9+" : String(count), SIZE - r, r + 1);
  try {
    return canvas.toDataURL("image/png");
  } catch {
    return null;
  }
}

export async function renderTabIcon(count: number): Promise<void> {
  latestCount = count;
  original ??= Array.from(
    document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
  );

  if (count === 0) {
    badgeLink?.remove();
    badgeLink = null;
    original.forEach((link) => document.head.appendChild(link));
    return;
  }

  const img = await loadSource().catch(() => null);
  // A reset may have landed while the image was loading.
  if (!img || latestCount !== count) return;
  const href = drawBadge(img, count);
  if (!href) return;

  original.forEach((link) => link.remove());
  if (!badgeLink) {
    badgeLink = document.createElement("link");
    badgeLink.rel = "icon";
    badgeLink.type = "image/png";
    document.head.appendChild(badgeLink);
  }
  badgeLink.href = href;
}
