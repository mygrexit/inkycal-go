// The "paper" view: the rendered page in e-paper tones instead of screen white, black and red.

export const PAPER = [232, 229, 220];
export const INK = [29, 29, 27];
export const RED = [163, 38, 42];
export const YELLOW = [214, 172, 30];
export const GREY = [92, 91, 87];

// what the preview sends (screen colours) → how e-paper shows it
const TONES = [
  [[255, 255, 255], PAPER],
  [[240, 240, 240], PAPER], // IT8951 paper
  [[0, 0, 0], INK],
  [[196, 22, 28], RED],
  [[255, 0, 0], RED],
  [[230, 190, 0], YELLOW],
  [[64, 64, 64], GREY],
];

function tone(r, g, b) {
  let best = TONES[0][1], dist = Infinity;
  for (const [c, t] of TONES) {
    const d = (r - c[0]) ** 2 + (g - c[1]) ** 2 + (b - c[2]) ** 2;
    if (d < dist) {
      dist = d;
      best = t;
    }
  }
  return best;
}

// toPaper recolours RGBA pixels in place. The grain is a fixed hash of the position, so
// re-rendering the same page gives the same pixels.
export function toPaper(data, w, h) {
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = (y * w + x) * 4;
      const base = tone(data[i], data[i + 1], data[i + 2]);
      const grain = ((Math.imul(x * 73856093 ^ y * 19349663, 2654435761) >>> 24) % 7) - 3;
      data[i] = base[0] + grain;
      data[i + 1] = base[1] + grain;
      data[i + 2] = base[2] + grain;
    }
  }
}
