// Form logic for the inspector: turning input values into settings.json values and back.
// Field kinds come from modules.FieldKind in Go.

const isList = kind => kind === "url_list" || kind === "text_list";

// coerce turns an input's value into the settings.json value; undefined removes the key.
export function coerce(field, raw) {
  switch (field.kind) {
    case "bool":
      return Boolean(raw);
    case "number": {
      const s = String(raw).trim();
      if (s === "") return undefined;
      const n = Number(s);
      return Number.isFinite(n) ? n : undefined;
    }
    case "select": {
      const hit = (field.options || []).find(o => String(o) === String(raw));
      return hit === undefined ? undefined : hit;
    }
  }
  if (isList(field.kind)) {
    const items = String(raw).split("\n").map(s => s.trim()).filter(Boolean);
    return items.length ? items.join(", ") : undefined;
  }
  const s = String(raw ?? "").trim();
  return s === "" ? undefined : s;
}

// display turns a settings.json value into the input's value.
export function display(field, value) {
  if (value === undefined || value === null) return field.kind === "bool" ? false : "";
  if (isList(field.kind)) {
    const items = Array.isArray(value) ? value : String(value).split(",");
    return items.map(s => String(s).trim()).filter(Boolean).join("\n");
  }
  return value;
}

export function visible(field, values) {
  const cond = field.show_if;
  return !cond || cond.values.some(v => String(v) === String(values[cond.key]));
}

// missing lists the required, visible fields that have no value.
export function missing(meta, values) {
  return (meta.fields || [])
    .filter(f => f.required && visible(f, values))
    .filter(f => values[f.key] === undefined || values[f.key] === null || String(values[f.key]).trim() === "")
    .map(f => f.key);
}

// stripSecrets blanks API keys and tokens, e.g. before sharing a tile.
export function stripSecrets(meta, values) {
  const out = { ...values };
  for (const f of meta.fields || []) {
    if (f.kind === "secret" && f.key in out) out[f.key] = "";
  }
  return out;
}
