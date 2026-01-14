# GitHub Copilot Instructions — HydraDNS Angular UI

You are an expert in TypeScript, Angular, and scalable web application development. You write functional, maintainable, performant, and accessible code following Angular and TypeScript best practices.

## Project Context

This is the web management UI for HydraDNS, a high-performance DNS forwarding server. The UI provides:
- Dashboard with real-time server statistics
- Filtering management (whitelist/blacklist)
- Custom DNS record management (hosts/CNAMEs)

## Technology Stack
- Angular 21+
- NgRx Signal Store for state management
- Tailwind CSS 4 for styling
- SCSS for component styles

---

## TypeScript Best Practices

- Use strict type checking
- Prefer type inference when the type is obvious
- Avoid the `any` type; use `unknown` when type is uncertain

---

## Angular Best Practices

- Always use standalone components over NgModules
- Must NOT set `standalone: true` inside Angular decorators. It's the default in Angular v20+.
- Use signals for state management
- Implement lazy loading for feature routes
- Do NOT use the `@HostBinding` and `@HostListener` decorators. Put host bindings inside the `host` object of the `@Component` or `@Directive` decorator instead
- Use `NgOptimizedImage` for all static images
  - `NgOptimizedImage` does not work for inline base64 images

---

## Accessibility Requirements

- It MUST pass all AXE checks
- It MUST follow all WCAG AA minimums, including focus management, color contrast, and ARIA attributes

---

## Components

- Keep components small and focused on a single responsibility
- Use `input()` and `output()` functions instead of decorators
- Use `computed()` for derived state
- Set `changeDetection: ChangeDetectionStrategy.OnPush` in `@Component` decorator
- Prefer Reactive forms instead of Template-driven ones
- Do NOT use `ngClass`, use `class` bindings instead
- Do NOT use `ngStyle`, use `style` bindings instead
- Use external templates/styles with paths relative to the component TS file

### Component File Structure
Each component should have its own folder with separate files:
```
component-name/
├── component-name.ts      # Component class
├── component-name.html    # Template
└── component-name.scss    # Styles
```

---

## State Management

- Use NgRx Signal Store for global/shared state
- Use signals for local component state
- Use `computed()` for derived state
- Keep state transformations pure and predictable
- Do NOT use `mutate` on signals, use `update` or `set` instead

---

## Templates

- Keep templates simple and avoid complex logic
- Use native control flow (`@if`, `@for`, `@switch`) instead of `*ngIf`, `*ngFor`, `*ngSwitch`
- Use the async pipe to handle observables
- Do not assume globals like (`new Date()`) are available in templates
- Do not write arrow functions in templates (they are not supported)

---

## Services

- Design services around a single responsibility
- Use the `providedIn: 'root'` option for singleton services
- Use the `inject()` function instead of constructor injection

---

## Styling

- Use Tailwind CSS utility classes for layout and common styles
- Use SCSS files for component-specific styles
- Follow dark mode patterns with `dark:` variants
- Use consistent color palette (slate for neutrals, blue for primary actions)
