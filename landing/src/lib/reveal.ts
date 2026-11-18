const reduced = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/** Each [data-reveal] block rises in once, the first time it enters the viewport. */
export function initReveals(): void {
  const blocks = document.querySelectorAll<HTMLElement>("[data-reveal]");
  if (!("IntersectionObserver" in window)) {
    blocks.forEach((el) => el.classList.add("is-visible"));
    return;
  }
  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add("is-visible");
        observer.unobserve(entry.target);
      }
    },
    { threshold: 0.15 },
  );
  blocks.forEach((el) => observer.observe(el));
}

/** Words of a [data-tagline] block light up one at a time as they cross the trigger line. */
export function initTagline(): void {
  const words = document.querySelectorAll<HTMLElement>("[data-tagline] [data-word]");
  if (reduced() || !("IntersectionObserver" in window)) {
    words.forEach((w) => w.classList.add("is-lit"));
    return;
  }
  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        entry.target.classList.add("is-lit");
        observer.unobserve(entry.target);
      }
    },
    { rootMargin: "0px 0px -35% 0px", threshold: 0 },
  );
  words.forEach((w) => observer.observe(w));
}

/** The phone action bar steps aside while the form is on screen. */
export function initMobileBar(): void {
  const bar = document.querySelector<HTMLElement>("[data-mobile-bar]");
  const form = document.getElementById("demo");
  if (!bar || !form || !("IntersectionObserver" in window)) return;
  new IntersectionObserver(
    ([entry]) => {
      const onScreen = entry?.isIntersecting ?? false;
      bar.classList.toggle("translate-y-full", onScreen);
      bar.toggleAttribute("inert", onScreen);
    },
    { threshold: 0.1 },
  ).observe(form);
}
