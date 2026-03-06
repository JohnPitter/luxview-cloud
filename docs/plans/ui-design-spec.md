# LuxView Cloud — UI Design Spec

## Design Direction

Premium, dark-first, glassmorphism aesthetic. Minimal, polished, toolbar-driven navigation.

## Reference: Main Toolbar

Horizontal pill bar as primary navigation:

- **Shape:** rounded-3xl, h-12, px-6
- **Effects:** shadow-2xl, backdrop-blur-md (glass effect)
- **Dark mode:** bg-zinc-950 text-white
- **Light mode:** bg-white text-zinc-950
- **Transitions:** 200ms smooth on all interactive elements

### Toolbar Items (L-to-R, gap-6)

1. **Logo/Brand** — Rounded-xl square with bold text (font-semibold tracking-tighter), thin amber-400 ring-1 + glow on hover/active only
2. **Signal icon** — Status/monitoring
3. **Layers icon** — Apps/stacks
4. **File-text icon** — Logs/docs
5. **Sun/Moon toggle** — Click switches full theme + icon

### Interaction Details

- Hover glow effect on logo/brand icon only (amber-400 ring glow)
- All icons: subtle opacity/scale transition on hover
- Theme toggle: smooth icon morph between Sun and Moon
- Fully interactive, responsive

## Design Principles

- **Glass morphism** — backdrop-blur + semi-transparent backgrounds
- **Pill-shaped containers** — rounded-3xl for primary UI elements
- **Minimal chrome** — content-first, toolbar is the only persistent navigation
- **Dark-first** — dark mode is the default, light mode supported
- **Amber accent** — amber-400 as primary brand color (rings, glows, active states)
- **Zinc palette** — zinc-950 (dark bg), zinc-900 (cards), zinc-800 (borders), zinc-400 (muted text)

## Tech Stack

- React 18
- Tailwind CSS
- Lucide React (icons)
- Zustand (theme state)
- Framer Motion (optional, for advanced animations)
