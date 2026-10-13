// The "paper" view: the rendered page in e-paper tones instead of screen white, black and red.

export const PAPER = [232, 229, 220];
export const INK = [29, 29, 27];
export const RED = [163, 38, 42];

// toPaper recolours RGBA pixels in place. The grain is a fixed hash of the position, so
// re-rendering the same page gives the same pixels.
export function toPaper(data, w, h) {
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = (y * w + x) * 4;
      const r = data[i], g = data[i + 1];
      const base = r > 128 && g < 128 ? RED : r < 128 ? INK : PAPER;
      const grain = ((Math.imul(x * 73856093 ^ y * 19349663, 2654435761) >>> 24) % 7) - 3;
      data[i] = base[0] + grain;
      data[i + 1] = base[1] + grain;
      data[i + 2] = base[2] + grain;
    }
  }
}
